package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ImageGenerationTask 图片生成任务表
type ImageGenerationTask struct {
	Id              int    `json:"id" gorm:"primaryKey;index:idx_image_tasks_user_id,priority:2;index:idx_image_tasks_user_status_id,priority:3;index:idx_image_tasks_status_id,priority:2"`
	UserId          int    `json:"user_id" gorm:"index;index:idx_image_tasks_user_id,priority:1;index:idx_image_tasks_user_created,priority:1;index:idx_image_tasks_user_status_id,priority:1;index:idx_image_tasks_user_completed,priority:1;not null"`
	ModelId         string `json:"model_id" gorm:"size:128;not null;index"`
	Prompt          string `json:"prompt" gorm:"type:text;not null"`
	RequestEndpoint string `json:"request_endpoint" gorm:"size:32;not null;index"` // openai, openai-response, gemini, openai_mod
	Status          string `json:"status" gorm:"size:20;not null;index;index:idx_image_tasks_user_status_id,priority:2;index:idx_image_tasks_status_id,priority:1;default:'pending'"`
	Params          string `json:"params" gorm:"type:text"`                                                        // JSON: size, quality, style, n, etc.
	ImageUrl        string `json:"image_url" gorm:"type:text"`                                                     // 生成的图片URL
	ThumbnailUrl    string `json:"thumbnail_url" gorm:"type:text"`                                                 // 列表页缩略图 URL
	ImageMetadata   string `json:"image_metadata" gorm:"type:text"`                                                // JSON: revised_prompt, etc.
	ErrorMessage    string `json:"error_message" gorm:"type:text"`                                                 // 错误信息
	Cost            int    `json:"cost" gorm:"default:0"`                                                          // 消耗的配额
	CreatedTime     int64  `json:"created_time" gorm:"bigint;index;index:idx_image_tasks_user_created,priority:2"` // 创建时间戳
	StartedTime     int64  `json:"started_time" gorm:"bigint"`                                                     // 当前轮次开始时间戳（首次创建或最近一次重试）
	CompletedTime   int64  `json:"completed_time" gorm:"bigint;index:idx_image_tasks_user_completed,priority:2"`   // 完成时间戳
	WorkerNode      string `json:"-" gorm:"size:128;index"`                                                        // 当前持有租约的 worker 节点
	LeaseExpiresAt  int64  `json:"-" gorm:"bigint;index"`                                                          // 当前租约过期时间戳
	RequestType     string `json:"request_type" gorm:"-"`
	ReferenceCount  int    `json:"reference_count" gorm:"-"`
	HasMask         bool   `json:"has_mask" gorm:"-"`
}

