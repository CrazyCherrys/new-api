package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	CanvasAssetCleanupJobStatusPending    = "pending"
	CanvasAssetCleanupJobStatusProcessing = "processing"
	CanvasAssetCleanupJobStatusFailed     = "failed"
)

type CanvasAssetCleanupJob struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	UserId         int    `json:"user_id" gorm:"index:idx_canvas_asset_cleanup_status_next,priority:4;not null"`
	TaskType       string `json:"task_type" gorm:"size:32;index;not null"`
	TaskRecordId   string `json:"task_record_id" gorm:"size:64;index;default:''"`
	Status         string `json:"status" gorm:"size:16;index:idx_canvas_asset_cleanup_status_next,priority:1;not null;default:'pending'"`
	Payload        string `json:"payload" gorm:"type:text"`
	RetryCount     int    `json:"retry_count" gorm:"default:0"`
	MaxRetries     int    `json:"max_retries" gorm:"default:5"`
	NextRunTime    int64  `json:"next_run_time" gorm:"bigint;index:idx_canvas_asset_cleanup_status_next,priority:2"`
	LeaseExpiresAt int64  `json:"lease_expires_at" gorm:"bigint;index:idx_canvas_asset_cleanup_status_next,priority:3"`
	LastError      string `json:"last_error" gorm:"type:text"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint;index"`
	UpdatedTime    int64  `json:"updated_time" gorm:"bigint"`
	StartedTime    int64  `json:"started_time" gorm:"bigint"`
}

func prepareCanvasAssetCleanupJobs(jobs []*CanvasAssetCleanupJob) []*CanvasAssetCleanupJob {
	now := common.GetTimestamp()
	prepared := make([]*CanvasAssetCleanupJob, 0, len(jobs))
	for _, job := range jobs {
		if job == nil {
			continue
		}
		job.Status = strings.TrimSpace(job.Status)
		if job.Status == "" {
			job.Status = CanvasAssetCleanupJobStatusPending
		}
		if job.NextRunTime <= 0 {
			job.NextRunTime = now
		}
		if job.MaxRetries <= 0 {
			job.MaxRetries = 5
		}
		if job.CreatedTime <= 0 {
			job.CreatedTime = now
		}
		job.UpdatedTime = now
		prepared = append(prepared, job)
	}
	return prepared
}

func CreateCanvasAssetCleanupJobsWithDB(db *gorm.DB, jobs []*CanvasAssetCleanupJob) error {
	if db == nil {
		db = DB
	}
	prepared := prepareCanvasAssetCleanupJobs(jobs)
	if len(prepared) == 0 {
		return nil
	}
	return db.Create(prepared).Error
}

func CreateCanvasAssetCleanupJobs(jobs []*CanvasAssetCleanupJob) error {
	return CreateCanvasAssetCleanupJobsWithDB(DB, jobs)
}

func ListClaimableCanvasAssetCleanupJobs(limit int, now int64) ([]*CanvasAssetCleanupJob, error) {
	if limit <= 0 {
		limit = 50
	}
	var jobs []*CanvasAssetCleanupJob
	err := DB.Where(
		"(((status IN ?) AND retry_count < max_retries AND next_run_time <= ?) OR (status = ? AND retry_count < max_retries AND lease_expires_at > 0 AND lease_expires_at <= ?))",
		[]string{CanvasAssetCleanupJobStatusPending, CanvasAssetCleanupJobStatusFailed},
		now,
		CanvasAssetCleanupJobStatusProcessing,
		now,
	).
		Order("next_run_time ASC").
		Order("id ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

func ClaimCanvasAssetCleanupJob(id int, now int64, leaseExpiresAt int64) (bool, error) {
	result := DB.Model(&CanvasAssetCleanupJob{}).
		Where(
			"id = ? AND (((status IN ?) AND retry_count < max_retries AND next_run_time <= ?) OR (status = ? AND retry_count < max_retries AND lease_expires_at > 0 AND lease_expires_at <= ?))",
			id,
			[]string{CanvasAssetCleanupJobStatusPending, CanvasAssetCleanupJobStatusFailed},
			now,
			CanvasAssetCleanupJobStatusProcessing,
			now,
		).
		Updates(map[string]any{
			"status":           CanvasAssetCleanupJobStatusProcessing,
			"started_time":     now,
			"updated_time":     now,
			"lease_expires_at": leaseExpiresAt,
			"last_error":       "",
		})
	return result.RowsAffected > 0, result.Error
}

func DeleteCanvasAssetCleanupJob(id int) error {
	return DB.Delete(&CanvasAssetCleanupJob{}, id).Error
}

func MarkCanvasAssetCleanupJobFailed(id int, retryCount int, nextRunTime int64, lastError string) error {
	return DB.Model(&CanvasAssetCleanupJob{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":           CanvasAssetCleanupJobStatusFailed,
			"retry_count":      retryCount,
			"next_run_time":    nextRunTime,
			"lease_expires_at": 0,
			"last_error":       strings.TrimSpace(lastError),
			"updated_time":     common.GetTimestamp(),
		}).Error
}
