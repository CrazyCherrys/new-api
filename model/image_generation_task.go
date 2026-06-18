package model

import (
	"fmt"
	"sort"
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
	SelectedGroup   string `json:"selected_group" gorm:"size:64;default:'';index"`
	Prompt          string `json:"prompt" gorm:"type:text;not null"`
	RequestEndpoint string `json:"request_endpoint" gorm:"size:32;not null;index"` // openai, openai-response, gemini
	Status          string `json:"status" gorm:"size:20;not null;index;index:idx_image_tasks_user_status_id,priority:2;index:idx_image_tasks_status_id,priority:1;default:'pending'"`
	Params          string `json:"params" gorm:"type:text"`                                                        // JSON: size, quality, style, n, etc.
	ImageUrl        string `json:"image_url" gorm:"type:text"`                                                     // 生成的图片URL
	ThumbnailUrl    string `json:"thumbnail_url" gorm:"type:text"`                                                 // 列表页缩略图 URL
	ResultAssetStatus string `json:"result_asset_status" gorm:"size:32;not null;default:'available'"`            // 结果图资产状态
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

type imageTaskStore struct {
	db *gorm.DB
}

func imageTaskStoreForCanvas() (*imageTaskStore, error) {
	db, err := canvasModeDataDB(CanvasModeImage)
	if err != nil {
		return nil, err
	}
	return &imageTaskStore{db: db}, nil
}

func (s *imageTaskStore) model() *gorm.DB {
	return s.db.Model(&ImageGenerationTask{})
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

const (
	ImageTaskResultAssetStatusAvailable      = "available"
	ImageTaskResultAssetStatusExpiredCleaned = "expired_cleaned"
)

// Insert 插入新任务
func (task *ImageGenerationTask) Insert() error {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return err
	}
	task.CreatedTime = common.GetTimestamp()
	if task.StartedTime == 0 {
		task.StartedTime = task.CreatedTime
	}
	if task.Status == "" {
		task.Status = ImageTaskStatusPending
	}
	if strings.TrimSpace(task.ResultAssetStatus) == "" {
		task.ResultAssetStatus = ImageTaskResultAssetStatusAvailable
	}
	return store.db.Create(task).Error
}

// Update 更新任务
func (task *ImageGenerationTask) Update() error {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return err
	}
	return store.db.Model(task).Updates(task).Error
}

// ResetImageTaskForRetry 将失败任务重置为待处理状态，返回是否实际更新。
func ResetImageTaskForRetry(id int) (bool, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return false, err
	}
	updates := map[string]interface{}{
		"status":           ImageTaskStatusPending,
		"error_message":    "",
		"started_time":     common.GetTimestamp(),
		"completed_time":   0,
		"worker_node":      "",
		"lease_expires_at": 0,
	}
	result := store.model().
		Where("id = ? AND status = ?", id, ImageTaskStatusFailed).
		Updates(updates)
	return result.RowsAffected > 0, result.Error
}

