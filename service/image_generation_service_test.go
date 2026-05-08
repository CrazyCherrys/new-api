package service

import (
	"context"
	"fmt"
	"strings"
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

	if err := db.AutoMigrate(&model.User{}, &model.ModelMapping{}, &model.ImageGenerationTask{}, &model.ImageCreativeSubmission{}); err != nil {
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

func TestRetryImageGenerationTaskResetsAndEnqueuesFailedTask(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)
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
	if len(enqueuedTaskIds) != 1 || enqueuedTaskIds[0] != task.Id {
		t.Fatalf("expected task %d to be enqueued once, got %v", task.Id, enqueuedTaskIds)
	}
}

func TestImageGenerationTimeoutUsesWorkerSetting(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousTimeout := cfg.ImageTimeout
	t.Cleanup(func() {
		cfg.ImageTimeout = previousTimeout
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
