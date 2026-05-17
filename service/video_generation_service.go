package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type CanvasVideoTaskCreateRequest struct {
	ModelID         string   `json:"model_id"`
	Prompt          string   `json:"prompt"`
	RequestEndpoint string   `json:"request_endpoint"`
	ReferenceImages []string `json:"reference_images"`
	Duration        int      `json:"duration"`
	Size            string   `json:"size"`
}

type CanvasVideoTaskItem struct {
	ID            int64          `json:"id"`
	TaskID        string         `json:"task_id"`
	ModelID       string         `json:"model_id"`
	DisplayName   string         `json:"display_name"`
	ModelSeries   string         `json:"model_series"`
	RequestType   string         `json:"request_type"`
	Status        string         `json:"status"`
	CreatedTime   int64          `json:"created_time"`
	CompletedTime int64          `json:"completed_time"`
	ImageURL      string         `json:"image_url,omitempty"`
	ThumbnailURL  string         `json:"thumbnail_url,omitempty"`
	ResultURL     string         `json:"result_url,omitempty"`
	FailReason    string         `json:"fail_reason,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	SubmitTime    int64          `json:"submit_time"`
	FinishTime    int64          `json:"finish_time"`
	Progress      int            `json:"progress"`
	Prompt        string         `json:"prompt"`
	RequestDetail map[string]any `json:"request_detail,omitempty"`
}

func normalizeCanvasVideoTaskProgress(progress string) int {
	trimmed := strings.TrimSpace(strings.TrimSuffix(progress, "%"))
	if trimmed == "" {
		return 0
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeCanvasVideoTaskStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusQueued, model.TaskStatusSubmitted:
		return "pending"
	case model.TaskStatusInProgress:
		return "generating"
	case model.TaskStatusSuccess:
		return "success"
	case model.TaskStatusFailure:
		return "failed"
	default:
		return strings.ToLower(string(status))
	}
}

func NormalizeVideoEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "openai-video-generations", "video-generation", "video-generations":
		return string(constant.EndpointTypeOpenAIVideoGeneration)
	case "openai-videos", "sora", "video", "openai-video":
		return string(constant.EndpointTypeOpenAIVideo)
	default:
		return strings.ToLower(strings.TrimSpace(endpoint))
	}
}

func ChannelTypesForVideoEndpoint(endpoint string) ([]int, error) {
	switch NormalizeVideoEndpoint(endpoint) {
	case string(constant.EndpointTypeOpenAIVideo):
		return []int{constant.ChannelTypeOpenAI, constant.ChannelTypeSora}, nil
	case string(constant.EndpointTypeOpenAIVideoGeneration):
		return []int{
			constant.ChannelTypeKling,
			constant.ChannelTypeJimeng,
			constant.ChannelTypeVidu,
			constant.ChannelTypeDoubaoVideo,
			constant.ChannelTypeVolcEngine,
			constant.ChannelTypeGemini,
			constant.ChannelTypeVertexAi,
			constant.ChannelTypeMiniMax,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported request_endpoint: %s", endpoint)
	}
}

func SelectVideoChannelForModel(modelName string, userID int, channelTypes []int) (int, error) {
	return selectChannelForModel(modelName, userID, channelTypes)
}

func GetActiveVideoModelMappings(startIdx int, num int) ([]*model.ModelMapping, int64, error) {
	var mappings []*model.ModelMapping
	query := model.DB.Model(&model.ModelMapping{}).
		Where("model_type = ? AND status = ? AND request_endpoint <> ''", 3, 1)
	err := query.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, int64(len(mappings)), err
}

func BuildCanvasVideoTaskItem(task *model.Task) *CanvasVideoTaskItem {
	if task == nil {
		return nil
	}
	item := &CanvasVideoTaskItem{
		ID:            task.ID,
		TaskID:        task.TaskID,
		Status:        normalizeCanvasVideoTaskStatus(task.Status),
		CreatedTime:   task.SubmitTime,
		CompletedTime: task.FinishTime,
		ImageURL:      "",
		ThumbnailURL:  "",
		ResultURL:     task.GetResultURL(),
		FailReason:    task.FailReason,
		ErrorMessage:  task.FailReason,
		SubmitTime:    task.SubmitTime,
		FinishTime:    task.FinishTime,
		Progress:      normalizeCanvasVideoTaskProgress(task.Progress),
		ModelID:       task.Properties.OriginModelName,
		DisplayName:   task.Properties.OriginModelName,
	}
	if mapping, err := model.GetModelMappingByRequestModel(task.Properties.OriginModelName); err == nil && mapping != nil {
		item.ModelID = mapping.RequestModel
		item.DisplayName = mapping.DisplayName
		item.ModelSeries = mapping.ModelSeries
	}

	var data map[string]any
	if len(task.Data) > 0 {
		_ = common.Unmarshal(task.Data, &data)
	}
	if task.Properties.Input != "" {
		item.Prompt = task.Properties.Input
	}
	if prompt, ok := data["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		item.Prompt = prompt
	}
	if coverURL, ok := data["cover_url"].(string); ok && strings.TrimSpace(coverURL) != "" {
		item.ThumbnailURL = strings.TrimSpace(coverURL)
	} else if coverURL, ok := data["coverUrl"].(string); ok && strings.TrimSpace(coverURL) != "" {
		item.ThumbnailURL = strings.TrimSpace(coverURL)
	} else if posterURL, ok := data["poster_url"].(string); ok && strings.TrimSpace(posterURL) != "" {
		item.ThumbnailURL = strings.TrimSpace(posterURL)
	}
	switch task.Action {
	case constant.TaskActionGenerate, constant.TaskActionReferenceGenerate, constant.TaskActionFirstTailGenerate:
		item.RequestType = "image_to_video"
	default:
		item.RequestType = "text_to_video"
	}
	item.RequestDetail = data
	return item
}

func ListCanvasVideoTasks(userID int, startIdx int, num int, status string, modelID string, startTime int64, endTime int64) ([]*CanvasVideoTaskItem, int64, error) {
	queryParams := model.SyncTaskQueryParams{
		ActionIn:       model.VideoTaskActions(),
		Status:         strings.TrimSpace(status),
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
	}
	if modelID == "" {
		tasks := model.TaskGetAllUserTask(userID, startIdx, num, queryParams)
		total := model.TaskCountAllUserTask(userID, queryParams)
		items := make([]*CanvasVideoTaskItem, 0, len(tasks))
		for _, task := range tasks {
			item := BuildCanvasVideoTaskItem(task)
			if item == nil {
				continue
			}
			items = append(items, item)
		}
		return items, total, nil
	}

	totalCandidates := int(model.TaskCountAllUserTask(userID, queryParams))
	if totalCandidates <= 0 {
		return []*CanvasVideoTaskItem{}, 0, nil
	}

	const batchSize = 200
	filtered := make([]*CanvasVideoTaskItem, 0)
	for offset := 0; offset < totalCandidates; offset += batchSize {
		tasks := model.TaskGetAllUserTask(userID, offset, batchSize, queryParams)
		if len(tasks) == 0 {
			break
		}
		for _, task := range tasks {
			item := BuildCanvasVideoTaskItem(task)
			if item == nil {
				continue
			}
			if item.ModelID != modelID {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(tasks) < batchSize {
			break
		}
	}

	total := int64(len(filtered))
	if startIdx >= len(filtered) {
		return []*CanvasVideoTaskItem{}, total, nil
	}

	endIdx := startIdx + num
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}
	return filtered[startIdx:endIdx], total, nil
}

func GetCanvasVideoTaskDetail(userID int, taskID string) (*CanvasVideoTaskItem, error) {
	task, exists, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		return nil, err
	}
	if !exists || task == nil {
		return nil, fmt.Errorf("task not found")
	}
	return BuildCanvasVideoTaskItem(task), nil
}