// GetImageTaskByID 根据ID获取任务
func GetImageTaskByID(id int) (*ImageGenerationTask, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var task ImageGenerationTask
	err = store.db.First(&task, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &task, err
}

func GetImageTasksByUserAndIDs(userId int, ids []int) ([]*ImageGenerationTask, error) {
	if userId <= 0 || len(ids) == 0 {
		return []*ImageGenerationTask{}, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var tasks []*ImageGenerationTask
	err = forEachChunk(ids, func(chunk []int) error {
		var partial []*ImageGenerationTask
		if err := store.db.Where("user_id = ? AND id IN ?", userId, chunk).
			Find(&partial).Error; err != nil {
			return err
		}
		tasks = append(tasks, partial...)
		return nil
	})
	return tasks, err
}

func DeleteImageTasksByUserAndIDs(userId int, ids []int) error {
	if userId <= 0 || len(ids) == 0 {
		return nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return err
	}
	return store.db.Transaction(func(tx *gorm.DB) error {
		return DeleteImageTasksByUserAndIDsWithDB(tx, userId, ids)
	})
}

func DeleteImageTasksByUserAndIDsWithDB(db *gorm.DB, userId int, ids []int) error {
	if userId <= 0 || len(ids) == 0 {
		return nil
	}
	if db == nil {
		store, err := imageTaskStoreForCanvas()
		if err != nil {
			return err
		}
		db = store.db
	}
	return forEachChunk(ids, func(chunk []int) error {
		return db.Where("user_id = ? AND id IN ?", userId, chunk).
			Delete(&ImageGenerationTask{}).Error
	})
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
	SelectedGroup string `json:"selected_group"`
	Prompt        string `json:"prompt"`
	Status        string `json:"status"`
	ImageUrl      string `json:"image_url"`
	ThumbnailUrl  string `json:"thumbnail_url"`
	ResultAssetStatus string `json:"result_asset_status"`
	ErrorMessage  string `json:"error_message"`
	CreatedTime   int64  `json:"created_time"`
	StartedTime   int64  `json:"started_time"`
	CompletedTime int64  `json:"completed_time"`
}

type ImageGenerationTaskDetail struct {
	Id              int    `json:"id"`
	ModelId         string `json:"model_id"`
	DisplayName     string `json:"display_name"`
	SelectedGroup   string `json:"selected_group"`
	Prompt          string `json:"prompt"`
	Status          string `json:"status"`
	RequestEndpoint string `json:"request_endpoint"`
	Params          string `json:"params"`
	ImageUrl        string `json:"image_url"`
	ThumbnailUrl    string `json:"thumbnail_url"`
	ResultAssetStatus string `json:"result_asset_status"`
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

type imageTaskModelDisplay struct {
	DisplayName string
	ModelSeries string
}

func listImageTasksByIDs(ids []int) ([]*ImageGenerationTask, error) {
	if len(ids) == 0 {
		return []*ImageGenerationTask{}, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	tasks := make([]*ImageGenerationTask, 0, len(ids))
	err = forEachChunk(ids, func(chunk []int) error {
		var partial []*ImageGenerationTask
		if err := store.db.Where("id IN ?", chunk).Find(&partial).Error; err != nil {
			return err
		}
		tasks = append(tasks, partial...)
		return nil
	})
	return tasks, err
}

func imageCreativeSubmissionsByTaskIDs(taskIDs []int) (map[int]*ImageCreativeSubmission, error) {
	result := make(map[int]*ImageCreativeSubmission)
	if len(taskIDs) == 0 {
		return result, nil
	}
	err := forEachChunk(taskIDs, func(chunk []int) error {
		var partial []*ImageCreativeSubmission
		if err := DB.Where("task_id IN ?", chunk).Find(&partial).Error; err != nil {
			return err
		}
		for _, item := range partial {
			if item == nil {
				continue
			}
			result[item.TaskId] = item
		}
		return nil
	})
	return result, err
}

func imageTaskModelDisplayByModelIDs(modelIDs []string) (map[string]imageTaskModelDisplay, error) {
	result := make(map[string]imageTaskModelDisplay)
	if len(modelIDs) == 0 {
		return result, nil
	}
	mappings, err := GetModelMappingsByRequestModels(modelIDs)
	if err != nil {
		return nil, err
	}
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		modelID := strings.TrimSpace(mapping.RequestModel)
		if modelID == "" {
			continue
		}
		result[modelID] = imageTaskModelDisplay{
			DisplayName: strings.TrimSpace(mapping.DisplayName),
			ModelSeries: strings.TrimSpace(mapping.ModelSeries),
		}
	}
	return result, nil
}

func imageTaskIDsByModelSeries(modelSeries string) ([]string, error) {
	modelSeries = strings.TrimSpace(modelSeries)
	if modelSeries == "" {
		return []string{}, nil
	}
	var modelIDs []string
	err := DB.Model(&ModelMapping{}).
		Where("model_series = ?", modelSeries).
		Pluck("request_model", &modelIDs).Error
	return modelIDs, err
}

func imageTaskIDsByDisplayNameKeyword(keyword string) ([]string, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []string{}, nil
	}
	var modelIDs []string
	err := DB.Model(&ModelMapping{}).
		Where("display_name LIKE ?", "%"+keyword+"%").
		Pluck("request_model", &modelIDs).Error
	return modelIDs, err
}

func buildImageGenerationAssetRecord(task *ImageGenerationTask, display imageTaskModelDisplay, submission *ImageCreativeSubmission) *ImageGenerationAsset {
	if task == nil {
		return nil
	}
	record := &ImageGenerationAsset{
		Id:              task.Id,
		TaskId:          task.Id,
		UserId:          task.UserId,
		ModelId:         task.ModelId,
		DisplayName:     display.DisplayName,
		ModelSeries:     display.ModelSeries,
		Prompt:          task.Prompt,
		RequestEndpoint: task.RequestEndpoint,
		Params:          task.Params,
		ImageUrl:        task.ImageUrl,
		ThumbnailUrl:    task.ThumbnailUrl,
		ImageMetadata:   task.ImageMetadata,
		Cost:            task.Cost,
		CreatedTime:     task.CreatedTime,
		CompletedTime:   task.CompletedTime,
	}
	if submission != nil {
		record.InspirationSubmissionId = submission.Id
		record.InspirationSubmissionStatus = submission.Status
		record.InspirationRejectReason = submission.RejectReason
	}
	return record
}

func CountUserImageTasksByResultAssetURL(userId int, assetURL string) (int64, error) {
	if userId <= 0 || strings.TrimSpace(assetURL) == "" {
		return 0, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return 0, err
	}
	var count int64
	err = store.model().
		Where("user_id = ? AND (image_url = ? OR thumbnail_url = ?)", userId, assetURL, assetURL).
		Count(&count).Error
	return count, err
}

func ListUserImageTaskParamsContainingAssetURL(userId int, assetURL string) ([]string, error) {
	if userId <= 0 || strings.TrimSpace(assetURL) == "" {
		return []string{}, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var tasks []*ImageGenerationTask
	if err := store.model().
		Select("params").
		Where("user_id = ?", userId).
		Where("params LIKE ?", "%"+assetURL+"%").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	params := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		params = append(params, task.Params)
	}
	return params, nil
}

func ListSuccessfulImageTaskIDsByResultAssetURL(assetURL string) ([]int, error) {
	assetURL = strings.TrimSpace(assetURL)
	if assetURL == "" {
		return []int{}, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var taskIDs []int
	err = store.model().
		Where("status = ? AND (image_url = ? OR thumbnail_url = ?)", ImageTaskStatusSuccess, assetURL, assetURL).
		Pluck("id", &taskIDs).Error
	return taskIDs, err
}

func ListExpiredImageTasksBefore(expirationTime int64) ([]*ImageGenerationTask, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var tasks []*ImageGenerationTask
	err = store.db.Where("status = ? AND created_time < ?", ImageTaskStatusSuccess, expirationTime).Find(&tasks).Error
	return tasks, err
}

func CountImageTasksGroupedByUserAndStatuses(statuses []string) (map[int]int64, error) {
	result := make(map[int]int64)
	if len(statuses) == 0 {
		return result, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	type userTaskCount struct {
		UserId int   `gorm:"column:user_id"`
		Count  int64 `gorm:"column:count"`
	}
	rows := make([]userTaskCount, 0)
	if err := store.model().
		Select("user_id, COUNT(*) AS count").
		Where("status IN ?", statuses).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserId] = row.Count
	}
	return result, nil
}

// GetImageTasksByUserID 根据用户ID获取任务列表（分页+筛选+排序）
func GetImageTasksByUserID(userId int, startIdx int, num int, queryParams ImageTaskQueryParams, includeTotal bool) ([]*ImageGenerationTask, int64, bool, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, 0, false, err
	}
	var tasks []*ImageGenerationTask
	var total int64

	query := store.model().Where("user_id = ?", userId)

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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
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

	baseQuery := applyImageTaskFilters(store.model(), userId, queryParams)

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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var tasks []*ImageGenerationTask
	activeStatuses := []string{ImageTaskStatusPending, ImageTaskStatusGenerating}
	query := store.model().
		Select("id, user_id, model_id, prompt, status, image_url, thumbnail_url, result_asset_status, error_message, created_time, started_time, completed_time").
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
	err = store.model().
		Select("id, user_id, model_id, prompt, status, image_url, thumbnail_url, result_asset_status, error_message, created_time, started_time, completed_time").
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
		SelectedGroup: task.SelectedGroup,
		Prompt:        task.Prompt,
		Status:        task.Status,
		ImageUrl:      task.ImageUrl,
		ThumbnailUrl:  task.ThumbnailUrl,
		ResultAssetStatus: NormalizeImageTaskResultAssetStatus(task.ResultAssetStatus),
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
		SelectedGroup:   task.SelectedGroup,
		Prompt:          task.Prompt,
		Status:          task.Status,
		RequestEndpoint: task.RequestEndpoint,
		Params:          task.Params,
		ImageUrl:        task.ImageUrl,
		ThumbnailUrl:    task.ThumbnailUrl,
		ResultAssetStatus: NormalizeImageTaskResultAssetStatus(task.ResultAssetStatus),
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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return 0, err
	}

	var total int64
	err = store.model().
		Where("user_id = ? AND status IN ?", userId, statuses).
		Count(&total).Error
	return total, err
}

