package model

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

type VideoTaskQueryParams struct {
	Status         string
	ModelID        string
	StartTimestamp int64
	EndTimestamp   int64
}

type VideoTaskPage struct {
	Items      []*Task
	Total      int64
	HasTotal   bool
	NextCursor string
	HasMore    bool
}

const videoTaskSummarySelectColumns = "id, task_id, action, status, progress, fail_reason, submit_time, start_time, finish_time, origin_model_name, upstream_model_name, properties, private_data, data"

var defaultVideoTaskActions = []string{
	constant.TaskActionGenerate,
	constant.TaskActionTextGenerate,
	constant.TaskActionFirstTailGenerate,
	constant.TaskActionReferenceGenerate,
	constant.TaskActionRemix,
}

func DefaultVideoTaskActions() []string {
	return append([]string(nil), defaultVideoTaskActions...)
}

func GetUserVideoTasks(userId int, startIdx int, num int, queryParams VideoTaskQueryParams, actions []string) (*VideoTaskPage, error) {
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = 20
	}
	if num > 100 {
		num = 100
	}

	query := buildUserVideoTaskQuery(userId, queryParams, actions)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	if total == 0 || int64(startIdx) >= total {
		return &VideoTaskPage{
			Items:    []*Task{},
			Total:    total,
			HasTotal: true,
		}, nil
	}

	var tasks []*Task
	pageQuery := query.Session(&gorm.Session{}).
		Select(videoTaskSummarySelectColumns).
		Order("submit_time DESC").
		Order("id DESC")
	if startIdx > 0 {
		pageQuery = pageQuery.Offset(startIdx)
	}
	if err := pageQuery.Limit(num + 1).Find(&tasks).Error; err != nil {
		return nil, err
	}

	hasMore := len(tasks) > num
	if hasMore {
		tasks = tasks[:num]
	}

	nextCursor := ""
	if hasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		nextCursor = encodeVideoTaskCursor(lastTask.SubmitTime, lastTask.ID)
	}

	return &VideoTaskPage{
		Items:      tasks,
		Total:      total,
		HasTotal:   true,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func GetUserVideoTaskByID(userId int, id int64, actions []string) (*Task, error) {
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	var task Task
	err := DB.Where("id = ? AND user_id = ? AND action IN ?", id, userId, actions).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func GetUserVideoTaskByIdentifier(userId int, identifier string, actions []string) (*Task, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("task identifier is required")
	}
	if numericID, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		return GetUserVideoTaskByID(userId, numericID, actions)
	}
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	var task Task
	err := DB.Where("task_id = ? AND user_id = ? AND action IN ?", identifier, userId, actions).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func GetUserVideoTasksByIDs(userId int, ids []int64, actions []string) ([]*Task, error) {
	if userId <= 0 || len(ids) == 0 {
		return []*Task{}, nil
	}
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	var tasks []*Task
	err := forEachChunk(ids, func(chunk []int64) error {
		var partial []*Task
		if err := DB.Where("user_id = ? AND id IN ? AND action IN ?", userId, chunk, actions).
			Find(&partial).Error; err != nil {
			return err
		}
		tasks = append(tasks, partial...)
		return nil
	})
	return tasks, err
}

func DeleteUserVideoTasksByIDs(userId int, ids []int64, actions []string) error {
	return DeleteUserVideoTasksByIDsWithDB(DB, userId, ids, actions)
}

func DeleteUserVideoTasksByIDsWithDB(db *gorm.DB, userId int, ids []int64, actions []string) error {
	if db == nil {
		db = DB
	}
	if userId <= 0 || len(ids) == 0 {
		return nil
	}
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	return forEachChunk(ids, func(chunk []int64) error {
		return db.Where("user_id = ? AND id IN ? AND action IN ?", userId, chunk, actions).
			Delete(&Task{}).Error
	})
}

func encodeVideoTaskCursor(submitTime int64, id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", submitTime, id)
}

func decodeVideoTaskCursor(cursor string) (int64, int64, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, 0, nil
	}

	parts := strings.Split(cursor, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cursor")
	}

	submitTime, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || id <= 0 {
		return 0, 0, fmt.Errorf("invalid cursor")
	}
	return submitTime, id, nil
}

