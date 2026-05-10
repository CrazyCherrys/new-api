package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/worker_setting"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	imageWorkerLimiter         imageGenerationWorkerLimiter
	enqueueImageGenerationTask = func(taskId int) {
		go processTaskAsync(taskId)
	}
)

type imageGenerationWorkerLimiter struct {
	mutex  sync.Mutex
	active int
}

func normalizeImageEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "dalle":
		return "openai"
	default:
		return strings.ToLower(strings.TrimSpace(endpoint))
	}
}

func imageEndpointIsResponses(endpoint string) bool {
	switch normalizeImageEndpoint(endpoint) {
	case "openai-response":
		return true
	default:
		return false
	}
}

func resolveOpenAIImageSize(resolution, aspectRatio string) string {
	size, _ := ResolveOpenAIImageSize(resolution, aspectRatio)
	return size
}

func buildOpenAIResponsesImageRequest(imageReq *dto.ImageRequest) (*dto.OpenAIResponsesRequest, error) {
	if imageReq == nil {
		return nil, fmt.Errorf("image request is nil")
	}

	var inputRaw []byte
	if len(imageReq.ReferenceImages) > 0 {
		content := make([]map[string]any, 0, len(imageReq.ReferenceImages)+1)
		content = append(content, map[string]any{
			"type": "input_text",
			"text": imageReq.Prompt,
		})
		for _, refImage := range imageReq.ReferenceImages {
			if strings.TrimSpace(refImage) == "" {
				continue
			}
			content = append(content, map[string]any{
				"type":      "input_image",
				"image_url": refImage,
			})
		}
		inputItems := []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		}
		raw, err := common.Marshal(inputItems)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal responses input: %w", err)
		}
		inputRaw = raw
	} else {
		raw, err := common.Marshal(imageReq.Prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal responses prompt: %w", err)
		}
		inputRaw = raw
	}

	tool := map[string]any{
		"type": "image_generation",
	}
	if mappedSize, ok := ResolveOpenAIImageSize(imageReq.Resolution, imageReq.AspectRatio); ok {
		tool["size"] = mappedSize
	} else if explicitSize := strings.TrimSpace(imageReq.Size); explicitSize != "" {
		tool["size"] = explicitSize
	} else {
		tool["size"] = mappedSize
	}
	if strings.TrimSpace(imageReq.Quality) != "" {
		tool["quality"] = strings.TrimSpace(imageReq.Quality)
	}
	if strings.TrimSpace(imageReq.Mask) != "" {
		tool["input_image_mask"] = map[string]any{
			"image_url": imageReq.Mask,
		}
	}
	if len(imageReq.ReferenceImages) > 0 || strings.TrimSpace(imageReq.Mask) != "" {
		tool["action"] = "edit"
	} else {
		tool["action"] = "generate"
	}

	toolsBytes, err := common.Marshal([]map[string]any{tool})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses tools: %w", err)
	}

	toolChoiceBytes, err := common.Marshal(map[string]any{
		"type": "image_generation",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses tool choice: %w", err)
	}

	return &dto.OpenAIResponsesRequest{
		Model:      imageReq.Model,
		Input:      inputRaw,
		Tools:      toolsBytes,
		ToolChoice: toolChoiceBytes,
	}, nil
}

func normalizeOpenAIResponsesImageResult(result string) string {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "data:") ||
		strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}

	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, trimmed)

	if _, err := base64.StdEncoding.DecodeString(cleaned); err == nil {
		return "data:image/png;base64," + cleaned
	}
	if _, err := base64.RawStdEncoding.DecodeString(cleaned); err == nil {
		return "data:image/png;base64," + cleaned
	}

	return trimmed
}

func imageGenerationTimeout() time.Duration {
	cfg := worker_setting.GetWorkerSetting()
	timeout := time.Duration(cfg.ImageTimeout) * time.Second
	if timeout <= 0 {
		return 120 * time.Second
	}
	return timeout
}

func imageGenerationMaxWorkers() int {
	cfg := worker_setting.GetWorkerSetting()
	maxWorkers := cfg.MaxWorkers
	if maxWorkers <= 0 {
		return 4
	}
	return maxWorkers
}

func imageGenerationMaxRetries() int {
	cfg := worker_setting.GetWorkerSetting()
	if cfg.MaxRetries < 0 {
		return 0
	}
	return cfg.MaxRetries
}

func imageGenerationRetryDelay() time.Duration {
	cfg := worker_setting.GetWorkerSetting()
	retryDelay := time.Duration(cfg.RetryDelay) * time.Second
	if retryDelay <= 0 {
		return 5 * time.Second
	}
	return retryDelay
}

func (l *imageGenerationWorkerLimiter) acquire(ctx context.Context) error {
	wait := time.NewTicker(200 * time.Millisecond)
	defer wait.Stop()

	for {
		l.mutex.Lock()
		if l.active < imageGenerationMaxWorkers() {
			l.active++
			l.mutex.Unlock()
			return nil
		}
		l.mutex.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait.C:
		}
	}
}