func applyImageAssetFilters(query *gorm.DB, queryParams ImageAssetQueryParams) (*gorm.DB, error) {
	if keywordText := strings.TrimSpace(queryParams.Keyword); keywordText != "" {
		keyword := "%" + keywordText + "%"
		displayNameModelIDs, err := imageTaskIDsByDisplayNameKeyword(keywordText)
		if err != nil {
			return nil, err
		}
		if len(displayNameModelIDs) > 0 {
			query = query.Where("(prompt LIKE ? OR model_id LIKE ? OR model_id IN ?)", keyword, keyword, displayNameModelIDs)
		} else {
			query = query.Where("(prompt LIKE ? OR model_id LIKE ?)", keyword, keyword)
		}
	}
	if modelId := strings.TrimSpace(queryParams.ModelId); modelId != "" {
		query = query.Where("model_id = ?", modelId)
	}
	if modelSeries := strings.TrimSpace(queryParams.ModelSeries); modelSeries != "" {
		seriesModelIDs, err := imageTaskIDsByModelSeries(modelSeries)
		if err != nil {
			return nil, err
		}
		if len(seriesModelIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("model_id IN ?", seriesModelIDs)
		}
	}
	if queryParams.StartTime > 0 {
		query = query.Where("created_time >= ?", queryParams.StartTime)
	}
	if queryParams.EndTime > 0 {
		query = query.Where("created_time <= ?", queryParams.EndTime)
	}
	return query, nil
}

