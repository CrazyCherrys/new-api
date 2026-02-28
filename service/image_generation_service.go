package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// ImageGenerationService 图像生成服务
type ImageGenerationService struct {
	storageService *ImageStorageService
}

// NewImageGenerationService 创建图像生成服务实例
func NewImageGenerationService(storageService *ImageStorageService) *ImageGenerationService {
	return &ImageGenerationService{
		storageService: storageService,
	}
}

// Generate 生成图片
func (s *ImageGenerationService) Generate(ctx context.Context, task *model.ImageGenerationTask) ([]string, error) {
	// 1. 参数验证
	if task.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if task.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	if task.Count <= 0 {
		task.Count = 1
	}

	// 2. 设置超时（从配置读取，默认3分钟）
	timeout := 3 * time.Minute
	common.OptionMapRWMutex.RLock()
	if timeoutStr, ok := common.OptionMap["ImageGenerationTimeout"]; ok && timeoutStr != "" {
		if duration, err := time.ParseDuration(timeoutStr); err == nil && duration > 0 {
			timeout = duration
		}
	}
	common.OptionMapRWMutex.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. 渠道选择 - 通过 model 和 group 查询可用渠道
	group := "default"
	if task.UserID > 0 {
		// 可以根据用户获取其所属的 group，这里简化处理使用 default
		// 实际项目中可能需要从 user 表查询 group
	}

	// 获取支持该模型的渠道
	channel, err := s.selectChannel(task.ModelID, group)
	if err != nil {
		return nil, fmt.Errorf("select channel failed: %w", err)
	}

	// 4. 根据渠道类型调用对应的生成方法
	var imageURLs []string
	switch channel.Type {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeCustom:
		imageURLs, err = s.generateOpenAI(ctx, channel, task)
	case constant.ChannelTypeGemini, constant.ChannelTypeVertexAi:
		imageURLs, err = s.generateGemini(ctx, channel, task)
	default:
		// 默认尝试 OpenAI 格式
		imageURLs, err = s.generateOpenAI(ctx, channel, task)
	}

	if err != nil {
		return nil, err
	}

	// 5. 存储图片
	if s.storageService != nil {
		storedURLs, storeErr := s.storageService.StoreGeneratedImages(imageURLs)
		if storeErr != nil {
			common.SysLog(fmt.Sprintf("Failed to store images: %v", storeErr))
			// 存储失败不影响返回原始 URL
		} else {
			imageURLs = storedURLs
		}
	}

	return imageURLs, nil
}

// selectChannel 选择可用渠道
func (s *ImageGenerationService) selectChannel(modelID string, group string) (*model.Channel, error) {
	// 使用现有的 ability 系统查询支持该模型的渠道
	var ability model.Ability

	// 构建查询条件，使用正确的列名处理
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	err := model.DB.Where(groupCol+" = ? AND model = ? AND enabled = ?", group, modelID, true).
		Order("priority DESC, weight DESC").
		First(&ability).Error
	if err != nil {
		return nil, fmt.Errorf("no available channel for model %s: %w", modelID, err)
	}

	// 获取渠道详情
	channel, err := model.GetChannelById(ability.ChannelId, true)
	if err != nil {
		return nil, fmt.Errorf("get channel failed: %w", err)
	}

	if channel.Status != common.ChannelStatusEnabled {
		return nil, fmt.Errorf("channel %d is not enabled", channel.Id)
	}

	return channel, nil
}

// generateOpenAI 使用 OpenAI 格式生成图片
func (s *ImageGenerationService) generateOpenAI(ctx context.Context, channel *model.Channel, task *model.ImageGenerationTask) ([]string, error) {
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[channel.Type]
	}

	// 获取 API Key
	apiKey, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return nil, apiErr.Err
	}

	// 构建请求
	var endpoint string
	var requestBody map[string]interface{}

	if task.ReferenceImage != "" {
		// 图生图：使用 /v1/images/edits
		endpoint = fmt.Sprintf("%s/v1/images/edits", strings.TrimRight(baseURL, "/"))
		requestBody = map[string]interface{}{
			"image":  task.ReferenceImage,
			"prompt": task.Prompt,
			"n":      task.Count,
		}
		if task.Resolution != "" {
			requestBody["size"] = task.Resolution
		}
	} else {
		// 文生图：使用 /v1/images/generations
		endpoint = fmt.Sprintf("%s/v1/images/generations", strings.TrimRight(baseURL, "/"))
		requestBody = map[string]interface{}{
			"prompt": task.Prompt,
			"model":  task.ModelID,
			"n":      task.Count,
		}
		if task.Resolution != "" {
			requestBody["size"] = task.Resolution
		}
		if task.AspectRatio != "" {
			requestBody["aspect_ratio"] = task.AspectRatio
		}
	}

	// 序列化请求体
	bodyBytes, err := common.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body failed: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// 发送请求
	client := GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var response struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("api error: %s", response.Error.Message)
	}

	// 提取图片 URL
	var imageURLs []string
	for _, item := range response.Data {
		if item.URL != "" {
			imageURLs = append(imageURLs, item.URL)
		} else if item.B64JSON != "" {
			// 将 base64 转换为 data URL
			imageURLs = append(imageURLs, fmt.Sprintf("data:image/png;base64,%s", item.B64JSON))
		}
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("no images returned")
	}

	return imageURLs, nil
}

