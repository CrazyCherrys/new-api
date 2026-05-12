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
	stored := storeImageGenerationResult(context.Background(), 123, source)
	if !strings.HasPrefix(stored.imageURL, imageGenerationAssetURLPrefix) {
		t.Fatalf("expected local asset URL, got %q", stored.imageURL)
	}
	if !strings.HasPrefix(stored.thumbnailURL, imageGenerationAssetURLPrefix) {
		t.Fatalf("expected local thumbnail URL, got %q", stored.thumbnailURL)
	}

	objectKey, ok := imageGenerationLocalAssetKeyFromURL(stored.imageURL)
	if !ok {
		t.Fatalf("expected local asset key from %q", stored.imageURL)
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

	thumbKey, ok := imageGenerationLocalAssetKeyFromURL(stored.thumbnailURL)
	if !ok {
		t.Fatalf("expected local thumbnail key from %q", stored.thumbnailURL)
	}
	thumbPath, err := imageGenerationLocalAssetPath(cfg, thumbKey)
	if err != nil {
		t.Fatalf("failed to resolve local thumbnail path: %v", err)
	}
	if _, err := os.Stat(thumbPath); err != nil {
		t.Fatalf("expected local thumbnail file to exist: %v", err)
	}
	thumbFile, thumbContentType, err := OpenImageGenerationLocalAsset(thumbKey)
	if err != nil {
		t.Fatalf("failed to open local thumbnail asset: %v", err)
	}
	_ = thumbFile.Close()
	if thumbContentType != "image/jpeg" {
		t.Fatalf("expected opaque thumbnail to use jpeg, got %q", thumbContentType)
	}
}

