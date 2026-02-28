package model

import (
	"database/sql/driver"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageGenerationTask 图片生成任务模型
type ImageGenerationTask struct {
	ID             string    `json:"id" gorm:"type:varchar(191);primaryKey"`
	UserID         int       `json:"user_id" gorm:"index:idx_image_tasks_user_created"`
	ModelID        string    `json:"model_id" gorm:"type:varchar(191)"`
	Prompt         string    `json:"prompt" gorm:"type:text"`
	Resolution     string    `json:"resolution" gorm:"type:varchar(20)"` // "1K", "2K", "4K"
	AspectRatio    string    `json:"aspect_ratio" gorm:"type:varchar(20)"` // "1:1", "16:9", etc.
	ReferenceImage string    `json:"reference_image" gorm:"type:text"`
	Count          int       `json:"count" gorm:"default:1"`
	Status         string    `json:"status" gorm:"type:varchar(20);index:idx_image_tasks_status_next_attempt"` // "pending", "running", "succeeded", "failed"
	ErrorMessage   string    `json:"error_message" gorm:"type:text"`
	ImageURLs      ImageURLs `json:"image_urls" gorm:"type:text"`
	Attempts       int       `json:"attempts" gorm:"default:0"`
	LastError      string    `json:"last_error" gorm:"type:text"`
	NextAttemptAt  *int64    `json:"next_attempt_at" gorm:"index:idx_image_tasks_status_next_attempt"`
	CreatedAt      int64     `json:"created_at" gorm:"index:idx_image_tasks_user_created"`
	UpdatedAt      int64     `json:"updated_at"`
	CompletedAt    *int64    `json:"completed_at"`
}

// ImageURLs 图片URL列表类型，用于存储生成的图片URL
type ImageURLs []string

// Scan 实现 sql.Scanner 接口，用于从数据库读取 JSON 数据
func (u *ImageURLs) Scan(val interface{}) error {
	if val == nil {
		*u = ImageURLs{}
		return nil
	}

	bytesValue, ok := val.([]byte)
	if !ok {
		*u = ImageURLs{}
		return nil
	}

	if len(bytesValue) == 0 {
		*u = ImageURLs{}
		return nil
	}

	return common.Unmarshal(bytesValue, u)
}

// Value 实现 driver.Valuer 接口，用于将数据写入数据库
func (u ImageURLs) Value() (driver.Value, error) {
	if len(u) == 0 {
		return nil, nil
	}
	return common.Marshal(u)
}

// TableName 指定表名
func (ImageGenerationTask) TableName() string {
	return "image_generation_tasks"
}

// BeforeCreate GORM 钩子，在创建记录前设置时间戳
func (t *ImageGenerationTask) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	if t.UpdatedAt == 0 {
		t.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate GORM 钩子，在更新记录前设置时间戳
func (t *ImageGenerationTask) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = time.Now().Unix()
	return nil
}

// Insert 插入新任务
func (t *ImageGenerationTask) Insert() error {
	return DB.Create(t).Error
}

// Update 更新任务
func (t *ImageGenerationTask) Update() error {
	return DB.Save(t).Error
}

// GetImageTaskByID 根据ID获取任务
func GetImageTaskByID(id string) (*ImageGenerationTask, error) {
	var task ImageGenerationTask
	err := DB.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetImageTasksByUserID 获取用户的任务列表
func GetImageTasksByUserID(userID int, offset, limit int) ([]*ImageGenerationTask, error) {
	var tasks []*ImageGenerationTask
	err := DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetPendingImageTasks 获取待处理的任务
func GetPendingImageTasks(limit int) ([]*ImageGenerationTask, error) {
	var tasks []*ImageGenerationTask
	now := time.Now().Unix()
	err := DB.Where("status = ?", "pending").
		Or("status = ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ?", "failed", now).
		Order("created_at ASC").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// CountImageTasksByUserID 统计用户的任务数量
func CountImageTasksByUserID(userID int) (int64, error) {
	var count int64
	err := DB.Model(&ImageGenerationTask{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// GetByTaskID 根据任务ID获取任务
func (t *ImageGenerationTask) GetByTaskID(taskID string) (*ImageGenerationTask, error) {
	var task ImageGenerationTask
	err := DB.Where("id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ListByUser 获取用户的任务列表，支持过滤和分页
func ListByUser(userID int, page, pageSize int, filters map[string]interface{}) ([]*ImageGenerationTask, int64, error) {
	var tasks []*ImageGenerationTask
	var total int64

	query := DB.Model(&ImageGenerationTask{}).Where("user_id = ?", userID)

	// 应用过滤条件
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if model, ok := filters["model"].(string); ok && model != "" {
		query = query.Where("model_id = ?", model)
	}
	if startTime, ok := filters["start_time"].(int64); ok && startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime, ok := filters["end_time"].(int64); ok && endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&tasks).Error

	return tasks, total, err
}

// ClaimNextPending 原子地获取并锁定下一个待处理任务
func ClaimNextPending() (*ImageGenerationTask, error) {
	now := time.Now().Unix()

	// PostgreSQL 和 MySQL 8+ 支持 FOR UPDATE SKIP LOCKED
	if common.UsingPostgreSQL || (common.UsingMySQL && !common.UsingSQLite) {
		var task ImageGenerationTask
		err := DB.Transaction(func(tx *gorm.DB) error {
			// 查询待处理的任务
			err := tx.Where("status = ?", "pending").
				Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
				Order("created_at ASC").
				Limit(1).
				Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				First(&task).Error

			if err != nil {
				return err
			}

			// 更新状态为运行中
			task.Status = "running"
			task.UpdatedAt = time.Now().Unix()
			return tx.Save(&task).Error
		})

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			return nil, err
		}
		return &task, nil
	}

	// SQLite 和 MySQL 5.7 使用 CAS 方式
	var task ImageGenerationTask
	err := DB.Where("status = ?", "pending").
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Order("created_at ASC").
		First(&task).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// 尝试原子更新
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ?", task.ID).
		Where("status = ?", "pending").
		Updates(map[string]interface{}{
			"status":     "running",
			"updated_at": time.Now().Unix(),
		})

	if result.Error != nil {
		return nil, result.Error
	}

	// 如果没有更新任何行，说明被其他进程抢占了
	if result.RowsAffected == 0 {
		return nil, nil
	}

	// 重新查询获取更新后的任务
	err = DB.Where("id = ?", task.ID).First(&task).Error
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// MarkRunning 标记任务为运行中
func (t *ImageGenerationTask) MarkRunning(taskID string) error {
	return DB.Model(&ImageGenerationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":     "running",
			"updated_at": time.Now().Unix(),
		}).Error
}

// MarkSuccess 标记任务成功，保存图片URL
func (t *ImageGenerationTask) MarkSuccess(taskID string, imageURLs []string) error {
	now := time.Now().Unix()
	return DB.Model(&ImageGenerationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":       "succeeded",
			"image_urls":   ImageURLs(imageURLs),
			"updated_at":   now,
			"completed_at": now,
		}).Error
}

// MarkFailed 标记任务失败
func (t *ImageGenerationTask) MarkFailed(taskID string, errorMsg string) error {
	now := time.Now().Unix()
	return DB.Model(&ImageGenerationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":         "failed",
			"error_message":  errorMsg,
			"last_error":     errorMsg,
			"updated_at":     now,
			"completed_at":   now,
			"next_attempt_at": nil,
		}).Error
}

// ScheduleRetry 安排任务重试
func (t *ImageGenerationTask) ScheduleRetry(taskID string, nextAttemptAt time.Time, errorMsg string) error {
	nextAttempt := nextAttemptAt.Unix()
	return DB.Model(&ImageGenerationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":          "pending",
			"attempts":        gorm.Expr("attempts + 1"),
			"last_error":      errorMsg,
			"next_attempt_at": nextAttempt,
			"updated_at":      time.Now().Unix(),
		}).Error
}

// ResetZombieRunning 重置超时的运行中任务
func ResetZombieRunning(staleAfter time.Duration) (int64, error) {
	staleTime := time.Now().Add(-staleAfter).Unix()
	now := time.Now().Unix()

	result := DB.Model(&ImageGenerationTask{}).
		Where("status = ?", "running").
		Where("updated_at < ?", staleTime).
		Updates(map[string]interface{}{
			"status":          "pending",
			"next_attempt_at": now,
			"updated_at":      now,
		})

	return result.RowsAffected, result.Error
}

// ToDTO 将 ImageGenerationTask 转换为 DTO 响应格式
func (t *ImageGenerationTask) ToDTO() map[string]interface{} {
	return map[string]interface{}{
		"task_id":         t.ID,
		"user_id":         t.UserID,
		"model":           t.ModelID,
		"prompt":          t.Prompt,
		"resolution":      t.Resolution,
		"aspect_ratio":    t.AspectRatio,
		"reference_image": t.ReferenceImage,
		"count":           t.Count,
		"status":          t.Status,
		"error_message":   t.ErrorMessage,
		"image_urls":      t.ImageURLs,
		"attempts":        t.Attempts,
		"created_at":      t.CreatedAt,
		"updated_at":      t.UpdatedAt,
		"completed_at":    t.CompletedAt,
	}
}

// TaskListToDTO 批量转换任务列表为 DTO 格式
func TaskListToDTO(tasks []*ImageGenerationTask) []map[string]interface{} {
	responses := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		responses = append(responses, task.ToDTO())
	}
	return responses
}