func applyVideoTaskCursor(query *gorm.DB, cursor string) (*gorm.DB, error) {
	submitTime, id, err := decodeVideoTaskCursor(cursor)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return query, nil
	}
	return query.Where("(submit_time < ?) OR (submit_time = ? AND id < ?)", submitTime, submitTime, id), nil
}

func GetUserVideoTasksByCursor(userId int, cursor string, num int, queryParams VideoTaskQueryParams, actions []string) (*VideoTaskPage, error) {
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	if num <= 0 {
		num = 20
	}
	if num > 100 {
		num = 100
	}

	baseQuery := buildUserVideoTaskQuery(userId, queryParams, actions)

	page := &VideoTaskPage{
		Items:    []*Task{},
		HasTotal: strings.TrimSpace(cursor) == "",
	}
	if page.HasTotal {
		if err := baseQuery.Count(&page.Total).Error; err != nil {
			return nil, err
		}
	}

	cursorQuery, err := applyVideoTaskCursor(baseQuery, cursor)
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	if err := cursorQuery.
		Select(videoTaskSummarySelectColumns).
		Order("submit_time DESC").
		Order("id DESC").
		Limit(num + 1).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	page.HasMore = len(tasks) > num
	if page.HasMore {
		tasks = tasks[:num]
	}
	page.Items = tasks
	if page.HasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		page.NextCursor = encodeVideoTaskCursor(lastTask.SubmitTime, lastTask.ID)
	}
	return page, nil
}

func GetUserVideoTaskUpdates(userId int, completedSince int64, limit int, actions []string) ([]*Task, error) {
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	activeStatuses := []TaskStatus{
		TaskStatusNotStart,
		TaskStatusSubmitted,
		TaskStatusQueued,
		TaskStatusInProgress,
	}
	baseQuery := buildUserVideoTaskQuery(userId, VideoTaskQueryParams{}, actions)

	var tasks []*Task
	if err := baseQuery.Session(&gorm.Session{}).
		Select(videoTaskSummarySelectColumns).
		Where("status IN ?", activeStatuses).
		Order("submit_time DESC").
		Order("id DESC").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	if completedSince <= 0 || len(tasks) >= limit {
		return tasks, nil
	}

	var terminalTasks []*Task
	if err := baseQuery.Session(&gorm.Session{}).
		Select(videoTaskSummarySelectColumns).
		Where("status IN ?", []TaskStatus{TaskStatusSuccess, TaskStatusFailure}).
		Where("finish_time >= ?", completedSince).
		Order("finish_time DESC").
		Order("id DESC").
		Limit(limit - len(tasks)).
		Find(&terminalTasks).Error; err != nil {
		return nil, err
	}

	return append(tasks, terminalTasks...), nil
}

func buildUserVideoTaskQuery(userId int, queryParams VideoTaskQueryParams, actions []string) *gorm.DB {
	query := DB.Model(&Task{}).Where("user_id = ?", userId).Where("action IN ?", actions)
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.StartTimestamp > 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp > 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	if strings.TrimSpace(queryParams.ModelID) == "" {
		return query
	}
	return query.Where(
		"origin_model_name = ? OR upstream_model_name = ?",
		queryParams.ModelID,
		queryParams.ModelID,
	)
}

func ExtractTaskThumbnailURL(task *Task) string {
	if task == nil || len(task.Data) == 0 {
		return ""
	}

	var openAIVideo map[string]any
	if err := common.Unmarshal(task.Data, &openAIVideo); err != nil {
		return ""
	}
	if metadata, ok := openAIVideo["metadata"].(map[string]any); ok {
		if poster, ok := metadata["poster_url"].(string); ok {
			return strings.TrimSpace(poster)
		}
		if thumb, ok := metadata["thumbnail_url"].(string); ok {
			return strings.TrimSpace(thumb)
		}
	}
	return ""
}

