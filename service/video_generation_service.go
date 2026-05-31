package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/worker_setting"
)

const (
	videoCapabilityImageToVideo = "image_to_video"
	videoCapabilityTextToVideo  = "text_to_video"
)

var getVideoReferenceImageFromURL = GetImageFromUrl

type VideoGenerationParams struct {
	Duration        int      `json:"duration"`
	Resolution      string   `json:"resolution"`
	AspectRatio     string   `json:"aspect_ratio"`
	ReferenceImages []string `json:"reference_images"`
}

func ListVideoGenerationModels() ([]*dto.VideoGenerationModel, error) {
	mappings, _, err := model.GetActiveVideoModelMappings(0, 1000)
	if err != nil {
		return nil, err
	}
	items := make([]*dto.VideoGenerationModel, 0, len(mappings))
	for _, mapping := range mappings {
		capabilities, err := model.EffectiveVideoCapabilities(mapping.VideoCapabilities)
		if err != nil {
			return nil, err
		}
		durations, err := model.EffectiveDurationOptions(mapping.DurationOptions)
		if err != nil {
			return nil, err
		}
		requestEndpoint := normalizeVideoEndpoint(mapping.RequestEndpoint)
		items = append(items, &dto.VideoGenerationModel{
			RequestModel:      mapping.RequestModel,
			DisplayName:       mapping.DisplayName,
			ModelSeries:       mapping.ModelSeries,
			RequestEndpoint:   requestEndpoint,
			VideoCapabilities: capabilities,
			DurationOptions:   durations,
			Resolutions:       defaultVideoModelResolutions(requestEndpoint),
			AspectRatios:      defaultVideoModelAspectRatios(requestEndpoint),
		})
	}
	return items, nil
}

func CreateVideoGenerationTask(userId int, modelId string, prompt string, requestEndpoint string, rawParams string) (*dto.VideoGenerationTaskSummary, error) {
	requestEndpoint = normalizeVideoEndpoint(requestEndpoint)

	mapping, err := model.GetActiveModelMappingByRequestModel(modelId)
	if err != nil {
		return nil, fmt.Errorf("failed to get model mapping: %w", err)
	}
	if mapping == nil {
		return nil, fmt.Errorf("model mapping not found for: %s", modelId)
	}
	if mapping.ModelType != 3 {
		return nil, fmt.Errorf("model %s is not a video model", modelId)
	}
	if normalizeVideoEndpoint(mapping.RequestEndpoint) != requestEndpoint {
		return nil, fmt.Errorf("request endpoint mismatch: expected %s, got %s", mapping.RequestEndpoint, requestEndpoint)
	}

	var params VideoGenerationParams
	if strings.TrimSpace(rawParams) != "" {
		if err := common.UnmarshalJsonStr(rawParams, &params); err != nil {
			return nil, fmt.Errorf("failed to parse params: %w", err)
		}
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	hasReferenceImage := len(params.ReferenceImages) > 0 && strings.TrimSpace(params.ReferenceImages[0]) != ""
	supportsImageToVideo, err := model.HasVideoCapability(mapping.VideoCapabilities, videoCapabilityImageToVideo)
	if err != nil {
		return nil, err
	}
	supportsTextToVideo, err := model.HasVideoCapability(mapping.VideoCapabilities, videoCapabilityTextToVideo)
	if err != nil {
		return nil, err
	}
	if hasReferenceImage {
		if !supportsImageToVideo {
			return nil, fmt.Errorf("current model does not support image-to-video")
		}
		params.ReferenceImages = []string{params.ReferenceImages[0]}
	} else {
		if !supportsTextToVideo {
			return nil, fmt.Errorf("current model does not support text-to-video")
		}
		params.ReferenceImages = nil
	}

	durationOptions, err := model.EffectiveDurationOptions(mapping.DurationOptions)
	if err != nil {
		return nil, err
	}
	if !containsInt(durationOptions, params.Duration) {
		return nil, fmt.Errorf("unsupported duration: %d", params.Duration)
	}

	resolutionOptions := defaultVideoModelResolutions(requestEndpoint)
	if len(resolutionOptions) > 0 && !containsString(resolutionOptions, params.Resolution) {
		return nil, fmt.Errorf("unsupported resolution: %s", params.Resolution)
	}

	aspectRatioOptions := defaultVideoModelAspectRatios(requestEndpoint)
	if len(aspectRatioOptions) > 0 && !containsString(aspectRatioOptions, params.AspectRatio) {
		return nil, fmt.Errorf("unsupported aspect ratio: %s", params.AspectRatio)
	}

	respBody, publicTaskID, err := callUpstreamVideoAPIViaRelay(context.Background(), userId, modelId, prompt, requestEndpoint, params)
	if err != nil {
		return nil, err
	}

	task, err := waitVideoTaskCreated(userId, publicTaskID)
	if err != nil {
		return nil, err
	}
	if err := persistVideoTaskRequestContext(task, prompt, params); err != nil {
		common.SysLog(fmt.Sprintf("Failed to persist video task %d request context: %v", task.ID, err))
	}
	if len(respBody) > 0 && len(task.Data) == 0 {
		task.Data = respBody
		_ = task.Update()
	}
	return buildVideoTaskSummary(task), nil
}

func ListVideoGenerationTasks(userId int, page int, pageSize int, status string, modelID string, startTime int64, endTime int64) ([]*dto.VideoGenerationTaskSummary, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	items, total, err := model.GetUserVideoTasks(userId, (page-1)*pageSize, pageSize, model.VideoTaskQueryParams{
		Status:         normalizeVideoTaskStatus(status),
		ModelID:        modelID,
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
	}, nil)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*dto.VideoGenerationTaskSummary, 0, len(items))
	for _, task := range items {
		result = append(result, buildVideoTaskSummary(task))
	}
	return result, total, nil
}

