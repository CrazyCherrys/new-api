package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/worker_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupImageGenerationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.ModelMapping{}, &model.ImageGenerationTask{}, &model.ImageCreativeSubmission{}); err != nil {
		t.Fatalf("failed to migrate image generation task table: %v", err)
	}

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUserToken(t *testing.T, db *gorm.DB, userId int, key string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:      userId,
		Key:         key,
		Name:        "image-task-token",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create user token: %v", err)
	}
	return token
}

func TestRetryImageGenerationTaskResetsAndEnqueuesFailedTask(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)
	user := &model.User{
		Id:       1,
		Username: "retry-task-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
		AffCode:  "aff-retry-task-user",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "test prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusFailed,
		ErrorMessage:    "previous failure",
		CompletedTime:   12345,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	previousEnqueue := enqueueImageGenerationTask
	enqueuedTaskIds := make([]int, 0, 1)
	enqueueImageGenerationTask = func(taskId int) {
		enqueuedTaskIds = append(enqueuedTaskIds, taskId)
	}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
	})

	if err := RetryImageGenerationTask(task.Id); err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded.Status != model.ImageTaskStatusPending {
		t.Fatalf("expected status %q, got %q", model.ImageTaskStatusPending, reloaded.Status)
	}
	if reloaded.ErrorMessage != "" {
		t.Fatalf("expected error message to be cleared, got %q", reloaded.ErrorMessage)
	}
	if reloaded.CompletedTime != 0 {
		t.Fatalf("expected completed time to be reset, got %d", reloaded.CompletedTime)
	}
	if reloaded.StartedTime == 0 {
		t.Fatal("expected started time to be reset for retry")
	}
	activeCount, err := model.GetUserImageGenerationActiveTaskCount(user.Id)
	if err != nil {
		t.Fatalf("failed to read active queue count after retry: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected active queue count 1 after retry reserve, got %d", activeCount)
	}
	if len(enqueuedTaskIds) != 1 || enqueuedTaskIds[0] != task.Id {
		t.Fatalf("expected task %d to be enqueued once, got %v", task.Id, enqueuedTaskIds)
	}
}

func TestImageGenerationTimeoutUsesWorkerSetting(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousTimeout := cfg.ImageTimeout
	previousTimeoutOverride := imageGenerationTimeoutOverride
	t.Cleanup(func() {
		cfg.ImageTimeout = previousTimeout
		imageGenerationTimeoutOverride = previousTimeoutOverride
	})

	cfg.ImageTimeout = 600
	if got := imageGenerationTimeout(); got != 600*time.Second {
		t.Fatalf("expected configured timeout 600s, got %v", got)
	}

	cfg.ImageTimeout = 0
	if got := imageGenerationTimeout(); got != 120*time.Second {
		t.Fatalf("expected default timeout 120s, got %v", got)
	}
}

func TestImageGenerationMaxRetriesPreservesZero(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousMaxRetries := cfg.MaxRetries
	t.Cleanup(func() {
		cfg.MaxRetries = previousMaxRetries
	})

	cfg.MaxRetries = 0
	if got := imageGenerationMaxRetries(); got != 0 {
		t.Fatalf("expected max retries 0, got %d", got)
	}

	cfg.MaxRetries = -1
	if got := imageGenerationMaxRetries(); got != 0 {
		t.Fatalf("expected negative max retries to clamp to 0, got %d", got)
	}

	cfg.MaxRetries = 4
	if got := imageGenerationMaxRetries(); got != 4 {
		t.Fatalf("expected max retries 4, got %d", got)
	}
}

func TestProcessImageGenerationTaskDoesNotDoubleDeductUserQuota(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "quota-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    700000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-serialized")

	task := &model.ImageGenerationTask{
		UserId:          user.Id,
		ModelId:         "gpt-image-1",
		Prompt:          "test prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	previousGenerateFn := generateImageFn
	generateImageFn = func(ctx context.Context, task *model.ImageGenerationTask) (string, string, string, int, error) {
		return "https://example.com/image.png", "", `{"size":"1024x1024"}`, 4800, nil
	}
	t.Cleanup(func() {
		generateImageFn = previousGenerateFn
	})

	var before model.User
	if err := db.Select("quota").Where("id = ?", user.Id).First(&before).Error; err != nil {
		t.Fatalf("failed to read quota before processing: %v", err)
	}

	if err := ProcessImageGenerationTask(task.Id); err != nil {
		t.Fatalf("process task failed: %v", err)
	}

	var after model.User
	if err := db.Select("quota").Where("id = ?", user.Id).First(&after).Error; err != nil {
		t.Fatalf("failed to read quota after processing: %v", err)
	}
	if before.Quota != after.Quota {
		t.Fatalf("expected quota unchanged after task success, before=%d after=%d", before.Quota, after.Quota)
	}
}

