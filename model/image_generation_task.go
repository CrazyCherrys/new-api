package model

import (
	"strings"

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

// ResetImageTaskForRetry 将失败任务重置为待处理状态，返回是否实际更新。
func ResetImageTaskForRetry(id int) (bool, error) {
	updates := map[string]interface{}{
		"status":         ImageTaskStatusPending,
		"error_message":  "",
		"completed_time": 0,
	}
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ? AND status = ?", id, ImageTaskStatusFailed).
		Updates(updates)
	return result.RowsAffected > 0, result.Error
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

// ImageAssetQueryParams 资产仓库查询参数。
type ImageAssetQueryParams struct {
	Keyword     string
	ModelId     string
	ModelSeries string
	StartTime   int64
	EndTime     int64
	SortBy      string // created_time | completed_time | cost
	SortOrder   string // asc | desc
}

// ImageGenerationAsset 图片生成资产视图，数据源仍为成功的图片生成任务。
type ImageGenerationAsset struct {
	Id              int    `json:"id"`
	TaskId          int    `json:"task_id"`
	UserId          int    `json:"user_id"`
	ModelId         string `json:"model_id"`
	DisplayName     string `json:"display_name"`
	ModelSeries     string `json:"model_series"`
	Prompt          string `json:"prompt"`
	RequestEndpoint string `json:"request_endpoint"`
	Params          string `json:"params"`
	ImageUrl        string `json:"image_url"`
	ImageMetadata   string `json:"image_metadata"`
	Cost            int    `json:"cost"`
	CreatedTime     int64  `json:"created_time"`
	CompletedTime   int64  `json:"completed_time"`
}

type ImageAssetStats struct {
	TotalAssets       int64 `json:"total_assets"`
	LatestCreatedTime int64 `json:"latest_created_time"`
}

type ImageAssetFilterOptions struct {
	Models []ImageAssetModelOption  `json:"models"`
	Series []ImageAssetSeriesOption `json:"series"`
}

type ImageAssetModelOption struct {
	ModelId     string `json:"model_id"`
	DisplayName string `json:"display_name"`
}

type ImageAssetSeriesOption struct {
	ModelSeries string `json:"model_series"`
	DisplayName string `json:"display_name"`
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

func imageAssetsBaseQuery(userId int) *gorm.DB {
	return DB.Table("image_generation_tasks AS t").
		Select("t.id, t.id AS task_id, t.user_id, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.request_endpoint, t.params, t.image_url, t.image_metadata, t.cost, t.created_time, t.completed_time").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Where("t.user_id = ? AND t.status = ? AND t.image_url <> ?", userId, ImageTaskStatusSuccess, "")
}

func applyImageAssetFilters(query *gorm.DB, queryParams ImageAssetQueryParams) *gorm.DB {
	if keywordText := strings.TrimSpace(queryParams.Keyword); keywordText != "" {
		keyword := "%" + keywordText + "%"
		query = query.Where("t.prompt LIKE ? OR t.model_id LIKE ? OR m.display_name LIKE ?", keyword, keyword, keyword)
	}
	if modelId := strings.TrimSpace(queryParams.ModelId); modelId != "" {
		query = query.Where("t.model_id = ?", modelId)
	}
	if modelSeries := strings.TrimSpace(queryParams.ModelSeries); modelSeries != "" {
		query = query.Where("m.model_series = ?", modelSeries)
	}
	if queryParams.StartTime > 0 {
		query = query.Where("t.created_time >= ?", queryParams.StartTime)
	}
	if queryParams.EndTime > 0 {
		query = query.Where("t.created_time <= ?", queryParams.EndTime)
	}
	return query
}

// GetImageAssetsByUserID 获取当前用户的图片资产列表（成功任务视图）。
func GetImageAssetsByUserID(userId int, startIdx int, num int, queryParams ImageAssetQueryParams) ([]*ImageGenerationAsset, int64, ImageAssetStats, error) {
	var assets []*ImageGenerationAsset
	var total int64
	stats := ImageAssetStats{}

	if err := applyImageAssetFilters(imageAssetsBaseQuery(userId), queryParams).Count(&total).Error; err != nil {
		return nil, 0, stats, err
	}

	if total > 0 {
		var latestCreatedTime int64
		if err := applyImageAssetFilters(imageAssetsBaseQuery(userId), queryParams).
			Select("MAX(t.created_time)").
			Scan(&latestCreatedTime).Error; err != nil {
			return nil, 0, stats, err
		}
		stats.LatestCreatedTime = latestCreatedTime
	}
	stats.TotalAssets = total

	sortField := "t.created_time"
	switch queryParams.SortBy {
	case "completed_time":
		sortField = "t.completed_time"
	case "cost":
		sortField = "t.cost"
	case "created_time":
		sortField = "t.created_time"
	}
	sortOrder := "DESC"
	if queryParams.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	orderClause := sortField + " " + sortOrder + ", t.id " + sortOrder
	if err := applyImageAssetFilters(imageAssetsBaseQuery(userId), queryParams).
		Order(orderClause).
		Limit(num).
		Offset(startIdx).
		Scan(&assets).Error; err != nil {
		return nil, 0, stats, err
	}

	return assets, total, stats, nil
}

// GetImageAssetFilterOptions 获取当前用户资产仓库可用筛选项。
func GetImageAssetFilterOptions(userId int) (ImageAssetFilterOptions, error) {
	options := ImageAssetFilterOptions{
		Models: []ImageAssetModelOption{},
		Series: []ImageAssetSeriesOption{},
	}

	if err := imageAssetsBaseQuery(userId).
		Select("t.model_id, COALESCE(MAX(m.display_name), '') AS display_name").
		Group("t.model_id").
		Order("t.model_id ASC").
		Scan(&options.Models).Error; err != nil {
		return options, err
	}

	if err := imageAssetsBaseQuery(userId).
		Select("m.model_series, COALESCE(MAX(m.display_name), '') AS display_name").
		Where("m.model_series <> ?", "").
		Group("m.model_series").
		Order("m.model_series ASC").
		Scan(&options.Series).Error; err != nil {
		return options, err
	}

	return options, nil
}

// GetImageAssetByID 根据任务 ID 获取当前用户的图片资产详情。
func GetImageAssetByID(userId int, taskId int) (*ImageGenerationAsset, error) {
	var asset ImageGenerationAsset
	err := imageAssetsBaseQuery(userId).Where("t.id = ?", taskId).Scan(&asset).Error
	if err != nil {
		return nil, err
	}
	if asset.Id == 0 {
		return nil, nil
	}
	return &asset, nil
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
