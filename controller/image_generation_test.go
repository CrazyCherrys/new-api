package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/worker_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type imageTaskDetailResponse struct {
	ID             int    `json:"id"`
	ModelID        string `json:"model_id"`
	DisplayName    string `json:"display_name"`
	SelectedGroup  string `json:"selected_group"`
	StartedTime    int64  `json:"started_time"`
	OutputWidth    int    `json:"output_width"`
	OutputHeight   int    `json:"output_height"`
	OutputSizeText string `json:"output_size_text"`
	SizeText       string `json:"size_text"`
	QualityText    string `json:"quality_text"`
	Quantity       int    `json:"quantity"`
}

func setupImageGenerationControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
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

	if err := db.AutoMigrate(&model.ImageGenerationTask{}, &model.ModelMapping{}); err != nil {
		t.Fatalf("failed to migrate image generation tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestGetImageGenerationTaskDetailReturnsComputedDetailFields(t *testing.T) {
	db := setupImageGenerationControllerTestDB(t)

	mapping := &model.ModelMapping{
		RequestModel:    "gpt-image-detail",
		ActualModel:     "gpt-image-detail",
		DisplayName:     "GPT Image Detail",
		ModelSeries:     "openai",
		ModelType:       2,
		Status:          1,
		RequestEndpoint: "openai",
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          42,
		ModelId:         "gpt-image-detail",
		SelectedGroup:   "vip",
		Prompt:          "detail prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		Params:          `{"aspect_ratio":"16:9","resolution":"2K","quality":"hd","style":"natural","n":3}`,
		ImageMetadata:   `{"width":2048,"height":1152}`,
		ImageUrl:        "https://example.com/detail.png",
		ThumbnailUrl:    "https://example.com/detail-thumb.png",
		CreatedTime:     100,
		CompletedTime:   130,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/image-generation/tasks/%d", task.Id), nil, task.UserId)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", task.Id)}}

	GetImageGenerationTaskDetail(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail imageTaskDetailResponse
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode image task detail response: %v", err)
	}

	if detail.DisplayName != mapping.DisplayName {
		t.Fatalf("expected display name %q, got %q", mapping.DisplayName, detail.DisplayName)
	}
	if detail.SelectedGroup != "vip" {
		t.Fatalf("expected selected group vip, got %q", detail.SelectedGroup)
	}
	if detail.StartedTime != task.CreatedTime {
		t.Fatalf("expected started time %d, got %d", task.CreatedTime, detail.StartedTime)
	}
	if detail.OutputWidth != 2048 || detail.OutputHeight != 1152 {
		t.Fatalf("expected output size 2048x1152, got %dx%d", detail.OutputWidth, detail.OutputHeight)
	}
	if detail.OutputSizeText != "2048x1152" {
		t.Fatalf("expected output size text 2048x1152, got %q", detail.OutputSizeText)
	}
	if detail.SizeText != "16:9 · 2K" {
		t.Fatalf("expected size text %q, got %q", "16:9 · 2K", detail.SizeText)
	}
	if detail.QualityText != "hd · 2K · natural" {
		t.Fatalf("expected quality text %q, got %q", "hd · 2K · natural", detail.QualityText)
	}
	if detail.Quantity != 3 {
		t.Fatalf("expected quantity 3, got %d", detail.Quantity)
	}
}

func TestDeleteImageGenerationTaskRejectsActiveTask(t *testing.T) {
	db := setupImageGenerationControllerTestDB(t)

	task := &model.ImageGenerationTask{
		UserId:          7,
		ModelId:         "gpt-image-active",
		Prompt:          "active prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusGenerating,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, fmt.Sprintf("/api/image-generation/tasks/%d", task.Id), nil, task.UserId)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", task.Id)}}

	DeleteImageGenerationTask(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected delete active task to fail")
	}
	if !strings.Contains(response.Message, "暂不支持删除") {
		t.Fatalf("expected active-task delete message, got %q", response.Message)
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected active task to remain in database")
	}
}

func TestDeleteImageGenerationTaskDeletesFinishedTask(t *testing.T) {
	db := setupImageGenerationControllerTestDB(t)

	task := &model.ImageGenerationTask{
		UserId:          9,
		ModelId:         "gpt-image-finished",
		Prompt:          "finished prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusFailed,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, fmt.Sprintf("/api/image-generation/tasks/%d", task.Id), nil, task.UserId)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", task.Id)}}

	DeleteImageGenerationTask(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected delete finished task to succeed, got message %q", response.Message)
	}

	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded != nil {
		t.Fatal("expected finished task to be deleted from database")
	}
}

func TestGetImageGenerationFileAllowsOwnerReferenceAssetFromTaskParams(t *testing.T) {
	db := setupImageGenerationControllerTestDB(t)

	cfg := worker_setting.GetWorkerSetting()
	previousStorageType := cfg.StorageType
	previousLocalPath := cfg.LocalStoragePath
	previousReferenceStorageType := cfg.ReferenceStorageType
	previousReferenceLocalPath := cfg.ReferenceLocalStoragePath
	service.InvalidateImageGenerationLocalAssetAccessCache()
	t.Cleanup(func() {
		cfg.StorageType = previousStorageType
		cfg.LocalStoragePath = previousLocalPath
		cfg.ReferenceStorageType = previousReferenceStorageType
		cfg.ReferenceLocalStoragePath = previousReferenceLocalPath
		service.InvalidateImageGenerationLocalAssetAccessCache()
	})

	cfg.StorageType = "local"
	cfg.LocalStoragePath = t.TempDir()
	cfg.ReferenceStorageType = "local"
	cfg.ReferenceLocalStoragePath = t.TempDir()

	objectKey := "image-generation/ref/20260522/controller-reference.png"
	assetURL := "/api/image-generation/files/" + objectKey
	fullPath := filepath.Join(cfg.ReferenceLocalStoragePath, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("failed to create reference asset directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("reference-data"), 0o644); err != nil {
		t.Fatalf("failed to write reference asset file: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          15,
		ModelId:         "gpt-image-controller-ref",
		Prompt:          "reference prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusFailed,
		Params:          `{"reference_images":["` + assetURL + `"]}`,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, assetURL, nil, task.UserId)
	ctx.Params = gin.Params{{Key: "path", Value: "/" + objectKey}}

	GetImageGenerationFile(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); body != "reference-data" {
		t.Fatalf("expected reference file body %q, got %q", "reference-data", body)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("expected content type image/png, got %q", contentType)
	}
}