func TestProcessImageGenerationTaskResetsStartedTimeWhenClaimed(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-claim",
		Prompt:          "claim prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
		StartedTime:     123,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	previousGenerateFn := generateImageFn
	generateImageFn = func(ctx context.Context, task *model.ImageGenerationTask) (string, string, string, int, error) {
		return "https://example.com/image.png", "", `{}`, 1, nil
	}
	t.Cleanup(func() {
		generateImageFn = previousGenerateFn
	})

	if err := ProcessImageGenerationTask(task.Id); err != nil {
		t.Fatalf("process task failed: %v", err)
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected task to exist")
	}
	if reloaded.StartedTime <= task.StartedTime {
		t.Fatalf("expected started time to move forward, old=%d new=%d", task.StartedTime, reloaded.StartedTime)
	}
}

func TestImageGenerationTaskQueueLimitUsesWorkerSetting(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousMaxWorkers := cfg.MaxWorkers
	t.Cleanup(func() {
		cfg.MaxWorkers = previousMaxWorkers
	})

	cfg.MaxWorkers = 0
	if got := imageGenerationTaskQueueLimit(); got != defaultImageGenerationTaskQueueLimit {
		t.Fatalf("expected default queue limit %d, got %d", defaultImageGenerationTaskQueueLimit, got)
	}

	cfg.MaxWorkers = 2
	if got := imageGenerationTaskQueueLimit(); got != defaultImageGenerationTaskQueueLimit {
		t.Fatalf("expected floor queue limit %d, got %d", defaultImageGenerationTaskQueueLimit, got)
	}

	cfg.MaxWorkers = 6
	if got := imageGenerationTaskQueueLimit(); got != 30 {
		t.Fatalf("expected queue limit 30, got %d", got)
	}
}

func TestEnforceImageGenerationTaskQueueLimitRejectsExcessQueuedTasks(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousMaxWorkers := cfg.MaxWorkers
	t.Cleanup(func() {
		cfg.MaxWorkers = previousMaxWorkers
	})
	cfg.MaxWorkers = 1

	for _, userId := range []int{1, 2} {
		user := &model.User{
			Id:       userId,
			Username: fmt.Sprintf("queue-limit-user-%d", userId),
			Password: "hashed-password",
			Status:   1,
			Group:    "default",
			Quota:    1000000,
			AffCode:  fmt.Sprintf("aff-queue-limit-%d", userId),
		}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to create user %d: %v", userId, err)
		}
	}

	for i := 0; i < defaultImageGenerationTaskQueueLimit; i++ {
		task := &model.ImageGenerationTask{
			UserId:          1,
			ModelId:         fmt.Sprintf("queued-%d", i),
			Prompt:          "queued",
			RequestEndpoint: "openai",
			Status:          model.ImageTaskStatusPending,
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create queued task %d: %v", i, err)
		}
	}
	if _, err := model.ReconcileUserImageGenerationActiveTaskCount(1); err != nil {
		t.Fatalf("failed to reconcile active task count: %v", err)
	}

	if err := enforceImageGenerationTaskQueueLimit(1, 1); err == nil {
		t.Fatal("expected queue limit error")
	}
	if err := enforceImageGenerationTaskQueueLimit(2, 1); err != nil {
		t.Fatalf("expected different user to pass queue check: %v", err)
	}
}

func TestCreateImageGenerationTaskQueueLimitIsSerializedWithinProcess(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousMaxWorkers := cfg.MaxWorkers
	t.Cleanup(func() {
		cfg.MaxWorkers = previousMaxWorkers
	})
	cfg.MaxWorkers = 1

	user := &model.User{
		Username: "serialized-queue-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-capability")

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-serialized",
		ActualModel:       "gpt-image-serialized",
		DisplayName:       "GPT Image Serialized",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	previousEnqueue := enqueueImageGenerationTask
	enqueueImageGenerationTask = func(taskId int) {}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
	})

	successes := 0
	var successMu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < defaultImageGenerationTaskQueueLimit+1; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := CreateImageGenerationTask(
				user.Id,
				"gpt-image-serialized",
				fmt.Sprintf("prompt-%d", i),
				"openai",
				`{}`,
			); err == nil {
				successMu.Lock()
				successes++
				successMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if successes != defaultImageGenerationTaskQueueLimit {
		t.Fatalf("expected %d successful creations, got %d", defaultImageGenerationTaskQueueLimit, successes)
	}
}

func TestRunImageCleanupTaskOnceReadsLatestConfig(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousAutoCleanupEnabled := cfg.AutoCleanupEnabled
	previousRetentionDays := cfg.RetentionDays
	previousStorageType := cfg.StorageType
	previousLocalPath := cfg.LocalStoragePath
	previousLastRun := imageCleanupLastRun.Load()
	imageCleanupTaskRunning.Store(false)
	t.Cleanup(func() {
		cfg.AutoCleanupEnabled = previousAutoCleanupEnabled
		cfg.RetentionDays = previousRetentionDays
		cfg.StorageType = previousStorageType
		cfg.LocalStoragePath = previousLocalPath
		imageCleanupLastRun.Store(previousLastRun)
		imageCleanupTaskRunning.Store(false)
	})

	cfg.StorageType = "local"
	cfg.LocalStoragePath = t.TempDir()
	cfg.AutoCleanupEnabled = false
	cfg.RetentionDays = 1
	imageCleanupLastRun.Store(0)

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "cleanup-model",
		Prompt:          "cleanup prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		CreatedTime:     common.GetTimestamp() - 10*24*60*60,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create old task: %v", err)
	}

	runImageCleanupTaskOnce(time.Now())
	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task before enabling cleanup: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected task to remain when cleanup is disabled")
	}

	cfg.AutoCleanupEnabled = true
	runImageCleanupTaskOnce(time.Now())
	reloaded, err = model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task after enabling cleanup: %v", err)
	}
	if reloaded != nil {
		t.Fatal("expected task to be cleaned up after enabling cleanup")
	}
}