func GetVideoGenerationTaskDetail(userId int, identifier string) (*dto.VideoGenerationTaskDetail, error) {
	task, err := model.GetUserVideoTaskByIdentifier(userId, identifier, nil)
	if err != nil {
		return nil, err
	}
	return buildVideoTaskDetail(task), nil
}

func RetryVideoGenerationTask(userId int, id int64) (*dto.VideoGenerationTaskSummary, error) {
	task, err := model.GetUserVideoTaskByID(userId, id, nil)
	if err != nil {
		return nil, err
	}
	if task.Status != model.TaskStatusFailure {
		return nil, fmt.Errorf("task %d is not failed", id)
	}

	modelID := strings.TrimSpace(task.Properties.OriginModelName)
	if modelID == "" {
		modelID = strings.TrimSpace(task.Properties.UpstreamModelName)
	}
	if modelID == "" {
		return nil, fmt.Errorf("task %d is missing model info", id)
	}
	mapping, err := model.GetModelMappingByRequestModel(modelID)
	if err != nil {
		return nil, err
	}
	if mapping == nil {
		return nil, fmt.Errorf("model mapping not found for: %s", modelID)
	}
	params, prompt, err := buildRetryVideoTaskPayload(task)
	if err != nil {
		return nil, err
	}
	paramsBytes, err := common.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry params: %w", err)
	}
	return CreateVideoGenerationTask(
		userId,
		modelID,
		prompt,
		mapping.RequestEndpoint,
		string(paramsBytes),
	)
}

func DeleteVideoGenerationTask(userId int, id int64) error {
	task, err := model.GetUserVideoTaskByID(userId, id, nil)
	if err != nil {
		return err
	}
	if task.Status == model.TaskStatusQueued || task.Status == model.TaskStatusInProgress || task.Status == model.TaskStatusSubmitted || task.Status == model.TaskStatusNotStart {
		return fmt.Errorf("running task cannot be deleted")
	}
	deleteVideoTaskStoredAssets(task)
	return model.DB.Delete(&model.Task{}, task.ID).Error
}

func normalizeVideoEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "openai-video-generations", "video-generation":
		return "openai-video-generation"
	case "openai-videos", "sora":
		return "openai-video"
	default:
		return strings.ToLower(strings.TrimSpace(endpoint))
	}
}

func normalizeVideoTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return string(model.TaskStatusQueued)
	case "in_progress", "processing":
		return string(model.TaskStatusInProgress)
	case "completed", "success", "succeeded":
		return string(model.TaskStatusSuccess)
	case "failed":
		return string(model.TaskStatusFailure)
	default:
		return ""
	}
}

func defaultVideoModelResolutions(requestEndpoint string) []string {
	switch requestEndpoint {
	case "openai-video":
		return []string{
			"1280x720",
			"720x1280",
			"1920x1080",
			"1080x1920",
			"1024x1024",
			"960x960",
			"1792x1024",
			"1024x1792",
		}
	default:
		return nil
	}
}

func defaultVideoModelAspectRatios(requestEndpoint string) []string {
	switch requestEndpoint {
	case "openai-video":
		return []string{"16:9", "9:16", "1:1"}
	default:
		return nil
	}
}