func (l *imageGenerationWorkerLimiter) release() {
	l.mutex.Lock()
	if l.active > 0 {
		l.active--
	}
	l.mutex.Unlock()
}

// CreateImageGenerationTask 创建图片生成任务
func CreateImageGenerationTask(userId int, modelId string, prompt string, requestEndpoint string, params string) (*model.ImageGenerationTask, error) {
	requestEndpoint = normalizeImageEndpoint(requestEndpoint)
	if err := validateImageGenerationReferenceImages(params); err != nil {
		return nil, err
	}
	mapping, err := model.GetActiveModelMappingByRequestModel(modelId)
	if err != nil {
		return nil, fmt.Errorf("failed to get model mapping: %w", err)
	}
	if mapping == nil {
		return nil, fmt.Errorf("model mapping not found for: %s", modelId)
	}
	mappingEndpoint := normalizeImageEndpoint(mapping.RequestEndpoint)
	if mappingEndpoint != requestEndpoint {
		return nil, fmt.Errorf("request endpoint mismatch: expected %s, got %s", mappingEndpoint, requestEndpoint)
	}
	if err := validateImageGenerationModelCapabilities(mapping, params); err != nil {
		return nil, err
	}

	// 检查用户余额
	userQuota, err := model.GetUserQuota(userId, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quota: %w", err)
	}

	// 预估费用（使用模型价格或倍率）
	estimatedCost := estimateImageGenerationCost(modelId)
	if userQuota < estimatedCost {
		return nil, fmt.Errorf("insufficient quota: required %s, available %s",
			logger.FormatQuota(estimatedCost),
			logger.FormatQuota(userQuota))
	}

	var (
		paramMap                map[string]interface{}
		referenceInputs         []string
		maskInput               string
		hasMaskInput            bool
		hadLegacyReferenceImage bool
		storedParams            = params
	)
	if strings.TrimSpace(params) != "" {
		if err := common.UnmarshalJsonStr(params, &paramMap); err != nil {
			return nil, fmt.Errorf("failed to parse params: %w", err)
		}
		referenceInputs, hadLegacyReferenceImage = collectImageGenerationReferenceImagesFromParamsMap(paramMap)
		maskInput, hasMaskInput = collectImageGenerationMaskFromParamsMap(paramMap)
		if len(referenceInputs) > 0 || strings.TrimSpace(maskInput) != "" || hasMaskInput {
			setImageGenerationReferenceImagesInParamsMap(paramMap, nil, false)
			setImageGenerationMaskInParamsMap(paramMap, "")
			storedParamsBytes, err := common.Marshal(paramMap)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal stored params: %w", err)
			}
			storedParams = string(storedParamsBytes)
		}
	}

	// 创建任务记录
	task := &model.ImageGenerationTask{
		UserId:          userId,
		ModelId:         modelId,
		Prompt:          prompt,
		RequestEndpoint: requestEndpoint,
		Status:          model.ImageTaskStatusPending,
		Params:          storedParams,
		Cost:            0, // 实际费用在完成后计算
		CreatedTime:     common.GetTimestamp(),
	}

	if err := task.Insert(); err != nil {
		return nil, fmt.Errorf("failed to insert task: %w", err)
	}

	if len(referenceInputs) > 0 || strings.TrimSpace(maskInput) != "" {
		storedRefs, err := storeImageGenerationReferenceImages(context.Background(), task.Id, referenceInputs)
		if err != nil {
			_ = model.DeleteImageTask(task.Id)
			return nil, fmt.Errorf("failed to store reference images: %w", err)
		}
		storedMask := ""
		if strings.TrimSpace(maskInput) != "" {
			storedMask, err = storeImageGenerationReferenceImage(context.Background(), task.Id, maskInput)
			if err != nil {
				cleanupStoredImageGenerationAssets(storedRefs)
				_ = model.DeleteImageTask(task.Id)
				return nil, fmt.Errorf("failed to store mask image: %w", err)
			}
		}
		setImageGenerationReferenceImagesInParamsMap(paramMap, storedRefs, hadLegacyReferenceImage)
		setImageGenerationMaskInParamsMap(paramMap, storedMask)
		storedParamsBytes, err := common.Marshal(paramMap)
		if err != nil {
			cleanupStoredImageGenerationAssets(append(storedRefs, storedMask))
			_ = model.DeleteImageTask(task.Id)
			return nil, fmt.Errorf("failed to marshal stored params: %w", err)
		}
		task.Params = string(storedParamsBytes)
		if err := task.Update(); err != nil {
			cleanupStoredImageGenerationAssets(append(storedRefs, storedMask))
			_ = model.DeleteImageTask(task.Id)
			return nil, fmt.Errorf("failed to update stored params: %w", err)
		}
	}

	// 启动异步处理
	enqueueImageGenerationTask(task.Id)

	return task, nil
}