func TestImageGenerationUserChannelOverrideReadsUserWorkerSettings(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "worker-settings-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{
		WorkerApiKey:  " sk-user-key ",
		WorkerApiBase: "https://custom.example.com/v1/",
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousKeyEnabled := cfg.UserCustomKeyEnabled
	previousBaseAllowed := cfg.UserCustomBaseURLAllowed
	t.Cleanup(func() {
		cfg.UserCustomKeyEnabled = previousKeyEnabled
		cfg.UserCustomBaseURLAllowed = previousBaseAllowed
	})

	cfg.UserCustomKeyEnabled = true
	cfg.UserCustomBaseURLAllowed = true
	override, err := getImageGenerationUserChannelOverride(user.Id)
	if err != nil {
		t.Fatalf("unexpected override error: %v", err)
	}
	if override == nil {
		t.Fatalf("expected custom worker override")
	}
	if override.APIKey != "sk-user-key" {
		t.Fatalf("expected trimmed user API key, got %q", override.APIKey)
	}
	if override.BaseURL != "https://custom.example.com/v1" {
		t.Fatalf("expected trimmed custom base URL, got %q", override.BaseURL)
	}

	cfg.UserCustomBaseURLAllowed = false
	override, err = getImageGenerationUserChannelOverride(user.Id)
	if err != nil {
		t.Fatalf("unexpected override error with base disabled: %v", err)
	}
	if override == nil {
		t.Fatalf("expected custom worker override when key is enabled")
	}
	if override.APIKey != "sk-user-key" {
		t.Fatalf("expected user API key with base disabled, got %q", override.APIKey)
	}
	if override.BaseURL != "" {
		t.Fatalf("expected empty base URL when custom base is disabled, got %q", override.BaseURL)
	}
}

