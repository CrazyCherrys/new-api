package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupImageAssetTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
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
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&ImageGenerationTask{}, &ModelMapping{}, &ImageCreativeSubmission{}); err != nil {
		t.Fatalf("failed to migrate image asset tables: %v", err)
	}

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
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

func TestSubmitImageAssetToInspirationValidatesOwnershipAndDuplicates(t *testing.T) {
	db := setupImageAssetTestDB(t)

	ownedSuccess := &ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "public gallery candidate",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "https://example.com/public.png",
		CreatedTime:     1000,
		CompletedTime:   1100,
	}
	failedTask := &ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "failed candidate",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusFailed,
		ImageUrl:        "https://example.com/failed.png",
		CreatedTime:     1200,
	}
	otherUserTask := &ImageGenerationTask{
		UserId:          2,
		ModelId:         "gpt-image-1",
		Prompt:          "other candidate",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "https://example.com/other.png",
		CreatedTime:     1300,
	}
	for _, task := range []*ImageGenerationTask{ownedSuccess, failedTask, otherUserTask} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create image task: %v", err)
		}
	}

	submission, err := SubmitImageAssetToInspiration(1, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("expected owned success asset to be submitted: %v", err)
	}
	if submission.Status != CreativeSubmissionStatusPending || submission.UserId != 1 || submission.TaskId != ownedSuccess.Id {
		t.Fatalf("unexpected submission: %#v", submission)
	}

	duplicate, err := SubmitImageAssetToInspiration(1, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("expected duplicate submit to return existing submission: %v", err)
	}
	if duplicate.Id != submission.Id {
		t.Fatalf("expected duplicate submission id %d, got %d", submission.Id, duplicate.Id)
	}

	if _, err := SubmitImageAssetToInspiration(1, failedTask.Id); err == nil {
		t.Fatalf("expected failed task submission to be rejected")
	}
	if _, err := SubmitImageAssetToInspiration(1, otherUserTask.Id); err == nil {
		t.Fatalf("expected other user's task submission to be rejected")
	}
}

func TestDeleteImageCreativeSubmission(t *testing.T) {
	db := setupImageAssetTestDB(t)

	submission := &ImageCreativeSubmission{
		TaskId:        1,
		UserId:        1,
		Status:        CreativeSubmissionStatusPending,
		SubmittedTime: 1000,
	}
	if err := db.Create(submission).Error; err != nil {
		t.Fatalf("failed to create submission: %v", err)
	}

	if err := DeleteImageCreativeSubmission(submission.Id); err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}

	deleted, err := GetImageCreativeSubmissionByTaskID(submission.TaskId)
	if err != nil {
		t.Fatalf("failed to reload deleted submission: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected submission to be deleted, got %#v", deleted)
	}
}

func TestApprovedCreativeAssetsOnlyExposeReviewedSubmissions(t *testing.T) {
	db := setupImageAssetTestDB(t)

	mapping := &ModelMapping{
		RequestModel:    "gpt-image-1",
		ActualModel:     "gpt-image-1",
		DisplayName:     "GPT Image",
		ModelSeries:     "openai",
		ModelType:       2,
		Status:          1,
		RequestEndpoint: "openai",
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	approvedTask := &ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "approved prompt",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "https://example.com/approved.png",
		ThumbnailUrl:    "https://example.com/approved-thumb.jpg",
		Params:          `{"size":"1024x1024"}`,
		CreatedTime:     1000,
		CompletedTime:   1100,
	}
	pendingTask := &ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "pending prompt",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "https://example.com/pending.png",
		CreatedTime:     1200,
	}
	rejectedTask := &ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "rejected prompt",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "https://example.com/rejected.png",
		CreatedTime:     1300,
	}
	for _, task := range []*ImageGenerationTask{approvedTask, pendingTask, rejectedTask} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create image task: %v", err)
		}
	}

	approvedSubmission := &ImageCreativeSubmission{
		TaskId:        approvedTask.Id,
		UserId:        1,
		Status:        CreativeSubmissionStatusApproved,
		SubmittedTime: 2000,
		ReviewedTime:  2100,
		ReviewerId:    10,
	}
	submissions := []*ImageCreativeSubmission{
		approvedSubmission,
		{
			TaskId:        pendingTask.Id,
			UserId:        1,
			Status:        CreativeSubmissionStatusPending,
			SubmittedTime: 2200,
		},
		{
			TaskId:        rejectedTask.Id,
			UserId:        1,
			Status:        CreativeSubmissionStatusRejected,
			SubmittedTime: 2300,
			ReviewedTime:  2400,
			ReviewerId:    10,
			RejectReason:  "not suitable",
		},
	}
	for _, submission := range submissions {
		if err := db.Create(submission).Error; err != nil {
			t.Fatalf("failed to create creative submission: %v", err)
		}
	}

	assets, total, err := GetApprovedInspirationAssets(0, 10)
	if err != nil {
		t.Fatalf("failed to get creative assets: %v", err)
	}
	if total != 1 || len(assets) != 1 {
		t.Fatalf("expected one approved asset, total=%d len=%d", total, len(assets))
	}
	if assets[0].Id != approvedSubmission.Id || assets[0].Prompt != approvedTask.Prompt {
		t.Fatalf("unexpected approved asset: %#v", assets[0])
	}
	if assets[0].DisplayName != "GPT Image" || assets[0].ModelSeries != "openai" {
		t.Fatalf("expected model mapping metadata, got display=%q series=%q", assets[0].DisplayName, assets[0].ModelSeries)
	}
	if assets[0].ThumbnailUrl != approvedTask.ThumbnailUrl {
		t.Fatalf("expected thumbnail url %q, got %q", approvedTask.ThumbnailUrl, assets[0].ThumbnailUrl)
	}

	detail, err := GetApprovedInspirationAssetByID(approvedSubmission.Id)
	if err != nil {
		t.Fatalf("failed to get creative asset detail: %v", err)
	}
	if detail == nil || detail.Params != approvedTask.Params {
		t.Fatalf("expected approved asset detail, got %#v", detail)
	}
	if detail.ThumbnailUrl != approvedTask.ThumbnailUrl {
		t.Fatalf("expected detail thumbnail url %q, got %q", approvedTask.ThumbnailUrl, detail.ThumbnailUrl)
	}

	pendingDetail, err := GetApprovedInspirationAssetByID(submissions[1].Id)
	if err != nil {
		t.Fatalf("failed to get pending creative asset detail: %v", err)
	}
	if pendingDetail != nil {
		t.Fatalf("expected pending submission to be hidden, got %#v", pendingDetail)
	}
}