// generateGemini 使用 Gemini 格式生成图片
func (s *ImageGenerationService) generateGemini(ctx context.Context, channel *model.Channel, task *model.ImageGenerationTask) ([]string, error) {
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[channel.Type]
	}

	// 获取 API Key
	apiKey, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		return nil, apiErr.Err
	}

	// 构建请求 URL
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", strings.TrimRight(baseURL, "/"), task.ModelID)

	// 构建请求体
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": task.Prompt,
					},
				},
			},
		},
	}

	// 添加图像配置
	if task.Resolution != "" || task.AspectRatio != "" || task.Count > 1 {
		imageConfig := make(map[string]interface{})
		if task.Resolution != "" {
			imageConfig["resolution"] = task.Resolution
		}
		if task.AspectRatio != "" {
			imageConfig["aspectRatio"] = task.AspectRatio
		}
		if task.Count > 1 {
			imageConfig["numberOfImages"] = task.Count
		}
		requestBody["generationConfig"] = map[string]interface{}{
			"imageConfig": imageConfig,
		}
	}

	// 如果有参考图片，添加到 parts
	if task.ReferenceImage != "" {
		parts := requestBody["contents"].([]map[string]interface{})[0]["parts"].([]map[string]interface{})

		// 判断是 URL 还是 base64
		if strings.HasPrefix(task.ReferenceImage, "http://") || strings.HasPrefix(task.ReferenceImage, "https://") {
			parts = append(parts, map[string]interface{}{
				"fileData": map[string]interface{}{
					"fileUri": task.ReferenceImage,
				},
			})
		} else {
			// 处理 base64 或 data URL
			b64Data := task.ReferenceImage
			mimeType := "image/png"

			if strings.HasPrefix(b64Data, "data:") {
				// 解析 data URL
				parts := strings.SplitN(b64Data, ",", 2)
				if len(parts) == 2 {
					if strings.Contains(parts[0], ";") {
						mimePart := strings.Split(parts[0], ";")[0]
						if strings.HasPrefix(mimePart, "data:") {
							mimeType = strings.TrimPrefix(mimePart, "data:")
						}
					}
					b64Data = parts[1]
				}
			}

			parts = append(parts, map[string]interface{}{
				"inlineData": map[string]interface{}{
					"mimeType": mimeType,
					"data":     b64Data,
				},
			})
		}
		requestBody["contents"].([]map[string]interface{})[0]["parts"] = parts
	}

	// 序列化请求体
	bodyBytes, err := common.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body failed: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	// 发送请求
	client := GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
					FileData *struct {
						FileURI  string `json:"fileUri"`
						MimeType string `json:"mimeType"`
					} `json:"fileData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("api error: %s", response.Error.Message)
	}

	// 提取图片
	var imageURLs []string
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				// base64 图片
				mimeType := part.InlineData.MimeType
				if mimeType == "" {
					mimeType = "image/png"
				}
				dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, part.InlineData.Data)
				imageURLs = append(imageURLs, dataURL)
			} else if part.FileData != nil && part.FileData.FileURI != "" {
				// 文件 URI
				imageURLs = append(imageURLs, part.FileData.FileURI)
			}
		}
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("no images returned")
	}

	return imageURLs, nil
}

// classifyImageError 分类错误，判断是否可重试
func (s *ImageGenerationService) classifyImageError(err error, statusCode int) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// 可重试的错误
	retryableErrors := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"temporary failure",
		"rate limit",
		"too many requests",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errMsg, retryable) {
			return true
		}
	}

	// 根据状态码判断
	switch statusCode {
	case 429: // Too Many Requests
		return true
	case 502, 503, 504: // Bad Gateway, Service Unavailable, Gateway Timeout
		return true
	case 408: // Request Timeout
		return true
	}

	// 不可重试的错误
	nonRetryableErrors := []string{
		"invalid",
		"bad request",
		"unauthorized",
		"forbidden",
		"not found",
		"insufficient",
		"quota exceeded",
		"balance",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if strings.Contains(errMsg, nonRetryable) {
			return false
		}
	}

	// 默认不重试
	return false
}

// ClassifyImageError 公开的错误分类方法
func (s *ImageGenerationService) ClassifyImageError(err error, statusCode int) bool {
	return s.classifyImageError(err, statusCode)
}