func TestImageGenerationUserChannelOverrideRequiresCustomKey(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "worker-settings-no-key",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{
		WorkerApiBase: "https://custom.example.com/v1",
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousKeyEnabled := cfg.UserCustomKeyEnabled
	previousBaseAllowed := cfg.UserCustomBaseURLAllowed
	t.Cleanup(func() {
		cfg.UserCustomKeyEnabled = previousKeyEnabled
		cfg.UserCustomBaseURLAllowed = previousBaseAllowed
	})

	cfg.UserCustomKeyEnabled = true
	cfg.UserCustomBaseURLAllowed = true
	override, err := getImageGenerationUserChannelOverride(user.Id)
	if err != nil {
		t.Fatalf("unexpected override error: %v", err)
	}
	if override != nil {
		t.Fatalf("expected no override without user API key, got %+v", override)
	}
}

func TestProcessTaskAsyncWaitsForWorkerWithoutTimingOut(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "image-queue-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-queue",
		ActualModel:       "gpt-image-queue",
		DisplayName:       "GPT Image Queue",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          user.Id,
		ModelId:         "gpt-image-queue",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousMaxWorkers := cfg.MaxWorkers
	previousTimeoutOverride := imageGenerationTimeoutOverride
	previousProcessFn := processImageGenerationTaskFn
	t.Cleanup(func() {
		cfg.MaxWorkers = previousMaxWorkers
		imageGenerationTimeoutOverride = previousTimeoutOverride
		processImageGenerationTaskFn = previousProcessFn
	})

	cfg.MaxWorkers = 1
	if err := imageWorkerLimiter.acquire(context.Background()); err != nil {
		t.Fatalf("failed to occupy worker slot: %v", err)
	}
	t.Cleanup(func() {
		imageWorkerLimiter.release()
	})

	finished := make(chan struct{})
	imageGenerationTimeoutOverride = func() time.Duration {
		return 50 * time.Millisecond
	}
	processImageGenerationTaskFn = func(taskId int) error {
		close(finished)
		return nil
	}

	go processTaskAsync(task.Id)
	time.Sleep(120 * time.Millisecond)
	select {
	case <-finished:
		t.Fatal("task started before worker slot was released")
	default:
	}

	imageWorkerLimiter.release()
	select {
	case <-finished:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task did not start after worker slot was released")
	}
}

func TestProcessTaskAsyncUsesFreshTimeoutPerRetry(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "image-retry-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-retry",
		ActualModel:       "gpt-image-retry",
		DisplayName:       "GPT Image Retry",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          user.Id,
		ModelId:         "gpt-image-retry",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousMaxWorkers := cfg.MaxWorkers
	previousMaxRetries := cfg.MaxRetries
	previousRetryDelay := cfg.RetryDelay
	previousTimeoutOverride := imageGenerationTimeoutOverride
	previousRetryDelayOverride := imageGenerationRetryDelayOverride
	previousGenerateFn := generateImageFn
	previousProcessFn := processImageGenerationTaskFn
	t.Cleanup(func() {
		cfg.MaxWorkers = previousMaxWorkers
		cfg.MaxRetries = previousMaxRetries
		cfg.RetryDelay = previousRetryDelay
		imageGenerationTimeoutOverride = previousTimeoutOverride
		imageGenerationRetryDelayOverride = previousRetryDelayOverride
		generateImageFn = previousGenerateFn
		processImageGenerationTaskFn = previousProcessFn
	})

	cfg.MaxWorkers = 1
	cfg.MaxRetries = 1
	cfg.RetryDelay = 0
	if err := imageWorkerLimiter.acquire(context.Background()); err != nil {
		t.Fatalf("failed to occupy worker slot: %v", err)
	}
	t.Cleanup(func() {
		imageWorkerLimiter.release()
	})

	retryCount := 0
	firstAttemptStarted := make(chan struct{})
	secondAttemptStarted := make(chan struct{})
	imageGenerationTimeoutOverride = func() time.Duration {
		retryCount++
		if retryCount == 1 {
			return 50 * time.Millisecond
		}
		return 200 * time.Millisecond
	}
	imageGenerationRetryDelayOverride = func() time.Duration {
		return 0
	}
	generateImageFn = func(ctx context.Context, task *model.ImageGenerationTask) (string, string, string, int, error) {
		switch retryCount {
		case 1:
			close(firstAttemptStarted)
			<-ctx.Done()
			return "", "", "", 0, ctx.Err()
		case 2:
			close(secondAttemptStarted)
			select {
			case <-ctx.Done():
				return "", "", "", 0, ctx.Err()
			case <-time.After(20 * time.Millisecond):
				return "url", "", "{}", 1, nil
			}
		default:
			return "", "", "", 0, fmt.Errorf("unexpected attempt %d", retryCount)
		}
	}

	go processTaskAsync(task.Id)
	imageWorkerLimiter.release()

	select {
	case <-firstAttemptStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first attempt did not start")
	}

	select {
	case <-secondAttemptStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second attempt did not start")
	}

	var reloaded *model.ImageGenerationTask
	var err error
	deadline := time.After(500 * time.Millisecond)
	for {
		reloaded, err = model.GetImageTaskByID(task.Id)
		if err != nil {
			t.Fatalf("failed to reload task: %v", err)
		}
		if reloaded.Status == model.ImageTaskStatusSuccess {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected success after retry, got %s (%s)", reloaded.Status, reloaded.ErrorMessage)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRecoverExpiredImageGenerationTasksMarksOnlyExpiredGeneratingTasks(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	now := common.GetTimestamp()
	expiredTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-expired",
		Prompt:          "expired",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		StartedTime:     now - 1000,
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  now - 1000,
	}
	freshTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-fresh",
		Prompt:          "fresh",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		StartedTime:     now - 10,
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  now + 100,
	}
	pendingTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-pending",
		Prompt:          "pending",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
		StartedTime:     now - 1000,
	}
	for _, task := range []*model.ImageGenerationTask{expiredTask, freshTask, pendingTask} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create task %s: %v", task.ModelId, err)
		}
	}

	recovered, err := model.FailExpiredGeneratingImageTasks(now-100, "expired", 100)
	if err != nil {
		t.Fatalf("failed to recover expired tasks: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered task, got %d", recovered)
	}

	expiredReloaded, _ := model.GetImageTaskByID(expiredTask.Id)
	freshReloaded, _ := model.GetImageTaskByID(freshTask.Id)
	pendingReloaded, _ := model.GetImageTaskByID(pendingTask.Id)

	if expiredReloaded.Status != model.ImageTaskStatusFailed {
		t.Fatalf("expected expired generating task to fail, got %s", expiredReloaded.Status)
	}
	if expiredReloaded.CompletedTime == 0 {
		t.Fatal("expected expired generating task completed time to be set")
	}
	if freshReloaded.Status != model.ImageTaskStatusGenerating {
		t.Fatalf("expected fresh generating task to stay generating, got %s", freshReloaded.Status)
	}
	if pendingReloaded.Status != model.ImageTaskStatusPending {
		t.Fatalf("expected pending task to stay pending, got %s", pendingReloaded.Status)
	}
}