// processTaskAsync 异步处理任务
func processTaskAsync(taskId int) {
	timeout := imageGenerationTimeout()

	// 创建超时上下文，包括等待 worker pool 的时间
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 尝试获取 worker slot，带超时
	if err := imageWorkerLimiter.acquire(ctx); err != nil {
		// 超时：标记任务为失败
		errorMsg := fmt.Sprintf("task timeout: failed to acquire worker slot within %v", timeout)
		if err := model.UpdateImageTaskStatus(taskId, model.ImageTaskStatusFailed, errorMsg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to update task %d status on timeout: %v", taskId, err))
		}
		common.SysLog(fmt.Sprintf("Task %d timed out waiting for worker slot", taskId))
		return
	}
	defer imageWorkerLimiter.release()

	// 处理任务
	if err := ProcessImageGenerationTask(taskId); err != nil {
		common.SysLog(fmt.Sprintf("Failed to process image generation task %d: %v", taskId, err))
	}
}

// RetryImageGenerationTask 重新排队失败任务并立即执行
func RetryImageGenerationTask(taskId int) error {
	task, err := model.GetImageTaskByID(taskId)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found: %d", taskId)
	}
	if task.Status != model.ImageTaskStatusFailed {
		return fmt.Errorf("task %d is not failed (status: %s)", taskId, task.Status)
	}

	updated, err := model.ResetImageTaskForRetry(taskId)
	if err != nil {
		return fmt.Errorf("failed to reset task for retry: %w", err)
	}
	if !updated {
		return fmt.Errorf("task %d is not failed", taskId)
	}

	enqueueImageGenerationTask(taskId)
	return nil
}

// ProcessImageGenerationTask 处理图片生成任务
func ProcessImageGenerationTask(taskId int) error {
	// 获取任务
	task, err := model.GetImageTaskByID(taskId)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found: %d", taskId)
	}

	// 检查任务状态
	if task.Status != model.ImageTaskStatusPending {
		return fmt.Errorf("task %d is not pending (status: %s)", taskId, task.Status)
	}

	// 更新状态为处理中
	if err := model.UpdateImageTaskStatus(taskId, model.ImageTaskStatusGenerating, ""); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	timeout := imageGenerationTimeout()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 重试逻辑
	maxRetries := imageGenerationMaxRetries()
	retryDelay := imageGenerationRetryDelay()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			common.SysLog(fmt.Sprintf("Retrying task %d (attempt %d/%d)", taskId, attempt, maxRetries))
			select {
			case <-ctx.Done():
				lastErr = fmt.Errorf("task timeout after %v", timeout)
				break
			case <-time.After(retryDelay):
			}
		}

		// 执行图片生成
		imageUrl, metadata, cost, err := generateImage(ctx, task)
		if err == nil {
			// 成功：更新任务结果
			if err := model.UpdateImageTaskResult(taskId, imageUrl, metadata, cost); err != nil {
				return fmt.Errorf("failed to update task result: %w", err)
			}

			// 扣除用户额度
			if err := deductUserQuota(task.UserId, cost); err != nil {
				common.SysLog(fmt.Sprintf("Failed to deduct quota for task %d: %v", taskId, err))
			}

			common.SysLog(fmt.Sprintf("Task %d completed successfully, cost: %s", taskId, logger.FormatQuota(cost)))
			return nil
		}

		lastErr = err
		common.SysLog(fmt.Sprintf("Task %d attempt %d failed: %v", taskId, attempt+1, err))
	}

	// 所有重试都失败
	errorMsg := fmt.Sprintf("failed after %d attempts: %v", maxRetries+1, lastErr)
	if err := model.UpdateImageTaskStatus(taskId, model.ImageTaskStatusFailed, errorMsg); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return fmt.Errorf("task %d failed: %s", taskId, errorMsg)
}

func extractImageGenerationReferenceImages(params string) ([]string, error) {
	if strings.TrimSpace(params) == "" {
		return nil, nil
	}

	var paramMap map[string]interface{}
	if err := common.UnmarshalJsonStr(params, &paramMap); err != nil {
		return nil, fmt.Errorf("failed to parse params: %w", err)
	}
	referenceImages, _ := collectImageGenerationReferenceImagesFromParamsMap(paramMap)
	return referenceImages, nil
}

func extractImageGenerationMask(params string) (string, error) {
	if strings.TrimSpace(params) == "" {
		return "", nil
	}

	var paramMap map[string]interface{}
	if err := common.UnmarshalJsonStr(params, &paramMap); err != nil {
		return "", fmt.Errorf("failed to parse params: %w", err)
	}
	mask, _ := collectImageGenerationMaskFromParamsMap(paramMap)
	return mask, nil
}