func ExtractTaskDuration(task *Task) int {
	if task == nil || len(task.Data) == 0 {
		return 0
	}

	var payload map[string]any
	if err := common.Unmarshal(task.Data, &payload); err != nil {
		return 0
	}
	switch value := payload["seconds"].(type) {
	case string:
		var duration int
		_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &duration)
		return duration
	case float64:
		return int(value)
	}
	return 0
}

func ExtractTaskResolution(task *Task) string {
	if task == nil || len(task.Data) == 0 {
		return ""
	}

	var payload map[string]any
	if err := common.Unmarshal(task.Data, &payload); err != nil {
		return ""
	}
	if size, ok := payload["size"].(string); ok {
		return strings.TrimSpace(size)
	}
	return ""
}

func ExtractTaskAspectRatio(task *Task) string {
	resolution := ExtractTaskResolution(task)
	if resolution == "" {
		return ""
	}
	if strings.Contains(resolution, ":") {
		return resolution
	}
	return ""
}

func ExtractVideoResultURLFromPayload(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		return ""
	}

	for _, path := range [][]string{
		{"content", "video_url"},
		{"data", "content", "video_url"},
		{"video_url"},
		{"data", "video_url"},
	} {
		if value := extractNestedVideoString(payload, path); value != "" {
			return value
		}
	}
	return ""
}

func IsTaskVideoProxyURL(task *Task, value string) bool {
	if task == nil {
		return false
	}

	proxyPath := BuildVideoProxyURL(task)
	value = strings.TrimSpace(value)
	if proxyPath == "" || value == "" {
		return false
	}
	if value == proxyPath {
		return true
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return parsed.Path == proxyPath
}

// EffectiveVideoResultURL prefers a stored direct URL, then recovers one from task.Data,
// and only falls back to the proxy URL when no direct result is available.
func EffectiveVideoResultURL(task *Task) string {
	if task == nil {
		return ""
	}

	storedResultURL := strings.TrimSpace(task.PrivateData.ResultURL)
	if storedResultURL != "" && !IsTaskVideoProxyURL(task, storedResultURL) {
		return storedResultURL
	}
	if directResultURL := ExtractVideoResultURLFromPayload(task.Data); directResultURL != "" {
		return directResultURL
	}
	if storedResultURL != "" {
		return storedResultURL
	}
	return BuildVideoProxyURL(task)
}

func BuildVideoProxyURL(task *Task) string {
	if task == nil || strings.TrimSpace(task.TaskID) == "" {
		return ""
	}
	return "/v1/videos/" + strings.TrimSpace(task.TaskID) + "/content"
}

func GuessTaskVideoPreviewType(task *Task) string {
	if task == nil {
		return ""
	}
	switch task.Action {
	case constant.TaskActionGenerate:
		return "image_to_video"
	case constant.TaskActionTextGenerate:
		return "text_to_video"
	case constant.TaskActionFirstTailGenerate:
		return "first_tail_video"
	case constant.TaskActionReferenceGenerate:
		return "reference_video"
	case constant.TaskActionRemix:
		return "remix_video"
	default:
		return ""
	}
}

func IsProbablyDataURL(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "data:")
}

func IsProbablyHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func DecodeBase64DataURL(dataURL string) ([]byte, error) {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data url")
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(parts[1])
}

func extractNestedVideoString(value any, path []string) string {
	if len(path) == 0 {
		text, _ := value.(string)
		return strings.TrimSpace(text)
	}

	obj, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	next, ok := obj[path[0]]
	if !ok {
		return ""
	}
	return extractNestedVideoString(next, path[1:])
}