func callUpstreamVideoAPIViaRelay(ctx context.Context, userId int, modelId string, prompt string, requestEndpoint string, params VideoGenerationParams) ([]byte, string, error) {
	userToken, err := getUserValidToken(userId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user token: %w", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	requestURL := fmt.Sprintf("http://127.0.0.1:%s/v1/video/generations", port)
	if normalizeVideoEndpoint(requestEndpoint) == "openai-video" {
		requestURL = fmt.Sprintf("http://127.0.0.1:%s/v1/videos", port)
	}

	var imageInput string
	if len(params.ReferenceImages) > 0 {
		imageInput = strings.TrimSpace(params.ReferenceImages[0])
		if !model.IsProbablyDataURL(imageInput) {
			converted, convErr := referenceImageAsDataURL(ctx, imageInput)
			if convErr != nil {
				return nil, "", fmt.Errorf("failed to load reference image: %w", convErr)
			}
			imageInput = converted
		}
	}

	requestBody, contentType, err := buildVideoRelaySubmitBody(modelId, prompt, requestEndpoint, params, imageInput)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, requestBody)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create video request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to send video request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read video response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("video relay error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	publicTaskID, _ := extractPublicTaskID(body)
	if publicTaskID == "" {
		return nil, "", fmt.Errorf("video relay did not return task id")
	}
	return body, publicTaskID, nil
}

func buildVideoRelaySubmitBody(modelId string, prompt string, requestEndpoint string, params VideoGenerationParams, imageInput string) (io.Reader, string, error) {
	videoReq := relaycommon.TaskSubmitReq{
		Prompt:   prompt,
		Model:    modelId,
		Duration: params.Duration,
		Seconds:  fmt.Sprintf("%d", params.Duration),
		Size:     params.Resolution,
	}
	if normalizeVideoEndpoint(requestEndpoint) == "openai-video" {
		if imageInput != "" {
			body, contentType, err := buildOpenAIVideoMultipartSubmitBody(modelId, prompt, params, imageInput)
			if err != nil {
				return nil, "", err
			}
			return body, contentType, nil
		}
	} else {
		if imageInput != "" {
			videoReq.Image = imageInput
			videoReq.Images = []string{imageInput}
			videoReq.InputReference = imageInput
		}
		videoReq.Metadata = map[string]interface{}{}
		if params.Resolution != "" {
			videoReq.Metadata["resolution"] = params.Resolution
		}
		if params.AspectRatio != "" {
			videoReq.Metadata["aspect_ratio"] = params.AspectRatio
		}
		if params.Duration > 0 {
			videoReq.Metadata["duration"] = params.Duration
		}
	}

	jsonData, err := common.Marshal(videoReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal video request: %w", err)
	}
	return bytes.NewBuffer(jsonData), "application/json", nil
}

func buildOpenAIVideoMultipartSubmitBody(modelId string, prompt string, params VideoGenerationParams, imageInput string) (*bytes.Buffer, string, error) {
	mimeType, imageBytes, err := resolveVideoReferenceImageBytes(imageInput)
	if err != nil {
		return nil, "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", modelId); err != nil {
		return nil, "", fmt.Errorf("failed to write model field: %w", err)
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, "", fmt.Errorf("failed to write prompt field: %w", err)
	}
	if params.Duration > 0 {
		if err := writer.WriteField("seconds", strconv.Itoa(params.Duration)); err != nil {
			return nil, "", fmt.Errorf("failed to write seconds field: %w", err)
		}
	}
	if strings.TrimSpace(params.Resolution) != "" {
		if err := writer.WriteField("size", strings.TrimSpace(params.Resolution)); err != nil {
			return nil, "", fmt.Errorf("failed to write size field: %w", err)
		}
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="input_reference"; filename="%s"`, videoReferenceImageFilename(mimeType)))
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create input_reference part: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, "", fmt.Errorf("failed to write input_reference part: %w", err)
	}
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize multipart body: %w", err)
	}
	return &body, contentType, nil
}

func resolveVideoReferenceImageBytes(imageInput string) (string, []byte, error) {
	imageInput = strings.TrimSpace(imageInput)
	if imageInput == "" {
		return "", nil, fmt.Errorf("reference image is required")
	}
	if model.IsProbablyHTTPURL(imageInput) {
		mimeType, base64Data, err := getVideoReferenceImageFromURL(imageInput)
		if err != nil {
			return "", nil, fmt.Errorf("failed to download reference image: %w", err)
		}
		imageBytes, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return "", nil, fmt.Errorf("failed to decode downloaded reference image: %w", err)
		}
		return mimeType, imageBytes, nil
	}

	mimeType, base64Data, err := DecodeBase64FileData(imageInput)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode reference image data: %w", err)
	}
	imageBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode reference image bytes: %w", err)
	}
	return mimeType, imageBytes, nil
}

func videoReferenceImageFilename(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return "reference.jpg"
	case "image/webp":
		return "reference.webp"
	case "image/gif":
		return "reference.gif"
	default:
		return "reference.png"
	}
}

func extractPublicTaskID(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	var payload map[string]interface{}
	if err := common.Unmarshal(data, &payload); err != nil {
		return "", false
	}
	if taskID, ok := payload["task_id"].(string); ok && strings.TrimSpace(taskID) != "" {
		return strings.TrimSpace(taskID), true
	}
	if id, ok := payload["id"].(string); ok && strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id), true
	}
	return "", false
}

func waitVideoTaskCreated(userId int, taskID string) (*model.Task, error) {
	for i := 0; i < 20; i++ {
		task, exists, err := model.GetByTaskId(userId, taskID)
		if err != nil {
			return nil, err
		}
		if exists && task != nil {
			return task, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("video task not found after submit: %s", taskID)
}

func buildVideoTaskSummary(task *model.Task) *dto.VideoGenerationTaskSummary {
	if task == nil {
		return nil
	}

	modelID := strings.TrimSpace(task.Properties.OriginModelName)
	if modelID == "" {
		modelID = strings.TrimSpace(task.Properties.UpstreamModelName)
	}

	displayName := modelID
	requestEndpoint := ""
	if mapping, err := model.GetModelMappingByRequestModel(modelID); err == nil && mapping != nil {
		if strings.TrimSpace(mapping.DisplayName) != "" {
			displayName = strings.TrimSpace(mapping.DisplayName)
		}
		requestEndpoint = strings.TrimSpace(mapping.RequestEndpoint)
	}

	duration, resolution, aspectRatio, prompt := extractVideoTaskContext(task)

	return &dto.VideoGenerationTaskSummary{
		ID:              task.ID,
		TaskID:          task.TaskID,
		Status:          task.Status.ToVideoStatus(),
		Progress:        task.Progress,
		Prompt:          prompt,
		ModelID:         modelID,
		DisplayName:     displayName,
		RequestEndpoint: requestEndpoint,
		RequestType:     model.GuessTaskVideoPreviewType(task),
		Duration:        duration,
		Resolution:      resolution,
		AspectRatio:     aspectRatio,
		CreatedTime:     task.SubmitTime,
		StartedTime:     task.StartTime,
		CompletedTime:   task.FinishTime,
		ThumbnailURL:    model.ExtractTaskThumbnailURL(task),
		VideoURL:        model.BuildVideoProxyURL(task),
		ResultURL:       model.EffectiveVideoResultURL(task),
		FailReason:      task.FailReason,
	}
}

func buildVideoTaskDetail(task *model.Task) *dto.VideoGenerationTaskDetail {
	summary := buildVideoTaskSummary(task)
	if summary == nil {
		return nil
	}
	return &dto.VideoGenerationTaskDetail{
		VideoGenerationTaskSummary: *summary,
		Quota:                      task.Quota,
	}
}

func persistVideoTaskRequestContext(task *model.Task, prompt string, params VideoGenerationParams) error {
	if task == nil {
		return nil
	}
	task.Properties.Input = strings.TrimSpace(prompt)
	req := relaycommon.TaskSubmitReq{
		Prompt:   strings.TrimSpace(prompt),
		Duration: params.Duration,
		Seconds:  fmt.Sprintf("%d", params.Duration),
		Size:     strings.TrimSpace(params.Resolution),
		Metadata: map[string]interface{}{},
	}
	if params.AspectRatio != "" {
		req.Metadata["aspect_ratio"] = strings.TrimSpace(params.AspectRatio)
	}
	if params.Resolution != "" {
		req.Metadata["resolution"] = strings.TrimSpace(params.Resolution)
	}
	if len(params.ReferenceImages) > 0 && strings.TrimSpace(params.ReferenceImages[0]) != "" {
		storedImage, err := storeImageGenerationReferenceImage(context.Background(), int(task.ID), params.ReferenceImages[0])
		if err != nil {
			return err
		}
		req.Image = storedImage
		req.Images = []string{storedImage}
		req.InputReference = storedImage
	}
	reqBytes, err := common.Marshal(req)
	if err != nil {
		return err
	}
	task.Properties.RequestParams = string(reqBytes)
	return task.Update()
}

func extractVideoTaskContext(task *model.Task) (duration int, resolution string, aspectRatio string, prompt string) {
	if task == nil {
		return 0, "", "", ""
	}

	prompt = strings.TrimSpace(task.Properties.Input)
	if strings.TrimSpace(task.Properties.RequestParams) != "" {
		var req relaycommon.TaskSubmitReq
		if err := common.UnmarshalJsonStr(task.Properties.RequestParams, &req); err == nil {
			if prompt == "" {
				prompt = strings.TrimSpace(req.Prompt)
			}
			if req.Duration > 0 {
				duration = req.Duration
			}
			if strings.TrimSpace(req.Size) != "" {
				resolution = strings.TrimSpace(req.Size)
			}
			if req.Metadata != nil {
				if v, ok := req.Metadata["resolution"].(string); ok && strings.TrimSpace(v) != "" {
					resolution = strings.TrimSpace(v)
				}
				if v, ok := req.Metadata["aspect_ratio"].(string); ok && strings.TrimSpace(v) != "" {
					aspectRatio = strings.TrimSpace(v)
				}
				if duration == 0 {
					switch typed := req.Metadata["duration"].(type) {
					case float64:
						duration = int(typed)
					case int:
						duration = typed
					}
				}
			}
		}
	}

	if duration == 0 {
		duration = model.ExtractTaskDuration(task)
	}
	if resolution == "" {
		resolution = model.ExtractTaskResolution(task)
	}
	if aspectRatio == "" {
		aspectRatio = model.ExtractTaskAspectRatio(task)
	}

	return duration, resolution, aspectRatio, prompt
}

func buildRetryVideoTaskPayload(task *model.Task) (VideoGenerationParams, string, error) {
	if task == nil {
		return VideoGenerationParams{}, "", fmt.Errorf("task is nil")
	}
	prompt := strings.TrimSpace(task.Properties.Input)
	if strings.TrimSpace(task.Properties.RequestParams) == "" {
		return VideoGenerationParams{}, prompt, fmt.Errorf("task %d is missing request params", task.ID)
	}
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalJsonStr(task.Properties.RequestParams, &req); err != nil {
		return VideoGenerationParams{}, prompt, fmt.Errorf("failed to parse stored request params: %w", err)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	params := VideoGenerationParams{
		Duration:        req.Duration,
		Resolution:      strings.TrimSpace(req.Size),
		ReferenceImages: nil,
	}
	if strings.TrimSpace(req.Image) != "" {
		params.ReferenceImages = []string{strings.TrimSpace(req.Image)}
	} else if strings.TrimSpace(req.InputReference) != "" {
		params.ReferenceImages = []string{strings.TrimSpace(req.InputReference)}
	}
	if req.Metadata != nil {
		if v, ok := req.Metadata["aspect_ratio"].(string); ok {
			params.AspectRatio = strings.TrimSpace(v)
		}
		if v, ok := req.Metadata["resolution"].(string); ok && strings.TrimSpace(v) != "" {
			params.Resolution = strings.TrimSpace(v)
		}
		if params.Duration == 0 {
			switch typed := req.Metadata["duration"].(type) {
			case float64:
				params.Duration = int(typed)
			case int:
				params.Duration = typed
			}
		}
	}
	if params.Duration == 0 {
		params.Duration = model.ExtractTaskDuration(task)
	}
	return params, prompt, nil
}

func deleteVideoTaskStoredAssets(task *model.Task) {
	if task == nil {
		return
	}
	cfg := worker_setting.GetWorkerSetting()
	if strings.TrimSpace(task.Properties.RequestParams) != "" {
		var req relaycommon.TaskSubmitReq
		if err := common.UnmarshalJsonStr(task.Properties.RequestParams, &req); err == nil {
			imageURL := strings.TrimSpace(req.Image)
			if imageURL == "" {
				imageURL = strings.TrimSpace(req.InputReference)
			}
			if imageURL != "" {
				if err := deleteImageFile(imageURL, cfg); err != nil {
					common.SysLog(fmt.Sprintf("Failed to delete video task %d reference image: %v", task.ID, err))
				}
			}
		}
	}
	if resultURL := strings.TrimSpace(task.GetResultURL()); resultURL != "" && isImageGenerationStoredAssetURL(resultURL) {
		if err := deleteImageFile(resultURL, cfg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to delete video task %d result asset: %v", task.ID, err))
		}
	}
	if thumbnailURL := strings.TrimSpace(model.ExtractTaskThumbnailURL(task)); thumbnailURL != "" && thumbnailURL != strings.TrimSpace(task.GetResultURL()) && isImageGenerationStoredAssetURL(thumbnailURL) {
		if err := deleteImageFile(thumbnailURL, cfg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to delete video task %d reference image: %v", task.ID, err))
		}
	}
}

func parseJSONStringArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := common.UnmarshalJsonStr(raw, &values); err != nil {
		return nil
	}
	return values
}

func containsInt(items []int, target int) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}