func collectImageGenerationReferenceImagesFromParamsMap(paramMap map[string]interface{}) ([]string, bool) {
	if len(paramMap) == 0 {
		return nil, false
	}

	refs := make([]string, 0)
	if raw, ok := paramMap["reference_images"]; ok {
		switch typed := raw.(type) {
		case []interface{}:
			for _, item := range typed {
				if ref, ok := item.(string); ok {
					ref = strings.TrimSpace(ref)
					if ref != "" {
						refs = append(refs, ref)
					}
				}
			}
		case []string:
			for _, item := range typed {
				item = strings.TrimSpace(item)
				if item != "" {
					refs = append(refs, item)
				}
			}
		}
	}

	hadLegacyReferenceImage := false
	if raw, ok := paramMap["reference_image"]; ok {
		hadLegacyReferenceImage = true
		if ref, ok := raw.(string); ok {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				refs = append([]string{ref}, refs...)
			}
		}
	}

	if len(refs) == 0 {
		return nil, hadLegacyReferenceImage
	}

	seen := make(map[string]struct{}, len(refs))
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out, hadLegacyReferenceImage
}

func collectImageGenerationMaskFromParamsMap(paramMap map[string]interface{}) (string, bool) {
	if len(paramMap) == 0 {
		return "", false
	}
	raw, ok := paramMap["mask"]
	if !ok {
		return "", false
	}
	mask, ok := raw.(string)
	if !ok {
		return "", true
	}
	return strings.TrimSpace(mask), true
}

func setImageGenerationReferenceImagesInParamsMap(paramMap map[string]interface{}, refs []string, keepLegacySingle bool) {
	delete(paramMap, "reference_images")
	delete(paramMap, "reference_image")
	if len(refs) == 0 {
		return
	}
	paramMap["reference_images"] = refs
	if keepLegacySingle {
		paramMap["reference_image"] = refs[0]
	}
}

func setImageGenerationMaskInParamsMap(paramMap map[string]interface{}, mask string) {
	delete(paramMap, "mask")
	mask = strings.TrimSpace(mask)
	if mask == "" {
		return
	}
	paramMap["mask"] = mask
}

func hasImageGenerationEditInputs(params string) (bool, error) {
	if strings.TrimSpace(params) == "" {
		return false, nil
	}

	var paramMap map[string]interface{}
	if err := common.UnmarshalJsonStr(params, &paramMap); err != nil {
		return false, fmt.Errorf("failed to parse params: %w", err)
	}
	referenceImages, _ := collectImageGenerationReferenceImagesFromParamsMap(paramMap)
	if len(referenceImages) > 0 {
		return true, nil
	}
	mask, _ := collectImageGenerationMaskFromParamsMap(paramMap)
	return strings.TrimSpace(mask) != "", nil
}

func SanitizeImageGenerationParamsForResponse(params string) string {
	if strings.TrimSpace(params) == "" {
		return ""
	}
	var paramMap map[string]interface{}
	if err := common.UnmarshalJsonStr(params, &paramMap); err != nil {
		return params
	}
	setImageGenerationReferenceImagesInParamsMap(paramMap, nil, false)
	setImageGenerationMaskInParamsMap(paramMap, "")
	if len(paramMap) == 0 {
		return ""
	}
	data, err := common.Marshal(paramMap)
	if err != nil {
		return params
	}
	return string(data)
}

func validateImageGenerationReferenceImages(params string) error {
	referenceImages, err := extractImageGenerationReferenceImages(params)
	if err != nil {
		return err
	}
	mask, err := extractImageGenerationMask(params)
	if err != nil {
		return err
	}

	cfg := worker_setting.GetWorkerSetting()
	maxImageSizeMB := cfg.MaxImageSize
	if maxImageSizeMB <= 0 {
		maxImageSizeMB = 10
	}
	maxBytes := int64(maxImageSizeMB) * 1024 * 1024
	for idx, imageData := range referenceImages {
		size, ok, err := referenceImageDecodedSize(imageData)
		if err != nil {
			return fmt.Errorf("reference image %d is invalid: %w", idx+1, err)
		}
		if !ok {
			continue
		}
		if size > maxBytes {
			return fmt.Errorf("reference image %d exceeds maximum size: %.2fMB > %dMB", idx+1, float64(size)/1024/1024, maxImageSizeMB)
		}
	}
	if strings.TrimSpace(mask) != "" {
		size, ok, err := referenceImageDecodedSize(mask)
		if err != nil {
			return fmt.Errorf("mask image is invalid: %w", err)
		}
		if ok && size > maxBytes {
			return fmt.Errorf("mask image exceeds maximum size: %.2fMB > %dMB", float64(size)/1024/1024, maxImageSizeMB)
		}
	}
	return nil
}

