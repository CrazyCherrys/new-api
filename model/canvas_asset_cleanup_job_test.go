package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCanvasAssetCleanupJobTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	InitCommonColumnNames()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	if err := db.AutoMigrate(&CanvasAssetCleanupJob{}); err != nil {
		t.Fatalf("failed to migrate canvas asset cleanup job table: %v", err)
	}

	t.Cleanup(func() {
		DB = previousDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		InitCommonColumnNames()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestCreateCanvasAssetCleanupJobsDefaultsPendingFields(t *testing.T) {
	db := setupCanvasAssetCleanupJobTestDB(t)

	jobs := []*CanvasAssetCleanupJob{
		{
			UserId:       1,
			TaskType:     CanvasTaskTypeImage,
			TaskRecordId: "123",
			Payload:      `{"task_id":123}`,
		},
	}
	if err := CreateCanvasAssetCleanupJobs(jobs); err != nil {
		t.Fatalf("CreateCanvasAssetCleanupJobs returned error: %v", err)
	}

	var stored CanvasAssetCleanupJob
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("failed to reload cleanup job: %v", err)
	}
	if stored.Status != CanvasAssetCleanupJobStatusPending {
		t.Fatalf("expected pending status, got %q", stored.Status)
	}
	if stored.MaxRetries != 5 {
		t.Fatalf("expected default max retries 5, got %d", stored.MaxRetries)
	}
	if stored.NextRunTime <= 0 {
		t.Fatalf("expected next run time to be initialized, got %d", stored.NextRunTime)
	}
	if stored.CreatedTime <= 0 || stored.UpdatedTime <= 0 {
		t.Fatalf("expected timestamps to be initialized, got created=%d updated=%d", stored.CreatedTime, stored.UpdatedTime)
	}
}

func TestCanvasAssetCleanupJobsSplitModeRouteByTaskType(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&CanvasAssetCleanupJob{}); err != nil {
		t.Fatalf("failed to migrate main cleanup job table: %v", err)
	}

	imageChildDSN := fmt.Sprintf("file:%s_image_cleanup?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	imageChildDB, err := gorm.Open(sqlite.Open(imageChildDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open image child sqlite db: %v", err)
	}
	defer func() {
		sqlDB, dbErr := imageChildDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	}()

	videoChildDSN := fmt.Sprintf("file:%s_video_cleanup?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	videoChildDB, err := gorm.Open(sqlite.Open(videoChildDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open video child sqlite db: %v", err)
	}
	defer func() {
		sqlDB, dbErr := videoChildDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	}()

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		switch envName {
		case "CANVAS_IMAGE_SQL_DSN":
			return imageChildDB, nil
		case "CANVAS_VIDEO_SQL_DSN":
			return videoChildDB, nil
		default:
			t.Fatalf("unexpected dedicated DB open request: %s", envName)
			return nil, nil
		}
	}
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-cleanup")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video-cleanup")

	InitCanvasDBs()

	if !imageChildDB.Migrator().HasTable("canvas_asset_cleanup_jobs") || !videoChildDB.Migrator().HasTable("canvas_asset_cleanup_jobs") {
		t.Fatal("expected cleanup job tables after canvas DB init")
	}

	jobs := []*CanvasAssetCleanupJob{
		{UserId: 1, TaskType: CanvasTaskTypeImage, TaskRecordId: "img-1", Payload: `{"task_id":1}`},
		{UserId: 2, TaskType: CanvasTaskTypeVideo, TaskRecordId: "vid-2", Payload: `{"task_id":2}`},
	}
	if err := CreateCanvasAssetCleanupJobs(jobs); err != nil {
		t.Fatalf("CreateCanvasAssetCleanupJobs returned error: %v", err)
	}

	var imageCount int64
	if err := imageChildDB.Model(&CanvasAssetCleanupJob{}).Where("task_type = ?", CanvasTaskTypeImage).Count(&imageCount).Error; err != nil {
		t.Fatalf("failed to count image cleanup jobs in dedicated db: %v", err)
	}
	if imageCount != 1 {
		t.Fatalf("expected 1 image cleanup job in dedicated db, got %d", imageCount)
	}

	var videoCount int64
	if err := videoChildDB.Model(&CanvasAssetCleanupJob{}).Where("task_type = ?", CanvasTaskTypeVideo).Count(&videoCount).Error; err != nil {
		t.Fatalf("failed to count video cleanup jobs in dedicated db: %v", err)
	}
	if videoCount != 1 {
		t.Fatalf("expected 1 video cleanup job in dedicated db, got %d", videoCount)
	}

	var mainCount int64
	if err := db.Model(&CanvasAssetCleanupJob{}).Count(&mainCount).Error; err != nil {
		t.Fatalf("failed to count main cleanup jobs: %v", err)
	}
	if mainCount != 0 {
		t.Fatalf("expected 0 main-db cleanup jobs in split mode, got %d", mainCount)
	}

	if jobs[0].Id <= 0 || jobs[1].Id <= 0 {
		t.Fatalf("expected routed cleanup jobs to receive persisted ids, got %#v", jobs)
	}
	if claimed, err := ClaimCanvasAssetCleanupJob(CanvasTaskTypeImage, jobs[0].Id, common.GetTimestamp(), common.GetTimestamp()+30); err != nil || !claimed {
		t.Fatalf("failed to claim image cleanup job: claimed=%v err=%v", claimed, err)
	}
}