func TestSearchModelMappingsIncludesDisabledItemsForAdmin(t *testing.T) {
	setupImageAssetTestDB(t)

	records := []*ModelMapping{
		{
			RequestModel:    "enabled-image-model",
			ActualModel:     "enabled-image-model",
			DisplayName:     "Enabled Image Model",
			ModelSeries:     "openai",
			ModelType:       2,
			Status:          1,
			RequestEndpoint: "openai",
			CreatedTime:     100,
			UpdatedTime:     100,
		},
		{
			RequestModel:    "disabled-image-model",
			ActualModel:     "disabled-image-model",
			DisplayName:     "Disabled Image Model",
			ModelSeries:     "openai",
			ModelType:       2,
			Status:          0,
			RequestEndpoint: "openai",
			CreatedTime:     200,
			UpdatedTime:     200,
		},
	}
	for _, record := range records {
		if err := record.Insert(); err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}
	}

	mappings, total, err := SearchModelMappings("image-model", 2, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
}

func TestGetActiveImageModelMappingsFiltersDisabledAndMissingEndpoint(t *testing.T) {
	setupImageAssetTestDB(t)

	records := []*ModelMapping{
		{
			RequestModel:    "enabled-with-endpoint",
			ActualModel:     "enabled-with-endpoint",
			DisplayName:     "Enabled With Endpoint",
			ModelSeries:     "openai",
			ModelType:       2,
			Status:          1,
			RequestEndpoint: "openai",
		},
		{
			RequestModel:    "disabled-with-endpoint",
			ActualModel:     "disabled-with-endpoint",
			DisplayName:     "Disabled With Endpoint",
			ModelSeries:     "openai",
			ModelType:       2,
			Status:          0,
			RequestEndpoint: "openai",
		},
		{
			RequestModel:    "enabled-without-endpoint",
			ActualModel:     "enabled-without-endpoint",
			DisplayName:     "Enabled Without Endpoint",
			ModelSeries:     "openai",
			ModelType:       2,
			Status:          1,
			RequestEndpoint: "",
		},
	}
	for _, record := range records {
		if err := record.Insert(); err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}
	}

	mappings, total, err := GetActiveImageModelMappings(0, 10)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(mappings) != 1 || mappings[0].RequestModel != "enabled-with-endpoint" {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestModelMappingUpdatePreservesCreatedTime(t *testing.T) {
	setupImageAssetTestDB(t)

	record := &ModelMapping{
		RequestModel:      "source-model",
		ActualModel:       "target-model-v1",
		DisplayName:       "Source Model",
		ModelSeries:       "openai",
		ModelType:         2,
		Status:            1,
		Priority:          3,
		RequestEndpoint:   "openai",
		ImageCapabilities: `["image_generation"]`,
		CreatedTime:       12345,
		UpdatedTime:       12345,
	}
	if err := record.Insert(); err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}
	createdTime := record.CreatedTime

	record.ActualModel = "target-model-v2"
	record.Priority = 9
	record.Status = 0
	record.Description = "updated description"
	if err := record.Update(); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	reloaded, err := GetModelMapping(record.Id)
	if err != nil {
		t.Fatalf("failed to reload model mapping: %v", err)
	}
	if reloaded.CreatedTime != createdTime {
		t.Fatalf("expected created_time to be preserved, got %d", reloaded.CreatedTime)
	}
	if reloaded.ActualModel != "target-model-v2" {
		t.Fatalf("expected actual model to update, got %q", reloaded.ActualModel)
	}
	if reloaded.Priority != 9 || reloaded.Status != 0 {
		t.Fatalf("unexpected updated values: %#v", reloaded)
	}
}