func TestRecoverExpiredImageGenerationTasksUsesConfiguredTimeoutWindow(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousTimeout := cfg.ImageTimeout
	previousRetries := cfg.MaxRetries
	previousDelay := cfg.RetryDelay
	t.Cleanup(func() {
		cfg.ImageTimeout = previousTimeout
		cfg.MaxRetries = previousRetries
		cfg.RetryDelay = previousDelay
	})

	cfg.ImageTimeout = 10
	cfg.MaxRetries = 1
	cfg.RetryDelay = 5

	now := common.GetTimestamp()
	expiryWindowSeconds := int64(imageGenerationTaskExpiryWindow() / time.Second)
	expiredTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-expired-window",
		Prompt:          "expired-window",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		StartedTime:     now - expiryWindowSeconds - 1,
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  now - 1,
	}
	freshTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-fresh-window",
		Prompt:          "fresh-window",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		StartedTime:     now - expiryWindowSeconds + 1,
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  now + expiryWindowSeconds,
	}
	for _, task := range []*model.ImageGenerationTask{expiredTask, freshTask} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create task %s: %v", task.ModelId, err)
		}
	}

	recovered := recoverExpiredImageGenerationTasks()
	if recovered != 1 {
		t.Fatalf("expected 1 recovered task, got %d", recovered)
	}

	expiredReloaded, _ := model.GetImageTaskByID(expiredTask.Id)
	freshReloaded, _ := model.GetImageTaskByID(freshTask.Id)
	if expiredReloaded.Status != model.ImageTaskStatusFailed {
		t.Fatalf("expected expired task to fail, got %s", expiredReloaded.Status)
	}
	if freshReloaded.Status != model.ImageTaskStatusGenerating {
		t.Fatalf("expected fresh task to remain generating, got %s", freshReloaded.Status)
	}
}

