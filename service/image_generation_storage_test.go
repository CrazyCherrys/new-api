package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/worker_setting"
)

func TestStoreImageGenerationResultLocally(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousStorageType := cfg.StorageType
	previousLocalPath := cfg.LocalStoragePath
	t.Cleanup(func() {
		cfg.StorageType = previousStorageType
		cfg.LocalStoragePath = previousLocalPath
	})

	cfg.StorageType = "local"
	cfg.LocalStoragePath = t.TempDir()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	storedURL := storeImageGenerationResult(context.Background(), 123, source)
	if !strings.HasPrefix(storedURL, imageGenerationAssetURLPrefix) {
		t.Fatalf("expected local asset URL, got %q", storedURL)
	}

	objectKey, ok := imageGenerationLocalAssetKeyFromURL(storedURL)
	if !ok {
		t.Fatalf("expected local asset key from %q", storedURL)
	}
	fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
	if err != nil {
		t.Fatalf("failed to resolve local asset path: %v", err)
	}
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("expected local image file to exist: %v", err)
	}

	file, contentType, err := OpenImageGenerationLocalAsset(objectKey)
	if err != nil {
		t.Fatalf("failed to open local asset: %v", err)
	}
	_ = file.Close()
	if contentType != "image/png" {
		t.Fatalf("expected image/png content type, got %q", contentType)
	}
}

func TestCanAccessImageGenerationLocalAssetRequiresOwner(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)
	objectKey := "image-generation/20260428/123-test.png"
	assetURL := buildImageGenerationLocalObjectURL(objectKey)
	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "test prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		ImageUrl:        assetURL,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	allowed, err := CanAccessImageGenerationLocalAsset(1, objectKey)
	if err != nil {
		t.Fatalf("failed to check access: %v", err)
	}
	if !allowed {
		t.Fatal("expected owner to access local asset")
	}

	allowed, err = CanAccessImageGenerationLocalAsset(2, objectKey)
	if err != nil {
		t.Fatalf("failed to check access: %v", err)
	}
	if allowed {
		t.Fatal("expected non-owner to be denied")
	}

	allowed, err = CanAccessImageGenerationLocalAsset(1, "other/20260428/123-test.png")
	if err != nil {
		t.Fatalf("failed to check access: %v", err)
	}
	if allowed {
		t.Fatal("expected invalid path to be denied")
	}
}

func TestCanAccessApprovedInspirationLocalAssetRequiresApprovedSubmission(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)
	objectKey := "image-generation/20260428/123-approved.png"
	assetURL := buildImageGenerationLocalObjectURL(objectKey)
	approvedTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "approved prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		ImageUrl:        assetURL,
	}
	pendingTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "pending prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		ImageUrl:        buildImageGenerationLocalObjectURL("image-generation/20260428/456-pending.png"),
	}
	for _, task := range []*model.ImageGenerationTask{approvedTask, pendingTask} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}
	if err := db.Create(&model.ImageCreativeSubmission{
		TaskId: approvedTask.Id,
		UserId: approvedTask.UserId,
		Status: model.CreativeSubmissionStatusApproved,
	}).Error; err != nil {
		t.Fatalf("failed to create approved submission: %v", err)
	}
	if err := db.Create(&model.ImageCreativeSubmission{
		TaskId: pendingTask.Id,
		UserId: pendingTask.UserId,
		Status: model.CreativeSubmissionStatusPending,
	}).Error; err != nil {
		t.Fatalf("failed to create pending submission: %v", err)
	}

	allowed, err := CanAccessApprovedInspirationLocalAsset(objectKey)
	if err != nil {
		t.Fatalf("failed to check approved public access: %v", err)
	}
	if !allowed {
		t.Fatal("expected approved creative asset to be public")
	}

	allowed, err = CanAccessApprovedInspirationLocalAsset("image-generation/20260428/456-pending.png")
	if err != nil {
		t.Fatalf("failed to check pending public access: %v", err)
	}
	if allowed {
		t.Fatal("expected pending creative asset to stay private")
	}
}

func TestDeleteImageGenerationTaskRemovesStoredReferenceImages(t *testing.T) {
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

	objectKey := "image-generation/ref/20260428/123-reference.png"
	referenceURL := buildImageGenerationLocalObjectURL(objectKey)
	fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
	if err != nil {
		t.Fatalf("failed to resolve local reference path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("failed to create local reference directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("reference"), 0o644); err != nil {
		t.Fatalf("failed to write local reference file: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusFailed,
		Params:          `{"reference_images":["` + referenceURL + `"]}`,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if err := DeleteImageGenerationTask(task); err != nil {
		t.Fatalf("failed to delete image generation task: %v", err)
	}
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("expected stored reference image file to be removed, stat err=%v", err)
	}
	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded != nil {
		t.Fatalf("expected task to be deleted, got %#v", reloaded)
	}
}
