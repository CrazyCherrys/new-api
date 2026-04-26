package service

import (
	"bytes"
	"context"
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
)

var (
	workerPool     chan struct{}
	workerPoolOnce sync.Once
)

func normalizeImageEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "dalle":
		return "openai"
	default:
		return strings.ToLower(strings.TrimSpace(endpoint))
	}
}

// initWorkerPool 初始化 worker pool
func initWorkerPool() {
	workerPoolOnce.Do(func() {
		cfg := worker_setting.GetWorkerSetting()
		maxWorkers := cfg.MaxWorkers
		if maxWorkers <= 0 {
			maxWorkers = 4
		}
		workerPool = make(chan struct{}, maxWorkers)
		common.SysLog(fmt.Sprintf("Image generation worker pool initialized with %d workers", maxWorkers))
	})
}

// CreateImageGenerationTask 创建图片生成任务
func CreateImageGenerationTask(userId int, modelId string, prompt string, requestEndpoint string, params string) (*model.ImageGenerationTask, error) {
	requestEndpoint = normalizeImageEndpoint(requestEndpoint)

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

	// 创建任务记录
	task := &model.ImageGenerationTask{
		UserId:          userId,
		ModelId:         modelId,
		Prompt:          prompt,
		RequestEndpoint: requestEndpoint,
		Status:          model.ImageTaskStatusPending,
		Params:          params,
		Cost:            0, // 实际费用在完成后计算
		CreatedTime:     common.GetTimestamp(),
	}

	if err := task.Insert(); err != nil {
		return nil, fmt.Errorf("failed to insert task: %w", err)
	}

	// 启动异步处理
	go processTaskAsync(task.Id)

	return task, nil
}

// processTaskAsync 异步处理任务
func processTaskAsync(taskId int) {
	initWorkerPool()

	cfg := worker_setting.GetWorkerSetting()
	timeout := time.Duration(cfg.ImageTimeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	// 创建超时上下文，包括等待 worker pool 的时间
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 尝试获取 worker slot，带超时
	select {
	case workerPool <- struct{}{}:
		defer func() { <-workerPool }()
		// 处理任务
		if err := ProcessImageGenerationTask(taskId); err != nil {
			common.SysLog(fmt.Sprintf("Failed to process image generation task %d: %v", taskId, err))
		}
	case <-ctx.Done():
		// 超时：标记任务为失败
		errorMsg := fmt.Sprintf("task timeout: failed to acquire worker slot within %v", timeout)
		if err := model.UpdateImageTaskStatus(taskId, model.ImageTaskStatusFailed, errorMsg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to update task %d status on timeout: %v", taskId, err))
		}
		common.SysLog(fmt.Sprintf("Task %d timed out waiting for worker slot", taskId))
	}
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

	cfg := worker_setting.GetWorkerSetting()
	timeout := time.Duration(cfg.ImageTimeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 重试逻辑
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := time.Duration(cfg.RetryDelay) * time.Second
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second
	}

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
	if refImages, ok := params["reference_images"].([]interface{}); ok && len(refImages) > 0 {
		for _, img := range refImages {
			if s, ok := img.(string); ok && s != "" {
				imageReq.ReferenceImages = append(imageReq.ReferenceImages, s)
			}
		}
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
	mapping, err := model.GetModelMappingByRequestModel(task.ModelId)
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
	// 使用 localhost 调用自己的 /v1/images/generations 端点
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	requestURL := fmt.Sprintf("http://127.0.0.1:%s/v1/images/generations", port)

	// 序列化请求
	jsonData, err := common.Marshal(imageReq)
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
		Timeout: 120 * time.Second,
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
	case "openai", "openai_mod":
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

// cleanupSingleTask 清理单个任务
func cleanupSingleTask(task *model.ImageGenerationTask, cfg *worker_setting.WorkerSetting) error {
	// 删除图片文件
	if task.ImageUrl != "" {
		if err := deleteImageFile(task.ImageUrl, cfg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to delete image file for task %d: %v", task.Id, err))
			// 继续删除数据库记录，即使文件删除失败
		}
	}

	// 删除数据库记录
	if err := model.DeleteImageTask(task.Id); err != nil {
		return fmt.Errorf("failed to delete task record: %w", err)
	}

	return nil
}

// deleteImageFile 删除图片文件（本地或S3）
func deleteImageFile(imageUrl string, cfg *worker_setting.WorkerSetting) error {
	// 如果是外部URL（http/https），不需要删除
	if strings.HasPrefix(imageUrl, "http://") || strings.HasPrefix(imageUrl, "https://") {
		// 检查是否是S3 URL
		if cfg.StorageType == "s3" && strings.Contains(imageUrl, cfg.S3Bucket) {
			return deleteS3File(imageUrl, cfg)
		}
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
	// 构建完整路径
	var fullPath string
	if cfg.LocalStoragePath != "" {
		fullPath = cfg.LocalStoragePath + "/" + filePath
	} else {
		// 使用系统临时目录
		fullPath = os.TempDir() + "/" + filePath
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
	// 从URL中提取对象键
	// 假设URL格式: https://{bucket}.s3.{region}.amazonaws.com/{key}
	// 或: https://{endpoint}/{bucket}/{key}

	var objectKey string

	// 尝试从URL中提取key
	if strings.Contains(imageUrl, cfg.S3Bucket) {
		parts := strings.Split(imageUrl, cfg.S3Bucket+"/")
		if len(parts) > 1 {
			objectKey = parts[1]
		}
	}

	if objectKey == "" {
		return fmt.Errorf("failed to extract S3 object key from URL: %s", imageUrl)
	}

	// 注意：这里需要AWS SDK来删除S3对象
	// 由于项目中已经有AWS相关代码，这里提供接口
	// 实际实现需要导入 github.com/aws/aws-sdk-go-v2/service/s3

	common.SysLog(fmt.Sprintf("S3 file deletion not fully implemented yet: %s", objectKey))
	// TODO: 实现S3删除逻辑
	// 需要使用 AWS SDK v2:
	// 1. 创建 S3 client
	// 2. 调用 DeleteObject

	return nil
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