// GetImageAssetsByUserID 获取当前用户的图片资产列表（成功任务视图）。
func GetImageAssetsByUserID(userId int, startIdx int, num int, queryParams ImageAssetQueryParams) ([]*ImageGenerationAsset, int64, ImageAssetStats, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, 0, ImageAssetStats{}, err
	}
	var assets []*ImageGenerationAsset
	var total int64
	stats := ImageAssetStats{}
	baseQuery, err := applyImageAssetFilters(
		store.model().Where("user_id = ? AND status = ? AND image_url <> ?", userId, ImageTaskStatusSuccess, ""),
		queryParams,
	)
	if err != nil {
		return nil, 0, stats, err
	}
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, stats, err
	}

	if total > 0 {
		var latestCreatedTime int64
		if err := baseQuery.Session(&gorm.Session{}).
			Select("MAX(created_time)").
			Scan(&latestCreatedTime).Error; err != nil {
			return nil, 0, stats, err
		}
		stats.LatestCreatedTime = latestCreatedTime
	}
	stats.TotalAssets = total

	sortField := "t.created_time"
	switch queryParams.SortBy {
	case "completed_time":
		sortField = "completed_time"
	case "cost":
		sortField = "cost"
	case "created_time":
		sortField = "created_time"
	}
	sortOrder := "DESC"
	if queryParams.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	orderClause := sortField + " " + sortOrder + ", id " + sortOrder
	var tasks []*ImageGenerationTask
	if err := baseQuery.Session(&gorm.Session{}).
		Order(orderClause).
		Limit(num).
		Offset(startIdx).
		Find(&tasks).Error; err != nil {
		return nil, 0, stats, err
	}
	taskIDs := make([]int, 0, len(tasks))
	modelIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		taskIDs = append(taskIDs, task.Id)
		modelIDs = append(modelIDs, task.ModelId)
	}
	submissionsByTaskID, err := imageCreativeSubmissionsByTaskIDs(taskIDs)
	if err != nil {
		return nil, 0, stats, err
	}
	displayByModelID, err := imageTaskModelDisplayByModelIDs(modelIDs)
	if err != nil {
		return nil, 0, stats, err
	}
	assets = make([]*ImageGenerationAsset, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		assets = append(assets, buildImageGenerationAssetRecord(
			task,
			displayByModelID[strings.TrimSpace(task.ModelId)],
			submissionsByTaskID[task.Id],
		))
	}

	return assets, total, stats, nil
}

