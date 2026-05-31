package model

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

type VideoTaskQueryParams struct {
	Status         string
	ModelID        string
	StartTimestamp int64
	EndTimestamp   int64
}

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

func GetUserVideoTasks(userId int, startIdx int, num int, queryParams VideoTaskQueryParams, actions []string) ([]*Task, int64, error) {
	if len(actions) == 0 {
		actions = DefaultVideoTaskActions()
	}

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

	var tasks []*Task
	if err := query.Order("id DESC").Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	if queryParams.ModelID != "" {
		filtered := make([]*Task, 0, len(tasks))
		for _, task := range tasks {
			if task == nil {
				continue
			}
			originModel := strings.TrimSpace(task.Properties.OriginModelName)
			upstreamModel := strings.TrimSpace(task.Properties.UpstreamModelName)
			if originModel == queryParams.ModelID || upstreamModel == queryParams.ModelID {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
	}

	total := int64(len(tasks))
	if startIdx >= len(tasks) {
		return []*Task{}, total, nil
	}
	endIdx := startIdx + num
	if endIdx > len(tasks) {
		endIdx = len(tasks)
	}
	return tasks[startIdx:endIdx], total, nil
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