func validateImageGenerationModelCapabilities(mapping *model.ModelMapping, params string) error {
	hasEditInputs, err := hasImageGenerationEditInputs(params)
	if err != nil {
		return err
	}
	referenceImages, err := extractImageGenerationReferenceImages(params)
	if err != nil {
		return err
	}
	mask, err := extractImageGenerationMask(params)
	if err != nil {
		return err
	}
	if strings.TrimSpace(mask) != "" && len(referenceImages) == 0 {
		return fmt.Errorf("mask image requires at least one reference image")
	}
	capabilities, err := model.EffectiveImageCapabilities(mapping.ImageCapabilities)
	if err != nil {
		return fmt.Errorf("invalid image capabilities for model %s: %w", mapping.RequestModel, err)
	}

	if hasEditInputs {
		for _, capability := range capabilities {
			if capability == model.ImageCapabilityEditing {
				return nil
			}
		}
		return fmt.Errorf("selected model does not support image editing")
	}

	for _, capability := range capabilities {
		if capability == model.ImageCapabilityGeneration {
			return nil
		}
	}
	return fmt.Errorf("selected model does not support image generation")
}

func referenceImageDecodedSize(raw string) (int64, bool, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return 0, false, nil
	}
	if strings.HasPrefix(payload, "http://") || strings.HasPrefix(payload, "https://") {
		return 0, false, nil
	}
	if strings.HasPrefix(payload, "data:") {
		commaIndex := strings.Index(payload, ",")
		if commaIndex < 0 {
			return 0, true, fmt.Errorf("invalid data URL")
		}
		payload = payload[commaIndex+1:]
	}
	payload = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, payload)
	if payload == "" {
		return 0, true, fmt.Errorf("empty image payload")
	}

	if decoded, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return int64(len(decoded)), true, nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return 0, true, err
	}
	return int64(len(decoded)), true, nil
}

// generateImage 调用上游 API 生成图片
func generateImage(ctx context.Context, task *model.ImageGenerationTask) (imageUrl string, metadata string, cost int, err error) {
	// 解析请求参数
	var params map[string]interface{}
	if task.Params != "" {
		if err := common.Unmarshal([]byte(task.Params), &params); err != nil {
			return "", "", 0, fmt.Errorf("failed to parse params: %w", err)
		}
	}

	// 构建图片请求
	taskEndpoint := normalizeImageEndpoint(task.RequestEndpoint)
	imageReq := &dto.ImageRequest{
		Model:           task.ModelId,
		Prompt:          task.Prompt,
		RequestEndpoint: taskEndpoint,
	}

	// 保留原始参数传递给relay层
	imageReq.RawParams = params

	// 应用参数（仅用于标准OpenAI API兼容）
	if size, ok := params["size"].(string); ok && size != "" {
		imageReq.Size = size
	}

	if quality, ok := params["quality"].(string); ok && quality != "" {
		imageReq.Quality = quality
	}

	// Gemini 专用参数：通过正式 JSON 字段传递，确保经 HTTP 序列化后不丢失
	if ar, ok := params["aspect_ratio"].(string); ok && ar != "" {
		imageReq.AspectRatio = ar
	}
	if res, ok := params["resolution"].(string); ok && res != "" {
		imageReq.Resolution = res
	}

	// 参考图片：从 params 提取并写入可序列化字段
	if referenceImages, _ := collectImageGenerationReferenceImagesFromParamsMap(params); len(referenceImages) > 0 {
		for _, ref := range referenceImages {
			dataURL, convErr := referenceImageAsDataURL(ctx, ref)
			if convErr != nil {
				return "", "", 0, fmt.Errorf("failed to load reference image: %w", convErr)
			}
			imageReq.ReferenceImages = append(imageReq.ReferenceImages, dataURL)
		}
	}
	if mask, _ := collectImageGenerationMaskFromParamsMap(params); strings.TrimSpace(mask) != "" {
		dataURL, convErr := referenceImageAsDataURL(ctx, mask)
		if convErr != nil {
			return "", "", 0, fmt.Errorf("failed to load mask image: %w", convErr)
		}
		imageReq.Mask = dataURL
	}

	var nVal float64
	hasN := false
	if v, ok := params["n"].(float64); ok {
		nVal = v
		hasN = true
	}
	if hasN && nVal > 0 {
		nUint := uint(nVal)
		imageReq.N = &nUint
	}

	// 获取模型映射
	mapping, err := model.GetActiveModelMappingByRequestModel(task.ModelId)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get model mapping: %w", err)
	}
	if mapping == nil {
		return "", "", 0, fmt.Errorf("model mapping not found for: %s", task.ModelId)
	}

	// 验证 request_endpoint
	mappingEndpoint := normalizeImageEndpoint(mapping.RequestEndpoint)
	if mappingEndpoint != taskEndpoint {
		return "", "", 0, fmt.Errorf("request endpoint mismatch: expected %s, got %s", mappingEndpoint, taskEndpoint)
	}

	// 使用 actual_model 作为上游模型名
	actualModel := mapping.ActualModel
	if actualModel == "" {
		actualModel = task.ModelId
	}
	imageReq.Model = actualModel

	// 根据 request_endpoint 获取渠道类型列表
	channelTypes, err := channelTypesForImageEndpoint(taskEndpoint)
	if err != nil {
		return "", "", 0, err
	}

	// 选择渠道（根据 channelTypes 过滤）
	channelId, err := selectChannelForModel(actualModel, task.UserId, channelTypes)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to select channel: %w", err)
	}

	// 获取渠道信息
	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get channel: %w", err)
	}

	// 使用 relay 层调用上游 API
	resp, err := callUpstreamImageAPIViaRelay(ctx, ch, task, imageReq)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to call upstream API: %w", err)
	}

	// 解析响应
	imageUrl, metadata, err = parseImageResponse(resp)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse response: %w", err)
	}
	imageUrl = storeImageGenerationResult(ctx, task.Id, imageUrl)

	// 计算费用
	cost = calculateImageCost(task.ModelId, imageReq)

	return imageUrl, metadata, cost, nil
}

