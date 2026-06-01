package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestRunCanvasAssetCleanupJobsOnceProcessesAndDeletesJob(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	previousHandler := handleCanvasAssetCleanupJobFn
	previousRunning := canvasAssetCleanupTaskRunning.Load()
	handleCanvasAssetCleanupJobFn = func(job *model.CanvasAssetCleanupJob) error {
		if job == nil || job.TaskRecordId != "123" {
			t.Fatalf("unexpected cleanup job: %#v", job)
		}
		return nil
	}
	canvasAssetCleanupTaskRunning.Store(false)
	t.Cleanup(func() {
		handleCanvasAssetCleanupJobFn = previousHandler
		canvasAssetCleanupTaskRunning.Store(previousRunning)
	})

	if err := model.CreateCanvasAssetCleanupJobs([]*model.CanvasAssetCleanupJob{
		{
			UserId:       1,
			TaskType:     model.CanvasTaskTypeImage,
			TaskRecordId: "123",
			Payload:      `{"task_id":123}`,
		},
	}); err != nil {
		t.Fatalf("failed to create cleanup job: %v", err)
	}

	runCanvasAssetCleanupJobsOnce()

	var count int64
	if err := db.Model(&model.CanvasAssetCleanupJob{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count cleanup jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected cleanup job to be deleted after success, count=%d", count)
	}
}

func TestRunCanvasAssetCleanupJobsOnceMarksJobFailedForRetry(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	previousHandler := handleCanvasAssetCleanupJobFn
	previousRunning := canvasAssetCleanupTaskRunning.Load()
	handleCanvasAssetCleanupJobFn = func(job *model.CanvasAssetCleanupJob) error {
		return fmt.Errorf("boom")
	}
	canvasAssetCleanupTaskRunning.Store(false)
	t.Cleanup(func() {
		handleCanvasAssetCleanupJobFn = previousHandler
		canvasAssetCleanupTaskRunning.Store(previousRunning)
	})

	if err := model.CreateCanvasAssetCleanupJobs([]*model.CanvasAssetCleanupJob{
		{
			UserId:       1,
			TaskType:     model.CanvasTaskTypeVideo,
			TaskRecordId: "456",
			Payload:      `{"task_id":456}`,
		},
	}); err != nil {
		t.Fatalf("failed to create cleanup job: %v", err)
	}

	beforeRun := common.GetTimestamp()
	runCanvasAssetCleanupJobsOnce()

	var job model.CanvasAssetCleanupJob
	if err := db.First(&job).Error; err != nil {
		t.Fatalf("failed to reload cleanup job: %v", err)
	}
	if job.Status != model.CanvasAssetCleanupJobStatusFailed {
		t.Fatalf("expected failed cleanup job status, got %q", job.Status)
	}
	if job.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", job.RetryCount)
	}
	if job.NextRunTime <= beforeRun {
		t.Fatalf("expected next run time to move forward, got %d <= %d", job.NextRunTime, beforeRun)
	}
	if job.LastError != "boom" {
		t.Fatalf("expected stored last error, got %q", job.LastError)
	}
}