func TestGetModelMappingByRequestModelAndActiveVariant(t *testing.T) {
	setupImageAssetTestDB(t)

	record := &ModelMapping{
		RequestModel:    "disabled-source-model",
		ActualModel:     "disabled-target-model",
		DisplayName:     "Disabled Source Model",
		ModelSeries:     "openai",
		ModelType:       2,
		Status:          0,
		RequestEndpoint: "openai",
	}
	if err := record.Insert(); err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	gotAny, err := GetModelMappingByRequestModel("disabled-source-model")
	if err != nil {
		t.Fatalf("failed to get mapping by request model: %v", err)
	}
	if gotAny == nil || gotAny.Id != record.Id {
		t.Fatalf("expected to find disabled record, got %#v", gotAny)
	}

	gotActive, err := GetActiveModelMappingByRequestModel("disabled-source-model")
	if err != nil {
		t.Fatalf("failed to get active mapping by request model: %v", err)
	}
	if gotActive != nil {
		t.Fatalf("expected no active mapping, got %#v", gotActive)
	}
}

func TestGetImageAssetsByUserIDFiltersSuccessfulOwnedImages(t *testing.T) {
	db := setupImageAssetTestDB(t)

	mapping := &ModelMapping{
		RequestModel:    "gpt-image-1",
		ActualModel:     "gpt-image-1",
		DisplayName:     "GPT Image",
		ModelSeries:     "openai",
		ModelType:       2,
		Status:          1,
		RequestEndpoint: "openai",
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	ownedSuccess := &ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-1",
		Prompt:          "sunset over a lake",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "https://example.com/image.png",
		ThumbnailUrl:    "https://example.com/image-thumb.jpg",
		Cost:            100,
		CreatedTime:     1000,
		CompletedTime:   1100,
	}
	records := []*ImageGenerationTask{
		ownedSuccess,
		{
			UserId:          1,
			ModelId:         "gpt-image-1",
			Prompt:          "failed sunset",
			RequestEndpoint: "openai",
			Status:          ImageTaskStatusFailed,
			ImageUrl:        "https://example.com/failed.png",
			CreatedTime:     2000,
		},
		{
			UserId:          1,
			ModelId:         "gpt-image-1",
			Prompt:          "empty image",
			RequestEndpoint: "openai",
			Status:          ImageTaskStatusSuccess,
			ImageUrl:        "",
			CreatedTime:     3000,
		},
		{
			UserId:          2,
			ModelId:         "gpt-image-1",
			Prompt:          "other user sunset",
			RequestEndpoint: "openai",
			Status:          ImageTaskStatusSuccess,
			ImageUrl:        "https://example.com/other.png",
			CreatedTime:     4000,
		},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("failed to create image task: %v", err)
		}
	}

	assets, total, stats, err := GetImageAssetsByUserID(1, 0, 10, ImageAssetQueryParams{
		Keyword:     "sunset",
		ModelSeries: "openai",
		SortBy:      "created_time",
		SortOrder:   "desc",
	})
	if err != nil {
		t.Fatalf("failed to get image assets: %v", err)
	}
	if total != 1 || stats.TotalAssets != 1 {
		t.Fatalf("expected one visible asset, total=%d stats=%d", total, stats.TotalAssets)
	}
	if stats.LatestCreatedTime != ownedSuccess.CreatedTime {
		t.Fatalf("expected latest created time %d, got %d", ownedSuccess.CreatedTime, stats.LatestCreatedTime)
	}
	if len(assets) != 1 {
		t.Fatalf("expected one asset item, got %d", len(assets))
	}
	if assets[0].TaskId != ownedSuccess.Id {
		t.Fatalf("expected task id %d, got %d", ownedSuccess.Id, assets[0].TaskId)
	}
	if assets[0].DisplayName != "GPT Image" || assets[0].ModelSeries != "openai" {
		t.Fatalf("expected model mapping metadata, got display=%q series=%q", assets[0].DisplayName, assets[0].ModelSeries)
	}
	if assets[0].ThumbnailUrl != ownedSuccess.ThumbnailUrl {
		t.Fatalf("expected thumbnail url %q, got %q", ownedSuccess.ThumbnailUrl, assets[0].ThumbnailUrl)
	}

	asset, err := GetImageAssetByID(1, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("failed to get image asset detail: %v", err)
	}
	if asset == nil || asset.Id != ownedSuccess.Id {
		t.Fatalf("expected owned asset detail, got %#v", asset)
	}
	if asset.ThumbnailUrl != ownedSuccess.ThumbnailUrl {
		t.Fatalf("expected asset thumbnail url %q, got %q", ownedSuccess.ThumbnailUrl, asset.ThumbnailUrl)
	}

	asset, err = GetImageAssetByID(2, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("failed to get image asset detail for other user: %v", err)
	}
	if asset != nil {
		t.Fatalf("expected asset to be hidden from other user, got %#v", asset)
	}
}