// callUpstreamImageAPIViaRelay 通过内部 API 调用 relay 层处理图片生成
func callUpstreamImageAPIViaRelay(ctx context.Context, ch *model.Channel, task *model.ImageGenerationTask, imageReq *dto.ImageRequest) (*http.Response, error) {
	// 获取用户的有效 Token
	userToken, err := getUserValidToken(task.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user token: %w", err)
	}

	// 构建内部 API 请求 URL（通过 relay 层）
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	// 路由规则（按 request_endpoint 区分）：
	//   - "openai"（标准 OpenAI 端点）：
	//     无编辑输入走 /v1/images/generations；
	//     有参考图或遮罩时走 /v1/images/edits。
	//   - "openai-response"（Responses 图像工具）：统一发往 /v1/responses，
	//     由 convertOpenAIResponsesImageRequest 组装 Responses API payload。
	//   - "openai_mod"（魔改端点）：无参考图走 /v1/images/generations；
	//     有参考图走 /v1/images/edits（JSON 透传，由 convertOpenAIModImageEditRequest 处理）。
	//   - 其他端点（gemini 等）：保持原行为。
	taskEndpoint := normalizeImageEndpoint(imageReq.RequestEndpoint)
	requestURL := fmt.Sprintf("http://127.0.0.1:%s/v1/images/generations", port)
	requestPayload := any(imageReq)
	if imageEndpointIsResponses(taskEndpoint) {
		requestURL = fmt.Sprintf("http://127.0.0.1:%s/v1/responses", port)
		requestPayload, err = buildOpenAIResponsesImageRequest(imageReq)
		if err != nil {
			return nil, fmt.Errorf("failed to build responses image request: %w", err)
		}
	} else if (len(imageReq.ReferenceImages) > 0 || strings.TrimSpace(imageReq.Mask) != "") && taskEndpoint == "openai" {
		requestURL = fmt.Sprintf("http://127.0.0.1:%s/v1/images/edits", port)
	} else if len(imageReq.ReferenceImages) > 0 && taskEndpoint != "openai" {
		requestURL = fmt.Sprintf("http://127.0.0.1:%s/v1/images/edits", port)
	}

	// 序列化请求
	jsonData, err := common.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头 - 使用用户的 token 来调用内部 API
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)

	// 发送请求
	client := &http.Client{
		Timeout: imageGenerationTimeout(),
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("upstream API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// parseImageResponse 解析图片响应
func parseImageResponse(resp *http.Response) (imageUrl string, metadata string, err error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(body, &imageResp); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(imageResp.Data) == 0 {
		var responsesResp dto.OpenAIResponsesResponse
		if err := common.Unmarshal(body, &responsesResp); err == nil {
			for _, output := range responsesResp.Output {
				if output.Type != dto.ResponsesOutputTypeImageGenerationCall {
					continue
				}
				if strings.TrimSpace(output.Result) == "" {
					continue
				}
				imageUrl = normalizeOpenAIResponsesImageResult(output.Result)
				metadataMap := make(map[string]any)
				if output.RevisedPrompt != "" {
					metadataMap["revised_prompt"] = output.RevisedPrompt
				}
				if output.Quality != "" {
					metadataMap["quality"] = output.Quality
				}
				if output.Size != "" {
					metadataMap["size"] = output.Size
				}
				if responsesResp.Metadata != nil {
					metadataMap["metadata"] = responsesResp.Metadata
				}
				metadataBytes, _ := common.Marshal(metadataMap)
				metadata = string(metadataBytes)
				return imageUrl, metadata, nil
			}
		}
		return "", "", fmt.Errorf("no image data in response")
	}

	// 获取第一张图片：优先使用上游提供的 url；若只有 b64_json（如 Gemini / OpenAI 的 b64_json 模式），
	// 则拼成 data URL 直接给前端 <img> 渲染。
	first := imageResp.Data[0]
	imageUrl = first.Url
	if imageUrl == "" && first.B64Json != "" {
		imageUrl = "data:image/png;base64," + first.B64Json
	}

	// 构建 metadata
	metadataMap := make(map[string]interface{})
	if first.RevisedPrompt != "" {
		metadataMap["revised_prompt"] = first.RevisedPrompt
	}
	if imageResp.Metadata != nil {
		metadataMap["metadata"] = imageResp.Metadata
	}

	metadataBytes, _ := common.Marshal(metadataMap)
	metadata = string(metadataBytes)

	return imageUrl, metadata, nil
}

// estimateImageGenerationCost 预估图片生成费用
func estimateImageGenerationCost(modelId string) int {
	// 使用模型价格或倍率
	modelPrice, usePrice := ratio_setting.GetModelPrice(modelId, false)
	if usePrice {
		return int(modelPrice * common.QuotaPerUnit)
	}

	modelRatio, success, _ := ratio_setting.GetModelRatio(modelId)
	if !success {
		modelRatio = 1.0
	}

	// 默认预估 1584 tokens（OpenAI 图片生成标准）
	return int(1584 * modelRatio)
}

// calculateImageCost 计算实际图片生成费用
func calculateImageCost(modelId string, imageReq *dto.ImageRequest) int {
	// 获取 token count meta
	meta := imageReq.GetTokenCountMeta()

	// 使用模型价格或倍率
	modelPrice, usePrice := ratio_setting.GetModelPrice(modelId, false)
	if usePrice {
		cost := modelPrice * meta.ImagePriceRatio * common.QuotaPerUnit
		return int(cost)
	}

	modelRatio, success, _ := ratio_setting.GetModelRatio(modelId)
	if !success {
		modelRatio = 1.0
	}

	// 计算费用
	cost := float64(meta.MaxTokens) * modelRatio * meta.ImagePriceRatio

	// 处理 n 参数
	if imageReq.N != nil && *imageReq.N > 1 {
		cost *= float64(*imageReq.N)
	}

	return int(cost)
}

// deductUserQuota 扣除用户额度
func deductUserQuota(userId int, quota int) error {
	return model.DecreaseUserQuota(userId, quota)
}

// channelTypesForImageEndpoint 根据 endpoint 返回对应的渠道类型列表
func channelTypesForImageEndpoint(endpoint string) ([]int, error) {
	switch normalizeImageEndpoint(endpoint) {
	case "openai", "openai-response", "openai_mod":
		return []int{constant.ChannelTypeOpenAI}, nil
	case "gemini":
		return []int{constant.ChannelTypeGemini}, nil
	default:
		return nil, fmt.Errorf("unsupported request_endpoint: %s", endpoint)
	}
}

// selectChannelForModel 为模型选择渠道（根据 request_endpoint 选择正确的渠道类型）
func selectChannelForModel(modelName string, userId int, channelTypes []int) (int, error) {
	// 获取用户组
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return 0, fmt.Errorf("failed to get user: %w", err)
	}
	group := user.Group

	// 使用 Task 1 实现的函数，传入 channelTypes 过滤
	channel, err := model.GetRandomSatisfiedChannelByTypes(group, modelName, 0, channelTypes)
	if err != nil {
		return 0, fmt.Errorf("failed to get channel: %w", err)
	}
	if channel == nil {
		return 0, fmt.Errorf("no enabled channel for model %s with channel types %v", modelName, channelTypes)
	}

	return channel.Id, nil
}

// getStringValue 获取字符串指针的值
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getUserValidToken 获取用户的有效 Token
func getUserValidToken(userId int) (string, error) {
	tokens, err := model.GetAllUserTokens(userId, 0, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get user tokens: %w", err)
	}

	// 查找第一个启用且未过期的 token
	now := time.Now().Unix()
	for _, token := range tokens {
		if token.Status == common.TokenStatusEnabled {
			// 检查是否过期（-1 表示永不过期）
			if token.ExpiredTime == -1 || token.ExpiredTime > now {
				return token.Key, nil
			}
		}
	}

	return "", fmt.Errorf("no valid token found for user %d", userId)
}

// CleanupExpiredImageTasks 清理过期的图片任务
func CleanupExpiredImageTasks() error {
	cfg := worker_setting.GetWorkerSetting()

	// 检查是否启用自动清理
	if !cfg.AutoCleanupEnabled {
		return nil
	}

	// 计算过期时间戳
	retentionDays := cfg.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30 // 默认保留30天
	}
	expirationTime := common.GetTimestamp() - int64(retentionDays*24*60*60)

	common.SysLog(fmt.Sprintf("Starting image cleanup: retention_days=%d, expiration_time=%d", retentionDays, expirationTime))

	// 查询过期任务
	var expiredTasks []*model.ImageGenerationTask
	err := model.DB.Where("created_time < ?", expirationTime).Find(&expiredTasks).Error
	if err != nil {
		return fmt.Errorf("failed to query expired tasks: %w", err)
	}

	if len(expiredTasks) == 0 {
		common.SysLog("No expired image tasks to clean up")
		return nil
	}

	common.SysLog(fmt.Sprintf("Found %d expired image tasks to clean up", len(expiredTasks)))

	// 清理每个任务
	successCount := 0
	failCount := 0
	for _, task := range expiredTasks {
		if err := cleanupSingleTask(task, cfg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to cleanup task %d: %v", task.Id, err))
			failCount++
		} else {
			successCount++
		}
	}

	common.SysLog(fmt.Sprintf("Image cleanup completed: success=%d, failed=%d", successCount, failCount))
	return nil
}