func TestProcessNextPendingImageGenerationTaskClaimsLease(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-lease",
		Prompt:          "lease prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	previousProcessFn := processImageGenerationTaskFn
	processImageGenerationTaskFn = func(taskId int) error {
		return nil
	}
	t.Cleanup(func() {
		processImageGenerationTaskFn = previousProcessFn
	})

	if !processNextPendingImageGenerationTask() {
		t.Fatal("expected pending task to be claimed")
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded.Status != model.ImageTaskStatusGenerating {
		t.Fatalf("expected task status generating, got %s", reloaded.Status)
	}
	if reloaded.WorkerNode != imageGenerationWorkerNodeID {
		t.Fatalf("expected worker node %q, got %q", imageGenerationWorkerNodeID, reloaded.WorkerNode)
	}
	if reloaded.LeaseExpiresAt <= reloaded.StartedTime {
		t.Fatalf("expected lease expiry after start time, start=%d lease=%d", reloaded.StartedTime, reloaded.LeaseExpiresAt)
	}
}

func TestClaimedResultUpdateRejectsWrongWorkerNode(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-claimed",
		Prompt:          "claimed prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		StartedTime:     common.GetTimestamp(),
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  common.GetTimestamp() + 300,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	won, err := model.UpdateImageTaskResultClaimed(task.Id, "worker-b", "url", "", "{}", 1)
	if err != nil {
		t.Fatalf("unexpected result update error: %v", err)
	}
	if won {
		t.Fatal("expected mismatched worker node to lose claimed result update")
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded.Status != model.ImageTaskStatusGenerating {
		t.Fatalf("expected task to remain generating, got %s", reloaded.Status)
	}
}

func TestProcessImageGenerationTaskRenewsLeaseWhileRunning(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "image-lease-renew-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-lease-renew")

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-lease-renew",
		ActualModel:       "gpt-image-lease-renew",
		DisplayName:       "GPT Image Lease Renew",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          user.Id,
		ModelId:         "gpt-image-lease-renew",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusPending,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousTimeout := cfg.ImageTimeout
	previousRenewOverride := imageGenerationLeaseRenewIntervalOverride
	previousGenerateFn := generateImageFn
	t.Cleanup(func() {
		cfg.ImageTimeout = previousTimeout
		imageGenerationLeaseRenewIntervalOverride = previousRenewOverride
		generateImageFn = previousGenerateFn
	})

	cfg.ImageTimeout = 3
	imageGenerationLeaseRenewIntervalOverride = func() time.Duration {
		return 200 * time.Millisecond
	}

	leaseObservations := make(chan [2]int64, 1)
	generateImageFn = func(ctx context.Context, task *model.ImageGenerationTask) (string, string, string, int, error) {
		reloaded, err := model.GetImageTaskByID(task.Id)
		if err != nil {
			return "", "", "", 0, err
		}
		firstLease := reloaded.LeaseExpiresAt
		time.Sleep(1300 * time.Millisecond)
		reloadedAgain, err := model.GetImageTaskByID(task.Id)
		if err != nil {
			return "", "", "", 0, err
		}
		leaseObservations <- [2]int64{firstLease, reloadedAgain.LeaseExpiresAt}
		time.Sleep(20 * time.Millisecond)
		return "https://example.com/image.png", "", `{}`, 1, nil
	}

	if err := ProcessImageGenerationTask(task.Id); err != nil {
		t.Fatalf("process task failed: %v", err)
	}

	leases := <-leaseObservations
	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded == nil || reloaded.Status != model.ImageTaskStatusSuccess {
		t.Fatalf("expected successful task, got %#v", reloaded)
	}
	if leases[0] == 0 {
		t.Fatal("expected initial lease to be recorded")
	}
	if leases[1] <= leases[0] {
		t.Fatalf("expected renewed lease to grow, before=%d after=%d", leases[0], leases[1])
	}
}

func TestRenewImageTaskLeaseRejectsWrongWorkerNode(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-renew-guard",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  common.GetTimestamp() + 10,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	renewed, err := model.RenewImageTaskLease(task.Id, "worker-b", common.GetTimestamp()+100)
	if err != nil {
		t.Fatalf("unexpected renew error: %v", err)
	}
	if renewed {
		t.Fatal("expected wrong worker node lease renew to fail")
	}
}

func TestValidateImageGenerationReferenceImagesUsesWorkerSetting(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousMaxImageSize := cfg.MaxImageSize
	t.Cleanup(func() {
		cfg.MaxImageSize = previousMaxImageSize
	})

	cfg.MaxImageSize = 1
	if err := validateImageGenerationReferenceImages(`{"reference_images":["data:image/png;base64,` + strings.Repeat("A", 1024) + `"]}`); err != nil {
		t.Fatalf("expected small reference image to pass, got %v", err)
	}

	if err := validateImageGenerationReferenceImages(`{"reference_images":["data:image/png;base64,` + strings.Repeat("A", 2*1024*1024) + `"]}`); err == nil {
		t.Fatal("expected oversized reference image to fail")
	}

	if err := validateImageGenerationReferenceImages(`{"mask":"data:image/png;base64,` + strings.Repeat("A", 1024) + `"}`); err != nil {
		t.Fatalf("expected small mask image to pass, got %v", err)
	}

	if err := validateImageGenerationReferenceImages(`{"mask":"data:image/png;base64,` + strings.Repeat("A", 2*1024*1024) + `"}`); err == nil {
		t.Fatal("expected oversized mask image to fail")
	}
}

func TestImageGenerationWorkerLimiterReadsCurrentMaxWorkers(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousMaxWorkers := cfg.MaxWorkers
	t.Cleanup(func() {
		cfg.MaxWorkers = previousMaxWorkers
	})

	limiter := &imageGenerationWorkerLimiter{}
	cfg.MaxWorkers = 1
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := limiter.acquire(ctx); err != nil {
		t.Fatalf("expected first worker slot, got %v", err)
	}

	blockedCtx, blockedCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer blockedCancel()
	if err := limiter.acquire(blockedCtx); err == nil {
		t.Fatal("expected second worker slot to block when max workers is 1")
	}

	cfg.MaxWorkers = 2
	if err := limiter.acquire(ctx); err != nil {
		t.Fatalf("expected updated max workers to allow second slot, got %v", err)
	}
	limiter.release()
	limiter.release()
}

func TestImageGenerationModelCapabilitiesValidation(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "image-capability-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-capability")

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-1",
		ActualModel:       "gpt-image-1",
		DisplayName:       "GPT Image",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	previousEnqueue := enqueueImageGenerationTask
	enqueueImageGenerationTask = func(taskId int) {}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
	})

	if _, err := CreateImageGenerationTask(user.Id, "gpt-image-1", "prompt", "openai", `{"reference_images":["data:image/png;base64,AAAA"]}`); err == nil {
		t.Fatal("expected image editing to be rejected when capability is missing")
	}

	if _, err := CreateImageGenerationTask(user.Id, "gpt-image-1", "prompt", "openai", `{"mask":"data:image/png;base64,AAAA"}`); err == nil {
		t.Fatal("expected mask-only edit request to be rejected")
	}

	task, err := CreateImageGenerationTask(user.Id, "gpt-image-1", "prompt", "openai", `{}`)
	if err != nil {
		t.Fatalf("expected generation without reference image to succeed: %v", err)
	}
	if task == nil || task.Id == 0 {
		t.Fatal("expected task to be created")
	}
	if task.Status != model.ImageTaskStatusPending {
		t.Fatalf("expected pending task, got %s", task.Status)
	}
	created, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if created == nil || created.RequestEndpoint != "openai" {
		t.Fatalf("unexpected created task: %#v", created)
	}
}

func TestCreateImageGenerationTaskRequiresValidUserToken(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "image-no-token-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-no-token",
		ActualModel:       "gpt-image-no-token",
		DisplayName:       "GPT Image No Token",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	previousEnqueue := enqueueImageGenerationTask
	enqueueImageGenerationTask = func(taskId int) {}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
	})

	if _, err := CreateImageGenerationTask(user.Id, "gpt-image-no-token", "prompt", "openai", `{}`); err == nil {
		t.Fatal("expected task creation to fail without valid user token")
	} else if !strings.Contains(err.Error(), "valid user token") {
		t.Fatalf("expected valid user token error, got %v", err)
	}

	activeCount, err := model.GetUserImageGenerationActiveTaskCount(user.Id)
	if err != nil {
		t.Fatalf("failed to read active queue count: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected no leaked active queue count, got %d", activeCount)
	}
}