func TestStoreTransparentImageGenerationThumbnailLocally(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousStorageType := cfg.StorageType
	previousLocalPath := cfg.LocalStoragePath
	t.Cleanup(func() {
		cfg.StorageType = previousStorageType
		cfg.LocalStoragePath = previousLocalPath
	})

	cfg.StorageType = "local"
	cfg.LocalStoragePath = t.TempDir()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 0})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode transparent test image: %v", err)
	}

	source := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	stored := storeImageGenerationResult(context.Background(), 456, source)
	thumbKey, ok := imageGenerationLocalAssetKeyFromURL(stored.thumbnailURL)
	if !ok {
		t.Fatalf("expected local thumbnail key from %q", stored.thumbnailURL)
	}
	thumbFile, thumbContentType, err := OpenImageGenerationLocalAsset(thumbKey)
	if err != nil {
		t.Fatalf("failed to open transparent local thumbnail asset: %v", err)
	}
	_ = thumbFile.Close()
	if thumbContentType != "image/png" {
		t.Fatalf("expected transparent thumbnail to stay png, got %q", thumbContentType)
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

	thumbKey := "image-generation/thumb/20260428/123-thumb.jpg"
	thumbURL := buildImageGenerationLocalObjectURL(thumbKey)
	thumbPath, err := imageGenerationLocalAssetPath(cfg, thumbKey)
	if err != nil {
		t.Fatalf("failed to resolve local thumbnail path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0o755); err != nil {
		t.Fatalf("failed to create local thumbnail directory: %v", err)
	}
	if err := os.WriteFile(thumbPath, []byte("thumbnail"), 0o644); err != nil {
		t.Fatalf("failed to write local thumbnail file: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusFailed,
		ThumbnailUrl:    thumbURL,
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
	if _, err := os.Stat(thumbPath); !os.IsNotExist(err) {
		t.Fatalf("expected stored thumbnail file to be removed, stat err=%v", err)
	}
	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded != nil {
		t.Fatalf("expected task to be deleted, got %#v", reloaded)
	}
}

func TestDeleteImageGenerationTaskRemovesStoredMaskImages(t *testing.T) {
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

	objectKey := "image-generation/ref/20260428/124-mask.png"
	maskURL := buildImageGenerationLocalObjectURL(objectKey)
	fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
	if err != nil {
		t.Fatalf("failed to resolve local mask path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("failed to create local mask directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte("mask"), 0o644); err != nil {
		t.Fatalf("failed to write local mask file: %v", err)
	}

	task := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "prompt",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusFailed,
		Params:          `{"mask":"` + maskURL + `"}`,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if err := DeleteImageGenerationTask(task); err != nil {
		t.Fatalf("failed to delete image generation task: %v", err)
	}
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("expected stored mask image file to be removed, stat err=%v", err)
	}
	reloaded, err := model.GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded != nil {
		t.Fatalf("expected task to be deleted, got %#v", reloaded)
	}
}

func TestBuildImageGenerationObjectURLUsesCDNBaseURL(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousEndpoint := cfg.S3Endpoint
	previousBucket := cfg.S3Bucket
	previousURLMode := cfg.S3URLMode
	previousPublicBaseURL := cfg.S3PublicBaseURL
	t.Cleanup(func() {
		cfg.S3Endpoint = previousEndpoint
		cfg.S3Bucket = previousBucket
		cfg.S3URLMode = previousURLMode
		cfg.S3PublicBaseURL = previousPublicBaseURL
	})

	cfg.S3Endpoint = "https://oss-cn-hongkong-internal.aliyuncs.com"
	cfg.S3Bucket = "image-bucket"
	cfg.S3URLMode = "cdn"
	cfg.S3PublicBaseURL = "https://img.example.com"

	actualURL := buildImageGenerationObjectURL(cfg, "image-generation/20260510/test image.png")
	expectedURL := "https://img.example.com/image-generation/20260510/test%20image.png"
	if actualURL != expectedURL {
		t.Fatalf("expected CDN URL %q, got %q", expectedURL, actualURL)
	}
}

func TestImageGenerationS3ObjectKeyFromCDNURL(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousEndpoint := cfg.S3Endpoint
	previousBucket := cfg.S3Bucket
	previousURLMode := cfg.S3URLMode
	previousPublicBaseURL := cfg.S3PublicBaseURL
	t.Cleanup(func() {
		cfg.S3Endpoint = previousEndpoint
		cfg.S3Bucket = previousBucket
		cfg.S3URLMode = previousURLMode
		cfg.S3PublicBaseURL = previousPublicBaseURL
	})

	cfg.S3Endpoint = "https://oss-cn-hongkong-internal.aliyuncs.com"
	cfg.S3Bucket = "image-bucket"
	cfg.S3URLMode = "cdn"
	cfg.S3PublicBaseURL = "https://img.example.com/static"

	objectKey, ok := imageGenerationS3ObjectKeyFromURL(
		"https://img.example.com/static/image-generation/20260510/test%20image.png",
		cfg,
	)
	if !ok {
		t.Fatal("expected CDN URL to map back to object key")
	}

	expectedKey := "image-generation/20260510/test image.png"
	if objectKey != expectedKey {
		t.Fatalf("expected object key %q, got %q", expectedKey, objectKey)
	}
}

func TestImageGenerationS3ObjectKeyFromLegacyPublicOSSURL(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousEndpoint := cfg.S3Endpoint
	previousBucket := cfg.S3Bucket
	t.Cleanup(func() {
		cfg.S3Endpoint = previousEndpoint
		cfg.S3Bucket = previousBucket
	})

	cfg.S3Endpoint = "https://oss-cn-hongkong-internal.aliyuncs.com"
	cfg.S3Bucket = "image-bucket"

	objectKey, ok := imageGenerationS3ObjectKeyFromURL(
		"https://oss-cn-hongkong.aliyuncs.com/image-bucket/image-generation/20260510/test.png",
		cfg,
	)
	if !ok {
		t.Fatal("expected legacy public OSS URL to map back to object key")
	}
	if objectKey != "image-generation/20260510/test.png" {
		t.Fatalf("unexpected object key %q", objectKey)
	}
}

func TestValidateImageS3ConfigRequiresPublicBaseURLForCDNMode(t *testing.T) {
	cfg := worker_setting.GetWorkerSetting()
	previousEndpoint := cfg.S3Endpoint
	previousBucket := cfg.S3Bucket
	previousAccessKey := cfg.S3AccessKey
	previousSecretKey := cfg.S3SecretKey
	previousURLMode := cfg.S3URLMode
	previousPublicBaseURL := cfg.S3PublicBaseURL
	t.Cleanup(func() {
		cfg.S3Endpoint = previousEndpoint
		cfg.S3Bucket = previousBucket
		cfg.S3AccessKey = previousAccessKey
		cfg.S3SecretKey = previousSecretKey
		cfg.S3URLMode = previousURLMode
		cfg.S3PublicBaseURL = previousPublicBaseURL
	})

	cfg.S3Endpoint = "https://oss-cn-hongkong-internal.aliyuncs.com"
	cfg.S3Bucket = "image-bucket"
	cfg.S3AccessKey = "ak"
	cfg.S3SecretKey = "sk"
	cfg.S3URLMode = "cdn"
	cfg.S3PublicBaseURL = ""

	err := validateImageS3Config(cfg)
	if err == nil || !strings.Contains(err.Error(), "s3 public base url is empty") {
		t.Fatalf("expected missing public base url validation error, got %v", err)
	}
}