// GetImageAssetFilterOptions 获取当前用户资产仓库可用筛选项。
func GetImageAssetFilterOptions(userId int) (ImageAssetFilterOptions, error) {
	options := ImageAssetFilterOptions{
		Models: []ImageAssetModelOption{},
		Series: []ImageAssetSeriesOption{},
	}

	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return options, err
	}
	var modelIDs []string
	if err := store.model().
		Where("user_id = ? AND status = ? AND image_url <> ?", userId, ImageTaskStatusSuccess, "").
		Distinct("model_id").
		Order("model_id ASC").
		Pluck("model_id", &modelIDs).Error; err != nil {
		return options, err
	}
	displayByModelID, err := imageTaskModelDisplayByModelIDs(modelIDs)
	if err != nil {
		return options, err
	}
	seriesSeen := make(map[string]struct{})
	for _, modelID := range modelIDs {
		display := displayByModelID[strings.TrimSpace(modelID)]
		options.Models = append(options.Models, ImageAssetModelOption{
			ModelId:     modelID,
			DisplayName: display.DisplayName,
		})
		if display.ModelSeries == "" {
			continue
		}
		if _, ok := seriesSeen[display.ModelSeries]; ok {
			continue
		}
		seriesSeen[display.ModelSeries] = struct{}{}
		options.Series = append(options.Series, ImageAssetSeriesOption{
			ModelSeries: display.ModelSeries,
			DisplayName: display.DisplayName,
		})
	}
	sort.Slice(options.Series, func(i, j int) bool {
		return options.Series[i].ModelSeries < options.Series[j].ModelSeries
	})
	if len(options.Models) == 0 {
		return options, err
	}
	return options, nil
}

// GetImageAssetByID 根据任务 ID 获取当前用户的图片资产详情。
func GetImageAssetByID(userId int, taskId int) (*ImageGenerationAsset, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var task ImageGenerationTask
	err = store.model().
		Where("user_id = ? AND id = ? AND status = ? AND image_url <> ?", userId, taskId, ImageTaskStatusSuccess, "").
		First(&task).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	submissionsByTaskID, err := imageCreativeSubmissionsByTaskIDs([]int{task.Id})
	if err != nil {
		return nil, err
	}
	displayByModelID, err := imageTaskModelDisplayByModelIDs([]string{task.ModelId})
	if err != nil {
		return nil, err
	}
	return buildImageGenerationAssetRecord(
		&task,
		displayByModelID[strings.TrimSpace(task.ModelId)],
		submissionsByTaskID[task.Id],
	), nil
}

// DeleteImageTask 删除任务
func DeleteImageTask(id int) error {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return err
	}
	return store.db.Delete(&ImageGenerationTask{}, id).Error
}