func TestCreateImageGenerationTaskStoresReferenceImagesOutsideDatabase(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousStorageType := cfg.StorageType
	previousLocalPath := cfg.LocalStoragePath
	t.Cleanup(func() {
		cfg.StorageType = previousStorageType
		cfg.LocalStoragePath = previousLocalPath
	})
	cfg.StorageType = "local"
	cfg.LocalStoragePath = t.TempDir()

	user := &model.User{
		Username: "image-storage-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-storage")

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-edit",
		ActualModel:       "gpt-image-edit",
		DisplayName:       "GPT Image Edit",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation","image_editing"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	previousEnqueue := enqueueImageGenerationTask
	enqueueImageGenerationTask = func(taskId int) {}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
	})

	task, err := CreateImageGenerationTask(user.Id, "gpt-image-edit", "prompt", "openai", `{"reference_images":["`+source+`"],"resolution":"2K"}`)
	if err != nil {
		t.Fatalf("expected task creation to succeed: %v", err)
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected task to be reloaded")
	}
	if strings.Contains(reloaded.Params, "data:image/png;base64") {
		t.Fatalf("expected database params to avoid base64 reference image, got %s", reloaded.Params)
	}

	references, err := extractImageGenerationReferenceImages(reloaded.Params)
	if err != nil {
		t.Fatalf("failed to extract stored reference images: %v", err)
	}
	if len(references) != 1 {
		t.Fatalf("expected 1 stored reference image, got %v", references)
	}
	if !strings.HasPrefix(references[0], imageGenerationAssetURLPrefix) {
		t.Fatalf("expected stored reference image URL, got %q", references[0])
	}
	objectKey, ok := imageGenerationLocalAssetKeyFromURL(references[0])
	if !ok {
		t.Fatalf("expected local asset key from %q", references[0])
	}
	fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
	if err != nil {
		t.Fatalf("failed to resolve local reference image path: %v", err)
	}
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("expected stored reference image file to exist: %v", err)
	}
}

func TestCreateImageGenerationTaskReservesAndSuccessReleasesQueueSlot(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username: "image-queue-counter-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-queue-counter")

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-queue-counter",
		ActualModel:       "gpt-image-queue-counter",
		DisplayName:       "GPT Image Queue Counter",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	previousEnqueue := enqueueImageGenerationTask
	enqueueImageGenerationTask = func(taskId int) {}
	previousGenerateFn := generateImageFn
	generateImageFn = func(ctx context.Context, task *model.ImageGenerationTask) (string, string, string, int, error) {
		return "https://example.com/image.png", "", `{}`, 1, nil
	}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
		generateImageFn = previousGenerateFn
	})

	task, err := CreateImageGenerationTask(user.Id, "gpt-image-queue-counter", "prompt", "openai", `{}`)
	if err != nil {
		t.Fatalf("expected task creation to succeed: %v", err)
	}

	activeCount, err := model.GetUserImageGenerationActiveTaskCount(user.Id)
	if err != nil {
		t.Fatalf("failed to read active queue count after reserve: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected active queue count 1 after create, got %d", activeCount)
	}

	if err := ProcessImageGenerationTask(task.Id); err != nil {
		t.Fatalf("expected task processing to succeed: %v", err)
	}

	activeCount, err = model.GetUserImageGenerationActiveTaskCount(user.Id)
	if err != nil {
		t.Fatalf("failed to read active queue count after success: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected active queue count 0 after success, got %d", activeCount)
	}
}