// DeleteImageGenerationTaskAssets 删除任务关联的图片资产（结果图 + 参考图）。
func DeleteImageGenerationTaskAssets(task *model.ImageGenerationTask, cfg *worker_setting.WorkerSetting) {
	if task == nil || cfg == nil {
		return
	}

	if task.ImageUrl != "" {
		if err := deleteImageFile(task.ImageUrl, cfg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to delete image file for task %d: %v", task.Id, err))
		}
	}

	if refs, err := collectStoredReferenceImages(task.Params); err == nil {
		for _, ref := range refs {
			if err := deleteImageFile(ref, cfg); err != nil {
				common.SysLog(fmt.Sprintf("Failed to delete reference image for task %d: %v", task.Id, err))
			}
		}
	}
}

// cleanupSingleTask 清理单个任务
func cleanupSingleTask(task *model.ImageGenerationTask, cfg *worker_setting.WorkerSetting) error {
	DeleteImageGenerationTaskAssets(task, cfg)

	// 删除数据库记录
	if err := model.DeleteImageTask(task.Id); err != nil {
		return fmt.Errorf("failed to delete task record: %w", err)
	}

	return nil
}

func DeleteImageGenerationTask(task *model.ImageGenerationTask) error {
	if task == nil {
		return nil
	}
	return cleanupSingleTask(task, worker_setting.GetWorkerSetting())
}

