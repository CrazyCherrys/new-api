package model

import (
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