func TestRecoverExpiredImageGenerationTasksReconcilesQueueCount(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	user := &model.User{
		Username:                  "image-recover-counter-user",
		Password:                  "hashed-password",
		Status:                    1,
		Group:                     "default",
		Quota:                     1000000,
		ImageGenerationActiveTasks: 1,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	now := common.GetTimestamp()
	task := &model.ImageGenerationTask{
		UserId:          user.Id,
		ModelId:         "gpt-image-recover-counter",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
		StartedTime:     now - 1000,
		WorkerNode:      "worker-a",
		LeaseExpiresAt:  now - 1,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	recovered := recoverExpiredImageGenerationTasks()
	if recovered != 1 {
		t.Fatalf("expected 1 recovered task, got %d", recovered)
	}

	activeCount, err := model.GetUserImageGenerationActiveTaskCount(user.Id)
	if err != nil {
		t.Fatalf("failed to read active queue count after recovery: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected active queue count 0 after recovery reconcile, got %d", activeCount)
	}
}

func TestCreateImageGenerationTaskStoresMaskOutsideDatabase(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousStorageType := cfg.StorageType
	previousLocalPath := cfg.LocalStoragePath
	t.Cleanup(func() {
		cfg.StorageType = previousStorageType
		cfg.LocalStoragePath = previousLocalPath
	})
	cfg.StorageType = "local"
	cfg.LocalStoragePath = t.TempDir()

	user := &model.User{
		Username: "image-mask-user",
		Password: "hashed-password",
		Status:   1,
		Group:    "default",
		Quota:    1000000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	seedUserToken(t, db, user.Id, "sk-image-mask")

	mapping := &model.ModelMapping{
		RequestModel:      "gpt-image-mask",
		ActualModel:       "gpt-image-mask",
		DisplayName:       "GPT Image Mask",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation","image_editing"]`,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create image mapping: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}
	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	previousEnqueue := enqueueImageGenerationTask
	enqueueImageGenerationTask = func(taskId int) {}
	t.Cleanup(func() {
		enqueueImageGenerationTask = previousEnqueue
	})

	task, err := CreateImageGenerationTask(user.Id, "gpt-image-mask", "prompt", "openai", `{"reference_images":["`+source+`"],"mask":"`+source+`","resolution":"2K"}`)
	if err != nil {
		t.Fatalf("expected task creation to succeed: %v", err)
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected task to be reloaded")
	}
	if strings.Contains(reloaded.Params, "data:image/png;base64") {
		t.Fatalf("expected database params to avoid base64 mask image, got %s", reloaded.Params)
	}

	mask, err := extractImageGenerationMask(reloaded.Params)
	if err != nil {
		t.Fatalf("failed to extract stored mask image: %v", err)
	}
	if !strings.HasPrefix(mask, imageGenerationAssetURLPrefix) {
		t.Fatalf("expected stored mask image URL, got %q", mask)
	}
	objectKey, ok := imageGenerationLocalAssetKeyFromURL(mask)
	if !ok {
		t.Fatalf("expected local asset key from %q", mask)
	}
	fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
	if err != nil {
		t.Fatalf("failed to resolve local mask image path: %v", err)
	}
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("expected stored mask image file to exist: %v", err)
	}
}

func TestModelMappingImageCapabilitiesNormalizeAndDefault(t *testing.T) {
	if got, err := model.NormalizeImageCapabilities(`["IMAGE_GENERATION","image_editing","image_generation"]`); err != nil {
		t.Fatalf("unexpected normalize error: %v", err)
	} else if got != `["image_generation","image_editing"]` {
		t.Fatalf("unexpected normalized capabilities: %s", got)
	}

	if got, err := model.EffectiveImageCapabilities(""); err != nil {
		t.Fatalf("unexpected effective capabilities error: %v", err)
	} else if len(got) != 2 || got[0] != model.ImageCapabilityGeneration || got[1] != model.ImageCapabilityEditing {
		t.Fatalf("unexpected default capabilities: %#v", got)
	}
}

func TestBuildOpenAIResponsesImageRequest(t *testing.T) {
	req, err := buildOpenAIResponsesImageRequest(&dto.ImageRequest{
		Model:       "gpt-image-1",
		Prompt:      "generate a skyline",
		Resolution:  "2K",
		AspectRatio: "16:9",
		Quality:     "high",
	})
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	if req.Model != "gpt-image-1" {
		t.Fatalf("unexpected model: %s", req.Model)
	}

	var input string
	if err := common.Unmarshal(req.Input, &input); err != nil {
		t.Fatalf("failed to decode string input: %v", err)
	}
	if input != "generate a skyline" {
		t.Fatalf("unexpected input: %q", input)
	}

	var tools []map[string]any
	if err := common.Unmarshal(req.Tools, &tools); err != nil {
		t.Fatalf("failed to decode tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected one tool, got %d", len(tools))
	}
	if got := tools[0]["type"]; got != "image_generation" {
		t.Fatalf("unexpected tool type: %#v", got)
	}
	if got := tools[0]["action"]; got != "generate" {
		t.Fatalf("unexpected tool action: %#v", got)
	}
	if got := tools[0]["size"]; got != "2048x1152" {
		t.Fatalf("unexpected tool size: %#v", got)
	}
	if got := tools[0]["quality"]; got != "high" {
		t.Fatalf("unexpected tool quality: %#v", got)
	}
}

func TestBuildOpenAIResponsesImageRequestWithReferenceImages(t *testing.T) {
	req, err := buildOpenAIResponsesImageRequest(&dto.ImageRequest{
		Model:           "gpt-image-1",
		Prompt:          "edit this image",
		Size:            "1536x1024",
		ReferenceImages: []string{"https://example.com/ref.png"},
	})
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	var input []map[string]any
	if err := common.Unmarshal(req.Input, &input); err != nil {
		t.Fatalf("failed to decode structured input: %v", err)
	}
	if len(input) != 1 {
		t.Fatalf("expected one input item, got %d", len(input))
	}

	content, ok := input[0]["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("unexpected content: %#v", input[0]["content"])
	}

	var tools []map[string]any
	if err := common.Unmarshal(req.Tools, &tools); err != nil {
		t.Fatalf("failed to decode tools: %v", err)
	}
	if got := tools[0]["action"]; got != "edit" {
		t.Fatalf("unexpected tool action: %#v", got)
	}
	if got := tools[0]["size"]; got != "1536x1024" {
		t.Fatalf("unexpected tool size: %#v", got)
	}
}

func TestNormalizeOpenAIResponsesImageResult(t *testing.T) {
	if got := normalizeOpenAIResponsesImageResult(" https://example.com/image.png "); got != "https://example.com/image.png" {
		t.Fatalf("unexpected URL normalization result: %q", got)
	}

	if got := normalizeOpenAIResponsesImageResult(" iVBORw0KGgoAAAANSUhEUgAAAAUA "); got != "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUA" {
		t.Fatalf("unexpected base64 normalization result: %q", got)
	}
}
