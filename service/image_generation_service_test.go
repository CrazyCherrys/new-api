package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
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

	if err := db.AutoMigrate(&model.ImageGenerationTask{}); err != nil {
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
