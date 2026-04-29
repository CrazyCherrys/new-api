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

func TestSubmitImageAssetToCreativeSpaceValidatesOwnershipAndDuplicates(t *testing.T) {
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

	submission, err := SubmitImageAssetToCreativeSpace(1, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("expected owned success asset to be submitted: %v", err)
	}
	if submission.Status != CreativeSubmissionStatusPending || submission.UserId != 1 || submission.TaskId != ownedSuccess.Id {
		t.Fatalf("unexpected submission: %#v", submission)
	}

	duplicate, err := SubmitImageAssetToCreativeSpace(1, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("expected duplicate submit to return existing submission: %v", err)
	}
	if duplicate.Id != submission.Id {
		t.Fatalf("expected duplicate submission id %d, got %d", submission.Id, duplicate.Id)
	}

	if _, err := SubmitImageAssetToCreativeSpace(1, failedTask.Id); err == nil {
		t.Fatalf("expected failed task submission to be rejected")
	}
	if _, err := SubmitImageAssetToCreativeSpace(1, otherUserTask.Id); err == nil {
		t.Fatalf("expected other user's task submission to be rejected")
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

	assets, total, err := GetApprovedCreativeAssets(0, 10)
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

	detail, err := GetApprovedCreativeAssetByID(approvedSubmission.Id)
	if err != nil {
		t.Fatalf("failed to get creative asset detail: %v", err)
	}
	if detail == nil || detail.Params != approvedTask.Params {
		t.Fatalf("expected approved asset detail, got %#v", detail)
	}

	pendingDetail, err := GetApprovedCreativeAssetByID(submissions[1].Id)
	if err != nil {
		t.Fatalf("failed to get pending creative asset detail: %v", err)
	}
	if pendingDetail != nil {
		t.Fatalf("expected pending submission to be hidden, got %#v", pendingDetail)
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

	asset, err := GetImageAssetByID(1, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("failed to get image asset detail: %v", err)
	}
	if asset == nil || asset.Id != ownedSuccess.Id {
		t.Fatalf("expected owned asset detail, got %#v", asset)
	}

	asset, err = GetImageAssetByID(2, ownedSuccess.Id)
	if err != nil {
		t.Fatalf("failed to get image asset detail for other user: %v", err)
	}
	if asset != nil {
		t.Fatalf("expected asset to be hidden from other user, got %#v", asset)
	}
}
