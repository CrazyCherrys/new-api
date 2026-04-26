package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ImageGenerationTask 图片生成任务表
type ImageGenerationTask struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"index;not null"`
	ModelId         string `json:"model_id" gorm:"size:128;not null;index"`
	Prompt          string `json:"prompt" gorm:"type:text;not null"`
	RequestEndpoint string `json:"request_endpoint" gorm:"size:32;not null;index"` // openai, gemini, openai_mod
	Status          string `json:"status" gorm:"size:20;not null;index;default:'pending'"`
	Params          string `json:"params" gorm:"type:text"`          // JSON: size, quality, style, n, etc.
	ImageUrl        string `json:"image_url" gorm:"type:text"`       // 生成的图片URL
	ImageMetadata   string `json:"image_metadata" gorm:"type:text"`  // JSON: revised_prompt, etc.
	ErrorMessage    string `json:"error_message" gorm:"type:text"`   // 错误信息
	Cost            int    `json:"cost" gorm:"default:0"`            // 消耗的配额
	CreatedTime     int64  `json:"created_time" gorm:"bigint;index"` // 创建时间戳
	CompletedTime   int64  `json:"completed_time" gorm:"bigint"`     // 完成时间戳
}

// 任务状态常量
const (
	ImageTaskStatusPending    = "pending"
	ImageTaskStatusGenerating = "generating"
	ImageTaskStatusSuccess    = "success"
	ImageTaskStatusFailed     = "failed"
)

// Insert 插入新任务
func (task *ImageGenerationTask) Insert() error {
	task.CreatedTime = common.GetTimestamp()
	if task.Status == "" {
		task.Status = ImageTaskStatusPending
	}
	return DB.Create(task).Error
}

// Update 更新任务
func (task *ImageGenerationTask) Update() error {
	return DB.Model(task).Updates(task).Error
}

// GetImageTaskByID 根据ID获取任务
func GetImageTaskByID(id int) (*ImageGenerationTask, error) {
	var task ImageGenerationTask
	err := DB.First(&task, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &task, err
}

// ImageTaskQueryParams 任务查询参数
type ImageTaskQueryParams struct {
	Status          string
	ModelId         string
	RequestEndpoint string
	StartTime       int64
	EndTime         int64
	SortBy          string // created_time | completed_time | status
	SortOrder       string // asc | desc
}

// GetImageTasksByUserID 根据用户ID获取任务列表（分页+筛选+排序）
func GetImageTasksByUserID(userId int, startIdx int, num int, queryParams ImageTaskQueryParams) ([]*ImageGenerationTask, int64, error) {
	var tasks []*ImageGenerationTask
	var total int64

	query := DB.Model(&ImageGenerationTask{}).Where("user_id = ?", userId)

	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.ModelId != "" {
		query = query.Where("model_id = ?", queryParams.ModelId)
	}
	if queryParams.RequestEndpoint != "" {
		query = query.Where("request_endpoint = ?", queryParams.RequestEndpoint)
	}
	if queryParams.StartTime > 0 {
		query = query.Where("created_time >= ?", queryParams.StartTime)
	}
	if queryParams.EndTime > 0 {
		query = query.Where("created_time <= ?", queryParams.EndTime)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// 仅允许白名单字段，避免 SQL 注入
	sortField := "id"
	switch queryParams.SortBy {
	case "created_time":
		sortField = "created_time"
	case "completed_time":
		sortField = "completed_time"
	case "status":
		sortField = "status"
	}
	sortOrder := "DESC"
	if queryParams.SortOrder == "asc" {
		sortOrder = "ASC"
	}
	orderClause := sortField + " " + sortOrder
	if sortField != "id" {
		orderClause += ", id " + sortOrder
	}

	err = query.Order(orderClause).Limit(num).Offset(startIdx).Find(&tasks).Error
	return tasks, total, err
}

// DeleteImageTask 删除任务
func DeleteImageTask(id int) error {
	return DB.Delete(&ImageGenerationTask{}, id).Error
}

// GetPendingImageTasks 获取所有待处理的任务
func GetPendingImageTasks(limit int) ([]*ImageGenerationTask, error) {
	var tasks []*ImageGenerationTask
	err := DB.Where("status = ?", ImageTaskStatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

// UpdateImageTaskStatus 更新任务状态
func UpdateImageTaskStatus(id int, status string, errorMessage string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	if status == ImageTaskStatusSuccess || status == ImageTaskStatusFailed {
		updates["completed_time"] = common.GetTimestamp()
	}
	return DB.Model(&ImageGenerationTask{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateImageTaskResult 更新任务结果
func UpdateImageTaskResult(id int, imageUrl string, imageMetadata string, cost int) error {
	updates := map[string]interface{}{
		"status":         ImageTaskStatusSuccess,
		"image_url":      imageUrl,
		"image_metadata": imageMetadata,
		"cost":           cost,
		"completed_time": common.GetTimestamp(),
	}
	return DB.Model(&ImageGenerationTask{}).Where("id = ?", id).Updates(updates).Error
}
