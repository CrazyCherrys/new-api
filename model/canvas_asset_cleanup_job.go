package model

import (
	"sort"
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

func canvasAssetCleanupJobMode(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case CanvasTaskTypeImage:
		return CanvasModeImage
	case CanvasTaskTypeVideo:
		return CanvasModeVideo
	default:
		return ""
	}
}

func canvasAssetCleanupJobDB(taskType string) (*gorm.DB, error) {
	mode := canvasAssetCleanupJobMode(taskType)
	if mode == "" {
		return nil, nil
	}
	return canvasModeDataDB(mode)
}

func createCanvasAssetCleanupJobsRouted(prepared []*CanvasAssetCleanupJob) error {
	grouped := make(map[string][]*CanvasAssetCleanupJob, 2)
	orderedTaskTypes := make([]string, 0, 2)
	for _, job := range prepared {
		if job == nil {
			continue
		}
		taskType := strings.TrimSpace(job.TaskType)
		if taskType == "" {
			return CreateCanvasAssetCleanupJobsWithDB(DB, prepared)
		}
		if _, ok := grouped[taskType]; !ok {
			orderedTaskTypes = append(orderedTaskTypes, taskType)
		}
		grouped[taskType] = append(grouped[taskType], job)
	}

	created := make([]*CanvasAssetCleanupJob, 0, len(prepared))
	for _, taskType := range orderedTaskTypes {
		db, err := canvasAssetCleanupJobDB(taskType)
		if err != nil {
			_ = DeleteCanvasAssetCleanupJobs(created)
			return err
		}
		if db == nil {
			db = DB
		}
		group := grouped[taskType]
		if len(group) == 0 {
			continue
		}
		if err := db.Create(group).Error; err != nil {
			_ = DeleteCanvasAssetCleanupJobs(created)
			return err
		}
		created = append(created, group...)
	}
	return nil
}

func CreateCanvasAssetCleanupJobsWithDB(db *gorm.DB, jobs []*CanvasAssetCleanupJob) error {
	prepared := prepareCanvasAssetCleanupJobs(jobs)
	if len(prepared) == 0 {
		return nil
	}
	if db == nil {
		return createCanvasAssetCleanupJobsRouted(prepared)
	}
	return db.Create(prepared).Error
}

func CreateCanvasAssetCleanupJobs(jobs []*CanvasAssetCleanupJob) error {
	return CreateCanvasAssetCleanupJobsWithDB(nil, jobs)
}

func ListClaimableCanvasAssetCleanupJobs(limit int, now int64) ([]*CanvasAssetCleanupJob, error) {
	if limit <= 0 {
		limit = 50
	}
	taskTypes := []string{CanvasTaskTypeImage, CanvasTaskTypeVideo}
	jobs := make([]*CanvasAssetCleanupJob, 0, limit*len(taskTypes))
	for _, taskType := range taskTypes {
		db, err := canvasAssetCleanupJobDB(taskType)
		if err != nil {
			if IsCanvasModeUnavailableError(err) {
				continue
			}
			return nil, err
		}
		if db == nil {
			db = DB
		}
		var partial []*CanvasAssetCleanupJob
		err = db.Where("task_type = ?", taskType).
			Where(
				"(((status IN ?) AND retry_count < max_retries AND next_run_time <= ?) OR (status = ? AND retry_count < max_retries AND lease_expires_at > 0 AND lease_expires_at <= ?))",
				[]string{CanvasAssetCleanupJobStatusPending, CanvasAssetCleanupJobStatusFailed},
				now,
				CanvasAssetCleanupJobStatusProcessing,
				now,
			).
			Order("next_run_time ASC").
			Order("id ASC").
			Limit(limit).
			Find(&partial).Error
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, partial...)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].NextRunTime == jobs[j].NextRunTime {
			return jobs[i].Id < jobs[j].Id
		}
		return jobs[i].NextRunTime < jobs[j].NextRunTime
	})
	if len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func ClaimCanvasAssetCleanupJob(taskType string, id int, now int64, leaseExpiresAt int64) (bool, error) {
	db, err := canvasAssetCleanupJobDB(taskType)
	if err != nil {
		return false, err
	}
	if db == nil {
		db = DB
	}
	result := db.Model(&CanvasAssetCleanupJob{}).
		Where(
			"id = ? AND task_type = ? AND (((status IN ?) AND retry_count < max_retries AND next_run_time <= ?) OR (status = ? AND retry_count < max_retries AND lease_expires_at > 0 AND lease_expires_at <= ?))",
			id,
			strings.TrimSpace(taskType),
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

func DeleteCanvasAssetCleanupJob(taskType string, id int) error {
	db, err := canvasAssetCleanupJobDB(taskType)
	if err != nil {
		return err
	}
	if db == nil {
		db = DB
	}
	return db.Where("task_type = ?", strings.TrimSpace(taskType)).Delete(&CanvasAssetCleanupJob{}, id).Error
}

func MarkCanvasAssetCleanupJobFailed(taskType string, id int, retryCount int, nextRunTime int64, lastError string) error {
	db, err := canvasAssetCleanupJobDB(taskType)
	if err != nil {
		return err
	}
	if db == nil {
		db = DB
	}
	return db.Model(&CanvasAssetCleanupJob{}).
		Where("id = ? AND task_type = ?", id, strings.TrimSpace(taskType)).
		Updates(map[string]any{
			"status":           CanvasAssetCleanupJobStatusFailed,
			"retry_count":      retryCount,
			"next_run_time":    nextRunTime,
			"lease_expires_at": 0,
			"last_error":       strings.TrimSpace(lastError),
			"updated_time":     common.GetTimestamp(),
		}).Error
}

func DeleteCanvasAssetCleanupJobs(jobs []*CanvasAssetCleanupJob) error {
	var firstErr error
	for _, job := range jobs {
		if job == nil || job.Id <= 0 {
			continue
		}
		if err := DeleteCanvasAssetCleanupJob(job.TaskType, job.Id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