// deleteImageFile 删除图片文件（本地或S3）
func deleteImageFile(imageUrl string, cfg *worker_setting.WorkerSetting) error {
	if isImageGenerationStoredReferenceURL(imageUrl) {
		if objectKey, ok := imageGenerationLocalAssetKeyFromURL(imageUrl); ok {
			return deleteLocalFile(objectKey, cfg)
		}
		if cfg != nil &&
			strings.TrimSpace(cfg.S3Endpoint) != "" &&
			strings.TrimSpace(cfg.S3Bucket) != "" &&
			strings.TrimSpace(cfg.S3AccessKey) != "" &&
			strings.TrimSpace(cfg.S3SecretKey) != "" {
			return deleteS3File(imageUrl, cfg)
		}
	}

	// 如果是外部URL（http/https），不需要删除
	if strings.HasPrefix(imageUrl, "http://") || strings.HasPrefix(imageUrl, "https://") {
		// 外部URL，跳过删除
		return nil
	}

	// 本地文件
	if cfg.StorageType == "local" {
		return deleteLocalFile(imageUrl, cfg)
	}

	return nil
}

// deleteLocalFile 删除本地文件
func deleteLocalFile(filePath string, cfg *worker_setting.WorkerSetting) error {
	fullPath, err := imageGenerationLocalAssetPath(cfg, filePath)
	if err != nil {
		return err
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，视为成功
			return nil
		}
		return fmt.Errorf("failed to remove local file: %w", err)
	}

	return nil
}

// deleteS3File 删除S3文件
func deleteS3File(imageUrl string, cfg *worker_setting.WorkerSetting) error {
	objectKey, ok := imageGenerationS3ObjectKeyFromURL(imageUrl, cfg)
	if !ok {
		return nil
	}

	client := newImageS3Client(cfg)
	_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(strings.TrimSpace(cfg.S3Bucket)),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}

func collectStoredReferenceImages(params string) ([]string, error) {
	references, err := extractImageGenerationReferenceImages(params)
	if err != nil {
		return nil, err
	}
	mask, err := extractImageGenerationMask(params)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mask) != "" {
		references = append(references, mask)
	}
	return references, nil
}

// StartImageCleanupTask 启动图片清理定时任务
func StartImageCleanupTask() {
	cfg := worker_setting.GetWorkerSetting()

	if !cfg.AutoCleanupEnabled {
		common.SysLog("Image auto cleanup is disabled")
		return
	}

	common.SysLog(fmt.Sprintf("Starting image cleanup task: retention_days=%d", cfg.RetentionDays))

	// 使用 time.Ticker 每天执行一次
	ticker := time.NewTicker(24 * time.Hour)

	// 立即执行一次
	go func() {
		if err := CleanupExpiredImageTasks(); err != nil {
			common.SysLog(fmt.Sprintf("Image cleanup error: %v", err))
		}
	}()

	// 定时执行
	go func() {
		for range ticker.C {
			if err := CleanupExpiredImageTasks(); err != nil {
				common.SysLog(fmt.Sprintf("Image cleanup error: %v", err))
			}
		}
	}()
}
