package model

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageTaskStatus 定义图像生成任务状态
type ImageTaskStatus string

const (
	ImageTaskStatusPending   ImageTaskStatus = "pending"
	ImageTaskStatusRunning   ImageTaskStatus = "running"
	ImageTaskStatusSucceeded ImageTaskStatus = "succeeded"
	ImageTaskStatusFailed    ImageTaskStatus = "failed"
)

// ImageTask 图像生成任务模型
// 独立于 Task 模型，专门用于 AI 图像生成功能
type ImageTask struct {
	ID             int64           `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt      int64           `json:"created_at" gorm:"index"`
	UpdatedAt      int64           `json:"updated_at"`
	UserID         int             `json:"user_id" gorm:"index"`
	ModelID        string          `json:"model_id" gorm:"type:varchar(200)"`
	Prompt         string          `json:"prompt" gorm:"type:text"`
	Resolution     string          `json:"resolution" gorm:"type:varchar(32)"`
	AspectRatio    string          `json:"aspect_ratio" gorm:"type:varchar(32)"`
	ReferenceImage string          `json:"reference_image" gorm:"type:text"`
	Count          int             `json:"count" gorm:"default:1"`
	Status         ImageTaskStatus `json:"status" gorm:"type:varchar(20);index"`
	ErrorMessage   string          `json:"error_message" gorm:"type:text"`
	ImageURLs      json.RawMessage `json:"image_urls" gorm:"type:json"` // 使用 json.RawMessage 确保跨DB兼容
	Attempts       int             `json:"attempts" gorm:"default:0"`
	LastError      string          `json:"last_error" gorm:"type:text"`
	NextAttemptAt  *int64          `json:"next_attempt_at" gorm:"index"` // Unix 时间戳，避免时区问题
	CompletedAt    *int64          `json:"completed_at"`
}

func (ImageTask) TableName() string {
	return "image_tasks"
}

// SetImageURLs 设置图片URL列表（使用 common.Marshal 确保兼容性）
func (t *ImageTask) SetImageURLs(urls []string) error {
	data, err := common.Marshal(urls)
	if err != nil {
		return err
	}
	t.ImageURLs = data
	return nil
}

// GetImageURLs 获取图片URL列表（使用 common.Unmarshal 确保兼容性）
func (t *ImageTask) GetImageURLs() ([]string, error) {
	var urls []string
	if len(t.ImageURLs) == 0 {
		return urls, nil
	}
	err := common.Unmarshal(t.ImageURLs, &urls)
	return urls, err
}

// ClaimNextPending 原子操作：获取下一个待处理任务并标记为 running
// 使用 FOR UPDATE SKIP LOCKED 确保并发安全
func ClaimNextPendingImageTask() (*ImageTask, error) {
	var task ImageTask
	now := common.GetTimestamp()

	err := DB.Transaction(func(tx *gorm.DB) error {
		// 查找待处理任务（pending 状态且到达重试时间）
		query := tx.Where("status = ?", ImageTaskStatusPending).
			Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
			Order("created_at ASC").
			Limit(1)

		// 使用 FOR UPDATE SKIP LOCKED 避免多个 Worker 竞争同一任务
		if common.UsingPostgreSQL || common.UsingMySQL {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}

		if err := query.First(&task).Error; err != nil {
			return err
		}

		// 更新状态为 running
		task.Status = ImageTaskStatusRunning
		task.Attempts++
		task.UpdatedAt = now

		return tx.Save(&task).Error
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 没有待处理任务
		}
		return nil, err
	}

	return &task, nil
}

// UpdateImageTaskRetry 更新任务重试信息
func UpdateImageTaskRetry(taskID int64, nextAttemptAt int64, lastError string) error {
	return DB.Model(&ImageTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":          ImageTaskStatusPending,
			"next_attempt_at": nextAttemptAt,
			"last_error":      lastError,
			"updated_at":      common.GetTimestamp(),
		}).Error
}

// UpdateImageTaskResult 更新任务结果
func UpdateImageTaskResult(taskID int64, status ImageTaskStatus, imageURLs []string, errorMessage string, completedAt *int64) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": common.GetTimestamp(),
	}

	if len(imageURLs) > 0 {
		data, err := common.Marshal(imageURLs)
		if err != nil {
			return err
		}
		updates["image_urls"] = data
	}

	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	if completedAt != nil {
		updates["completed_at"] = *completedAt
	}

	return DB.Model(&ImageTask{}).
		Where("id = ?", taskID).
		Updates(updates).Error
}

// ResetStaleImageTasks 重置僵尸任务（长时间处于 running 状态的任务）
func ResetStaleImageTasks(staleAfterSeconds int64) error {
	cutoff := common.GetTimestamp() - staleAfterSeconds

	return DB.Model(&ImageTask{}).
		Where("status = ?", ImageTaskStatusRunning).
		Where("updated_at < ?", cutoff).
		Updates(map[string]interface{}{
			"status":     ImageTaskStatusPending,
			"updated_at": common.GetTimestamp(),
		}).Error
}

// GetImageTaskByID 根据ID获取任务
func GetImageTaskByID(id int64) (*ImageTask, error) {
	var task ImageTask
	err := DB.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetImageTasksByUserID 获取用户的任务列表（分页）
func GetImageTasksByUserID(userID int, page, pageSize int, status, modelID, startTime, endTime string) ([]*ImageTask, int64, error) {
	var tasks []*ImageTask
	var total int64

	query := DB.Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if modelID != "" {
		query = query.Where("model_id = ?", modelID)
	}
	if startTime != "" && endTime != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startTime, endTime)
	}

	// 统计总数
	if err := query.Model(&ImageTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&tasks).Error

	return tasks, total, err
}

// DeleteImageTask 删除任务（软删除或硬删除）
func DeleteImageTask(id int64, userID int) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).
		Delete(&ImageTask{}).Error
}