// GetPendingImageTasks 获取所有待处理的任务
func GetPendingImageTasks(limit int) ([]*ImageGenerationTask, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	var tasks []*ImageGenerationTask
	err = store.db.Where("status = ?", ImageTaskStatusPending).
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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}
	result := store.model().
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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return false, err
	}
	result := store.model().
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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return err
	}
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
	return store.model().Where("id = ?", id).Updates(updates).Error
}

// UpdateImageTaskResult 更新任务结果
func UpdateImageTaskResult(id int, imageUrl string, thumbnailUrl string, imageMetadata string, cost int) error {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return err
	}
	updates := map[string]interface{}{
		"status":         ImageTaskStatusSuccess,
		"image_url":      imageUrl,
		"thumbnail_url":  thumbnailUrl,
		"result_asset_status": ImageTaskResultAssetStatusAvailable,
		"image_metadata": imageMetadata,
		"cost":           cost,
		"completed_time": common.GetTimestamp(),
	}
	return store.model().Where("id = ?", id).Updates(updates).Error
}

func UpdateImageTaskResultClaimed(id int, workerNode string, imageUrl string, thumbnailUrl string, imageMetadata string, cost int) (bool, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return false, err
	}
	updates := map[string]interface{}{
		"status":           ImageTaskStatusSuccess,
		"image_url":        imageUrl,
		"thumbnail_url":    thumbnailUrl,
		"result_asset_status": ImageTaskResultAssetStatusAvailable,
		"image_metadata":   imageMetadata,
		"cost":             cost,
		"completed_time":   common.GetTimestamp(),
		"worker_node":      "",
		"lease_expires_at": 0,
	}
	result := store.model().
		Where("id = ? AND worker_node = ? AND status = ?", id, workerNode, ImageTaskStatusGenerating).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func UpdateImageTaskTerminalStatusClaimed(id int, workerNode string, status string, errorMessage string) (bool, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return false, err
	}
	updates := map[string]interface{}{
		"status":           status,
		"completed_time":   common.GetTimestamp(),
		"worker_node":      "",
		"lease_expires_at": 0,
	}
	if strings.TrimSpace(errorMessage) != "" {
		updates["error_message"] = errorMessage
	}
	result := store.model().
		Where("id = ? AND worker_node = ? AND status = ?", id, workerNode, ImageTaskStatusGenerating).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ExpireImageTaskResultAssets(id int) (bool, error) {
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return false, err
	}
	result := store.model().
		Where("id = ? AND status = ? AND result_asset_status <> ?", id, ImageTaskStatusSuccess, ImageTaskResultAssetStatusExpiredCleaned).
		Updates(map[string]interface{}{
			"image_url":           "",
			"thumbnail_url":       "",
			"result_asset_status": ImageTaskResultAssetStatusExpiredCleaned,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func RenewImageTaskLease(id int, workerNode string, leaseExpiresAt int64) (bool, error) {
	if leaseExpiresAt <= 0 {
		return false, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return false, err
	}
	result := store.model().
		Where("id = ? AND worker_node = ? AND status = ?", id, workerNode, ImageTaskStatusGenerating).
		Update("lease_expires_at", leaseExpiresAt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func NormalizeImageTaskResultAssetStatus(status string) string {
	switch strings.TrimSpace(status) {
	case ImageTaskResultAssetStatusExpiredCleaned:
		return ImageTaskResultAssetStatusExpiredCleaned
	default:
		return ImageTaskResultAssetStatusAvailable
	}
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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return 0, err
	}

	var ids []int
	if err := store.model().
		Where("status = ? AND lease_expires_at > 0 AND lease_expires_at <= ?", ImageTaskStatusGenerating, expiredBefore).
		Order("lease_expires_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	result := store.model().
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
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return nil, err
	}

	var ids []int
	if err := store.model().
		Distinct("user_id").
		Where("status = ? AND lease_expires_at > 0 AND lease_expires_at <= ?", ImageTaskStatusGenerating, expiredBefore).
		Limit(limit).
		Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