func (task *ImageGenerationTask) EffectiveStartedTime() int64 {
	if task == nil {
		return 0
	}
	if task.StartedTime > 0 {
		return task.StartedTime
	}
	return task.CreatedTime
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
	if task.StartedTime == 0 {
		task.StartedTime = task.CreatedTime
	}
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
		"started_time":   common.GetTimestamp(),
		"completed_time": 0,
		"worker_node":    "",
		"lease_expires_at": 0,
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

type ImageTaskCursorPage struct {
	Items      []*ImageGenerationTask
	Total      int64
	NextCursor string
	HasMore    bool
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
	Id                          int    `json:"id"`
	TaskId                      int    `json:"task_id"`
	UserId                      int    `json:"user_id"`
	ModelId                     string `json:"model_id"`
	DisplayName                 string `json:"display_name"`
	ModelSeries                 string `json:"model_series"`
	Prompt                      string `json:"prompt"`
	RequestEndpoint             string `json:"request_endpoint"`
	Params                      string `json:"params"`
	ImageUrl                    string `json:"image_url"`
	ThumbnailUrl                string `json:"thumbnail_url"`
	ImageMetadata               string `json:"image_metadata"`
	Cost                        int    `json:"cost"`
	CreatedTime                 int64  `json:"created_time"`
	CompletedTime               int64  `json:"completed_time"`
	InspirationSubmissionId     int    `json:"inspiration_submission_id"`
	InspirationSubmissionStatus string `json:"inspiration_submission_status"`
	InspirationRejectReason     string `json:"inspiration_reject_reason"`
}

type ImageGenerationTaskSummary struct {
	Id            int    `json:"id"`
	ModelId       string `json:"model_id"`
	Prompt        string `json:"prompt"`
	Status        string `json:"status"`
	ImageUrl      string `json:"image_url"`
	ThumbnailUrl  string `json:"thumbnail_url"`
	ErrorMessage  string `json:"error_message"`
	CreatedTime   int64  `json:"created_time"`
	StartedTime   int64  `json:"started_time"`
	CompletedTime int64  `json:"completed_time"`
}

type ImageGenerationTaskDetail struct {
	Id              int    `json:"id"`
	ModelId         string `json:"model_id"`
	DisplayName     string `json:"display_name"`
	Prompt          string `json:"prompt"`
	Status          string `json:"status"`
	RequestEndpoint string `json:"request_endpoint"`
	Params          string `json:"params"`
	ImageUrl        string `json:"image_url"`
	ThumbnailUrl    string `json:"thumbnail_url"`
	ImageMetadata   string `json:"image_metadata"`
	ErrorMessage    string `json:"error_message"`
	Cost            int    `json:"cost"`
	CreatedTime     int64  `json:"created_time"`
	StartedTime     int64  `json:"started_time"`
	CompletedTime   int64  `json:"completed_time"`
	RequestType     string `json:"request_type"`
	ReferenceCount  int    `json:"reference_count"`
	HasMask         bool   `json:"has_mask"`
	OutputWidth     int    `json:"output_width"`
	OutputHeight    int    `json:"output_height"`
	OutputSizeText  string `json:"output_size_text"`
	SizeText        string `json:"size_text"`
	QualityText     string `json:"quality_text"`
	Quantity        int    `json:"quantity"`
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
func GetImageTasksByUserID(userId int, startIdx int, num int, queryParams ImageTaskQueryParams, includeTotal bool) ([]*ImageGenerationTask, int64, bool, error) {
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

	if includeTotal {
		err := query.Count(&total).Error
		if err != nil {
			return nil, 0, false, err
		}
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

	if num <= 0 {
		num = 20
	}
	if num > 100 {
		num = 100
	}

	findErr := query.Order(orderClause).Limit(num + 1).Offset(startIdx).Find(&tasks).Error
	if findErr != nil {
		return nil, 0, false, findErr
	}

	hasMore := len(tasks) > num
	if hasMore {
		tasks = tasks[:num]
	}
	return tasks, total, hasMore, nil
}

func imageTaskSortFieldAndOrder(queryParams ImageTaskQueryParams) (string, string) {
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
	return sortField, sortOrder
}

func ImageTaskCursorPaginationSupported(queryParams ImageTaskQueryParams) bool {
	sortField, _ := imageTaskSortFieldAndOrder(queryParams)
	return sortField == "created_time" || sortField == "completed_time"
}

func encodeImageTaskCursor(sortValue int64, id int) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", sortValue, id)
}

func decodeImageTaskCursor(cursor string) (int64, int, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, 0, nil
	}

	parts := strings.Split(cursor, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cursor")
	}

	sortValue, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	id64, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id64 <= 0 {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	return sortValue, int(id64), nil
}

func applyImageTaskFilters(query *gorm.DB, userId int, queryParams ImageTaskQueryParams) *gorm.DB {
	query = query.Where("user_id = ?", userId)
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
	return query
}

func applyImageTaskCursor(query *gorm.DB, cursor string, sortField string, sortOrder string) (*gorm.DB, error) {
	sortValue, id, err := decodeImageTaskCursor(cursor)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return query, nil
	}

	operator := "<"
	if sortOrder == "ASC" {
		operator = ">"
	}

	condition := fmt.Sprintf("(%s %s ?) OR (%s = ? AND id %s ?)", sortField, operator, sortField, operator)
	return query.Where(condition, sortValue, sortValue, id), nil
}

func GetImageTasksByUserCursor(userId int, cursor string, num int, queryParams ImageTaskQueryParams) (*ImageTaskCursorPage, error) {
	if num <= 0 {
		num = 20
	}
	if num > 100 {
		num = 100
	}

	sortField, sortOrder := imageTaskSortFieldAndOrder(queryParams)
	if !ImageTaskCursorPaginationSupported(queryParams) {
		return nil, fmt.Errorf("cursor pagination is not supported for sort field %s", sortField)
	}

	baseQuery := applyImageTaskFilters(DB.Model(&ImageGenerationTask{}), userId, queryParams)

	var total int64
	if strings.TrimSpace(cursor) == "" {
		if err := baseQuery.Count(&total).Error; err != nil {
			return nil, err
		}
	}

	cursorQuery, err := applyImageTaskCursor(baseQuery, cursor, sortField, sortOrder)
	if err != nil {
		return nil, err
	}

	orderClause := sortField + " " + sortOrder + ", id " + sortOrder
	var tasks []*ImageGenerationTask
	if err := cursorQuery.
		Order(orderClause).
		Limit(num + 1).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	hasMore := len(tasks) > num
	if hasMore {
		tasks = tasks[:num]
	}

	nextCursor := ""
	if hasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		var cursorValue int64
		switch sortField {
		case "completed_time":
			cursorValue = lastTask.CompletedTime
		default:
			cursorValue = lastTask.CreatedTime
		}
		nextCursor = encodeImageTaskCursor(cursorValue, lastTask.Id)
	}

	return &ImageTaskCursorPage{
		Items:      tasks,
		Total:      total,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// GetImageTaskUpdatesByUserID 获取 SSE 所需的轻量任务更新，不执行分页统计。
func GetImageTaskUpdatesByUserID(userId int, completedSince int64, limit int) ([]*ImageGenerationTask, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var tasks []*ImageGenerationTask
	activeStatuses := []string{ImageTaskStatusPending, ImageTaskStatusGenerating}
	query := DB.Model(&ImageGenerationTask{}).
		Select("id, user_id, model_id, prompt, status, image_url, thumbnail_url, error_message, created_time, started_time, completed_time").
		Where("user_id = ? AND status IN ?", userId, activeStatuses).
		Order("id DESC").
		Limit(limit)
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}

	if completedSince <= 0 || len(tasks) >= limit {
		return tasks, nil
	}

	var terminalTasks []*ImageGenerationTask
	remaining := limit - len(tasks)
	err := DB.Model(&ImageGenerationTask{}).
		Select("id, user_id, model_id, prompt, status, image_url, thumbnail_url, error_message, created_time, started_time, completed_time").
		Where("user_id = ? AND status NOT IN ? AND completed_time >= ?", userId, activeStatuses, completedSince).
		Order("id DESC").
		Limit(remaining).
		Find(&terminalTasks).Error
	if err != nil {
		return nil, err
	}

	tasks = append(tasks, terminalTasks...)
	return tasks, nil
}

func BuildImageGenerationTaskSummary(task *ImageGenerationTask) *ImageGenerationTaskSummary {
	if task == nil {
		return nil
	}
	return &ImageGenerationTaskSummary{
		Id:            task.Id,
		ModelId:       task.ModelId,
		Prompt:        task.Prompt,
		Status:        task.Status,
		ImageUrl:      task.ImageUrl,
		ThumbnailUrl:  task.ThumbnailUrl,
		ErrorMessage:  task.ErrorMessage,
		CreatedTime:   task.CreatedTime,
		StartedTime:   task.EffectiveStartedTime(),
		CompletedTime: task.CompletedTime,
	}
}

func BuildImageGenerationTaskDetail(task *ImageGenerationTask, displayName string) *ImageGenerationTaskDetail {
	if task == nil {
		return nil
	}
	return &ImageGenerationTaskDetail{
		Id:              task.Id,
		ModelId:         task.ModelId,
		DisplayName:     displayName,
		Prompt:          task.Prompt,
		Status:          task.Status,
		RequestEndpoint: task.RequestEndpoint,
		Params:          task.Params,
		ImageUrl:        task.ImageUrl,
		ThumbnailUrl:    task.ThumbnailUrl,
		ImageMetadata:   task.ImageMetadata,
		ErrorMessage:    task.ErrorMessage,
		Cost:            task.Cost,
		CreatedTime:     task.CreatedTime,
		StartedTime:     task.EffectiveStartedTime(),
		CompletedTime:   task.CompletedTime,
		RequestType:     task.RequestType,
		ReferenceCount:  task.ReferenceCount,
		HasMask:         task.HasMask,
	}
}

func CountImageTasksByUserAndStatuses(userId int, statuses []string) (int64, error) {
	if userId <= 0 || len(statuses) == 0 {
		return 0, nil
	}

	var total int64
	err := DB.Model(&ImageGenerationTask{}).
		Where("user_id = ? AND status IN ?", userId, statuses).
		Count(&total).Error
	return total, err
}

func imageAssetsBaseQuery(userId int) *gorm.DB {
	return DB.Table("image_generation_tasks AS t").
		Select("t.id, t.id AS task_id, t.user_id, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.request_endpoint, t.params, t.image_url, t.thumbnail_url, t.image_metadata, t.cost, t.created_time, t.completed_time, COALESCE(s.id, 0) AS inspiration_submission_id, COALESCE(s.status, '') AS inspiration_submission_status, COALESCE(s.reject_reason, '') AS inspiration_reject_reason").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Joins("LEFT JOIN image_creative_submissions AS s ON s.task_id = t.id").
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

func ClaimNextPendingImageTask(workerNode string, startedTime int64, leaseExpiresAt int64) (*ImageGenerationTask, error) {
	tasks, err := GetPendingImageTasks(1)
	if err != nil || len(tasks) == 0 || tasks[0] == nil {
		return nil, err
	}

	task := tasks[0]
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ? AND status = ?", task.Id, ImageTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           ImageTaskStatusGenerating,
			"started_time":     startedTime,
			"worker_node":      workerNode,
			"lease_expires_at": leaseExpiresAt,
			"error_message":    "",
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	task.Status = ImageTaskStatusGenerating
	task.ErrorMessage = ""
	task.StartedTime = startedTime
	task.WorkerNode = workerNode
	task.LeaseExpiresAt = leaseExpiresAt
	return task, nil
}

func MarkImageTaskGenerating(id int, workerNode string, startedTime int64, leaseExpiresAt int64) (bool, error) {
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ? AND status = ?", id, ImageTaskStatusPending).
		Updates(map[string]interface{}{
			"status":           ImageTaskStatusGenerating,
			"started_time":     startedTime,
			"worker_node":      workerNode,
			"lease_expires_at": leaseExpiresAt,
			"error_message":    "",
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// UpdateImageTaskStatus 更新任务状态
func UpdateImageTaskStatus(id int, status string, errorMessage string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	if status == ImageTaskStatusGenerating {
		updates["started_time"] = common.GetTimestamp()
	}
	if status == ImageTaskStatusSuccess || status == ImageTaskStatusFailed {
		updates["completed_time"] = common.GetTimestamp()
	}
	return DB.Model(&ImageGenerationTask{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateImageTaskResult 更新任务结果
func UpdateImageTaskResult(id int, imageUrl string, thumbnailUrl string, imageMetadata string, cost int) error {
	updates := map[string]interface{}{
		"status":         ImageTaskStatusSuccess,
		"image_url":      imageUrl,
		"thumbnail_url":  thumbnailUrl,
		"image_metadata": imageMetadata,
		"cost":           cost,
		"completed_time": common.GetTimestamp(),
	}
	return DB.Model(&ImageGenerationTask{}).Where("id = ?", id).Updates(updates).Error
}

func UpdateImageTaskResultClaimed(id int, workerNode string, imageUrl string, thumbnailUrl string, imageMetadata string, cost int) (bool, error) {
	updates := map[string]interface{}{
		"status":           ImageTaskStatusSuccess,
		"image_url":        imageUrl,
		"thumbnail_url":    thumbnailUrl,
		"image_metadata":   imageMetadata,
		"cost":             cost,
		"completed_time":   common.GetTimestamp(),
		"worker_node":      "",
		"lease_expires_at": 0,
	}
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ? AND worker_node = ? AND status = ?", id, workerNode, ImageTaskStatusGenerating).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func UpdateImageTaskTerminalStatusClaimed(id int, workerNode string, status string, errorMessage string) (bool, error) {
	updates := map[string]interface{}{
		"status":           status,
		"completed_time":   common.GetTimestamp(),
		"worker_node":      "",
		"lease_expires_at": 0,
	}
	if strings.TrimSpace(errorMessage) != "" {
		updates["error_message"] = errorMessage
	}
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ? AND worker_node = ? AND status = ?", id, workerNode, ImageTaskStatusGenerating).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func RenewImageTaskLease(id int, workerNode string, leaseExpiresAt int64) (bool, error) {
	if leaseExpiresAt <= 0 {
		return false, nil
	}
	result := DB.Model(&ImageGenerationTask{}).
		Where("id = ? AND worker_node = ? AND status = ?", id, workerNode, ImageTaskStatusGenerating).
		Update("lease_expires_at", leaseExpiresAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func FailExpiredGeneratingImageTasks(expiredBefore int64, errorMessage string, limit int) (int64, error) {
	if expiredBefore <= 0 {
		return 0, nil
	}
	if strings.TrimSpace(errorMessage) == "" {
		errorMessage = "task expired"
	}
	if limit <= 0 {
		limit = 100
	}

	var ids []int
	if err := DB.Model(&ImageGenerationTask{}).
		Where("status = ? AND lease_expires_at > 0 AND lease_expires_at <= ?", ImageTaskStatusGenerating, expiredBefore).
		Order("lease_expires_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	result := DB.Model(&ImageGenerationTask{}).
		Where("id IN ? AND status = ?", ids, ImageTaskStatusGenerating).
		Updates(map[string]interface{}{
			"status":           ImageTaskStatusFailed,
			"error_message":    errorMessage,
			"completed_time":   common.GetTimestamp(),
			"worker_node":      "",
			"lease_expires_at": 0,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func GetExpiredGeneratingImageTaskUserIDs(expiredBefore int64, limit int) ([]int, error) {
	if expiredBefore <= 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	var ids []int
	if err := DB.Model(&ImageGenerationTask{}).
		Distinct("user_id").
		Where("status = ? AND lease_expires_at > 0 AND lease_expires_at <= ?", ImageTaskStatusGenerating, expiredBefore).
		Limit(limit).
		Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
