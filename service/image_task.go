package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	// Worker 配置
	imageTaskPollInterval       = 2 * time.Second
	imageTaskWorkerCount        = 2
	imageTaskStaleAfter         = 10 * time.Minute
	imageTaskStaleCheckInterval = 30 * time.Second
	imageTaskMaxAttempts        = 3
)

// 重试间隔配置（指数退避）
var imageTaskRetryBackoff = []time.Duration{
	10 * time.Second, // 第1次重试：10秒后
	30 * time.Second, // 第2次重试：30秒后
	2 * time.Minute,  // 第3次重试：2分钟后
}

// 限流错误
var ErrRateLimited = errors.New("rate limit exceeded")
var ErrInvalidModel = errors.New("invalid image model")
var ErrInvalidResolution = errors.New("invalid image resolution")
var ErrInvalidAspectRatio = errors.New("invalid image aspect ratio")
var ErrInvalidImageCount = errors.New("invalid image count")

// 内存限流器（Redis 不可用时使用）
var imageRateLimiter = &common.InMemoryRateLimiter{}

func init() {
	// 初始化内存限流器，过期时间设为 2 分钟
	imageRateLimiter.Init(2 * time.Minute)
}

// ImageTaskService 图像生成任务服务
// 负责 Worker 轮询、任务调度、重试机制
type ImageTaskService struct {
	imageGenService *ImageGenerationService
	startOnce       sync.Once
	stopCh          chan struct{}
	maxAttempts     int
	mu              sync.RWMutex
}

// NewImageTaskService 创建图像任务服务
func NewImageTaskService(imageGenService *ImageGenerationService) *ImageTaskService {
	return &ImageTaskService{
		imageGenService: imageGenService,
		stopCh:          make(chan struct{}),
		maxAttempts:     imageTaskMaxAttempts,
	}
}

// Start 启动 Worker 进程
func (s *ImageTaskService) Start() {
	s.startOnce.Do(func() {
		// 启动时重置所有 running 状态的任务为 pending（处理进程重启的情况）
		if err := model.ResetStaleImageTasks(0); err != nil {
			log.Printf("[ImageTask] Failed to reset stale tasks on startup: %v", err)
		}

		// 启动僵尸任务检测循环
		go s.staleResetLoop()

		// 启动多个 Worker 进程
		for i := 0; i < imageTaskWorkerCount; i++ {
			go s.workerLoop(i)
		}

		log.Printf("[ImageTask] Started %d workers", imageTaskWorkerCount)
	})
}

// Stop 停止 Worker 进程
func (s *ImageTaskService) Stop() {
	close(s.stopCh)
}

// checkRpmLimit 检查 RPM 限流
func checkRpmLimit(userID int, modelID string, rpmLimit int) error {
	if rpmLimit <= 0 {
		return nil // 限流未启用
	}

	// 限流 key: img_rpm:{userID}:{modelID}
	key := fmt.Sprintf("img_rpm:%d:%s", userID, modelID)

	// 尝试使用 Redis（如果可用）
	if common.RedisEnabled {
		// Redis 限流逻辑
		success, err := common.RedisRateLimitRequest(key, rpmLimit, 60)
		if err != nil {
			// Redis 失败，回退到内存限流
			log.Printf("[ImageTask] Redis rate limit failed, fallback to memory: %v", err)
		} else {
			if !success {
				return ErrRateLimited
			}
			return nil
		}
	}

	// 使用内存限流器
	if !imageRateLimiter.Request(key, rpmLimit, 60) {
		return ErrRateLimited
	}

	return nil
}

// CreateTask 创建新的图像生成任务
func (s *ImageTaskService) CreateTask(ctx context.Context, userID int, modelID, prompt, resolution, aspectRatio, referenceImage string, count int) (*model.ImageTask, error) {
	cfg := system_setting.GetImageGenerationSetting()
	modelCfg, allowed := resolveImageModelSetting(cfg, modelID)
	if !allowed {
		return nil, fmt.Errorf("%w: %s", ErrInvalidModel, modelID)
	}
	if strings.ToLower(modelCfg.ModelType) != "image" {
		return nil, fmt.Errorf("%w: %s is not an image model", ErrInvalidModel, modelID)
	}

	if resolution == "" {
		resolution = modelCfg.DefaultResolution
	}
	if aspectRatio == "" {
		aspectRatio = modelCfg.DefaultAspectRatio
	}

	if count <= 0 {
		count = 1
	}

	if len(modelCfg.Resolutions) > 0 && resolution != "" && !containsExact(modelCfg.Resolutions, resolution) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidResolution, resolution)
	}
	if len(modelCfg.AspectRatios) > 0 && aspectRatio != "" && !containsExact(modelCfg.AspectRatios, aspectRatio) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAspectRatio, aspectRatio)
	}

	maxImageCount := modelCfg.MaxImageCount
	if maxImageCount <= 0 {
		maxImageCount = 10
	}
	if count > maxImageCount {
		return nil, fmt.Errorf("%w: max=%d", ErrInvalidImageCount, maxImageCount)
	}

	rpmLimit := cfg.RpmLimit
	if modelCfg.RpmEnabled {
		rpmLimit = modelCfg.RpmLimit
	} else if rpmLimit <= 0 && modelCfg.RpmLimit > 0 {
		// 兼容旧配置：当全局 RPM 未设置时，允许模型独立 RPM 生效
		rpmLimit = modelCfg.RpmLimit
	}

	// 限流检查
	if err := checkRpmLimit(userID, modelID, rpmLimit); err != nil {
		return nil, err
	}

	task := &model.ImageTask{
		CreatedAt:      common.GetTimestamp(),
		UpdatedAt:      common.GetTimestamp(),
		UserID:         userID,
		ModelID:        modelID,
		Prompt:         prompt,
		Resolution:     resolution,
		AspectRatio:    aspectRatio,
		ReferenceImage: referenceImage,
		Count:          count,
		Status:         model.ImageTaskStatusPending,
		Attempts:       0,
	}

	if err := model.DB.Create(task).Error; err != nil {
		return nil, fmt.Errorf("create image task: %w", err)
	}

	return task, nil
}

// GetTaskByID 获取任务详情
func (s *ImageTaskService) GetTaskByID(taskID int64, userID int) (*model.ImageTask, error) {
	task, err := model.GetImageTaskByID(taskID)
	if err != nil {
		return nil, err
	}

	// 验证任务所有权
	if task.UserID != userID {
		return nil, errors.New("unauthorized access to task")
	}

	return task, nil
}

// ListTasksByUser 获取用户的任务列表
func (s *ImageTaskService) ListTasksByUser(userID int, page, pageSize int, status, modelID, startTime, endTime string) ([]*model.ImageTask, int64, error) {
	return model.GetImageTasksByUserID(userID, page, pageSize, status, modelID, startTime, endTime)
}

// DeleteTask 删除任务
func (s *ImageTaskService) DeleteTask(taskID int64, userID int) error {
	return model.DeleteImageTask(taskID, userID)
}

// workerLoop Worker 主循环
func (s *ImageTaskService) workerLoop(workerID int) {
	log.Printf("[ImageTask] Worker #%d started", workerID)

	for {
		select {
		case <-s.stopCh:
			log.Printf("[ImageTask] Worker #%d stopped", workerID)
			return
		default:
		}

		// 获取下一个待处理任务
		task, err := model.ClaimNextPendingImageTask()
		if err != nil {
			log.Printf("[ImageTask] Worker #%d: Failed to claim task: %v", workerID, err)
			time.Sleep(imageTaskPollInterval)
			continue
		}

		// 没有待处理任务，等待
		if task == nil {
			time.Sleep(imageTaskPollInterval)
			continue
		}

		// 处理任务
		log.Printf("[ImageTask] Worker #%d: Processing task #%d (attempt %d/%d)",
			workerID, task.ID, task.Attempts, s.getMaxAttempts())
		s.processTask(task)
	}
}

// processTask 处理单个任务
func (s *ImageTaskService) processTask(task *model.ImageTask) {
	if task == nil {
		return
	}

	// Panic 恢复
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("internal error: %v", r)
			log.Printf("[ImageTask] Task #%d panic: %v", task.ID, r)
			completedAt := common.GetTimestamp()
			model.UpdateImageTaskResult(task.ID, model.ImageTaskStatusFailed, nil, errMsg, &completedAt)
		}
	}()

	// 检查重试次数
	maxAttempts := s.getMaxAttempts()
	if task.Attempts > maxAttempts {
		errMsg := fmt.Sprintf("retry limit reached (%d attempts)", task.Attempts)
		log.Printf("[ImageTask] Task #%d: %s", task.ID, errMsg)
		completedAt := common.GetTimestamp()
		model.UpdateImageTaskResult(task.ID, model.ImageTaskStatusFailed, nil, errMsg, &completedAt)
		return
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 调用生图服务
	imageURLs, err := s.imageGenService.Generate(ctx, task)
	if err != nil {
		log.Printf("[ImageTask] Task #%d generation failed: %v", task.ID, err)
		s.handleTaskError(task, err)
		return
	}

	// 成功：更新任务结果
	log.Printf("[ImageTask] Task #%d succeeded, generated %d images", task.ID, len(imageURLs))
	completedAt := common.GetTimestamp()
	if err := model.UpdateImageTaskResult(task.ID, model.ImageTaskStatusSucceeded, imageURLs, "", &completedAt); err != nil {
		log.Printf("[ImageTask] Task #%d: Failed to update result: %v", task.ID, err)
	}
}

// handleTaskError 处理任务错误
func (s *ImageTaskService) handleTaskError(task *model.ImageTask, err error) {
	errMsg := sanitizeError(err)
	maxAttempts := s.getMaxAttempts()

	// 判断是否可重试
	if isRetryableError(err) && task.Attempts < maxAttempts {
		// 计算下次重试时间
		nextAttemptAt := common.GetTimestamp() + int64(pickRetryDelay(task.Attempts).Seconds())

		log.Printf("[ImageTask] Task #%d: Retryable error, next attempt at %d", task.ID, nextAttemptAt)
		if err := model.UpdateImageTaskRetry(task.ID, nextAttemptAt, errMsg); err != nil {
			log.Printf("[ImageTask] Task #%d: Failed to update retry info: %v", task.ID, err)
		}
		return
	}

	// 不可重试或超过重试次数：标记失败
	log.Printf("[ImageTask] Task #%d: Non-retryable error or max attempts reached", task.ID)
	completedAt := common.GetTimestamp()
	if err := model.UpdateImageTaskResult(task.ID, model.ImageTaskStatusFailed, nil, errMsg, &completedAt); err != nil {
		log.Printf("[ImageTask] Task #%d: Failed to update result: %v", task.ID, err)
	}
}

// staleResetLoop 僵尸任务检测循环
func (s *ImageTaskService) staleResetLoop() {
	ticker := time.NewTicker(imageTaskStaleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			staleAfterSeconds := int64(imageTaskStaleAfter.Seconds())
			if err := model.ResetStaleImageTasks(staleAfterSeconds); err != nil {
				log.Printf("[ImageTask] Failed to reset stale tasks: %v", err)
			}
		}
	}
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 上下文超时
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// 网络超时
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// 检查错误消息中的关键词
	errMsg := err.Error()
	retryableKeywords := []string{
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"503",
		"504",
	}

	for _, keyword := range retryableKeywords {
		if contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// pickRetryDelay 选择重试延迟时间
func pickRetryDelay(attempts int) time.Duration {
	if attempts <= 0 {
		return imageTaskRetryBackoff[0]
	}
	if attempts-1 < len(imageTaskRetryBackoff) {
		return imageTaskRetryBackoff[attempts-1]
	}
	return imageTaskRetryBackoff[len(imageTaskRetryBackoff)-1]
}

// sanitizeError 清理错误消息（移除敏感信息）
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	// TODO: 实现敏感信息过滤（如 API key）
	return err.Error()
}

// contains 字符串包含检查（不区分大小写）
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsExact(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func defaultImageModelSetting(cfg *system_setting.ImageGenerationSetting, modelID string) system_setting.ImageGenerationModelSetting {
	defaultResolution := cfg.DefaultResolution
	if defaultResolution == "" {
		defaultResolution = "1024x1024"
	}
	defaultAspectRatio := cfg.DefaultAspectRatio
	if defaultAspectRatio == "" {
		defaultAspectRatio = "1:1"
	}
	maxImageCount := cfg.MaxImageCount
	if maxImageCount <= 0 {
		maxImageCount = 10
	}
	return system_setting.ImageGenerationModelSetting{
		DisplayName:        modelID,
		RequestModelID:     modelID,
		RequestEndpoint:    "openai",
		ModelType:          "image",
		DefaultResolution:  defaultResolution,
		DefaultAspectRatio: defaultAspectRatio,
		Resolutions:        []string{defaultResolution},
		AspectRatios:       []string{defaultAspectRatio},
		Durations:          []string{},
		MaxImageCount:      maxImageCount,
		RpmLimit:           cfg.RpmLimit,
		RpmEnabled:         false,
	}
}

func resolveImageModelSetting(cfg *system_setting.ImageGenerationSetting, modelID string) (system_setting.ImageGenerationModelSetting, bool) {
	if cfg == nil || strings.TrimSpace(modelID) == "" {
		return system_setting.ImageGenerationModelSetting{}, false
	}

	if len(cfg.ModelSettings) > 0 {
		if setting, ok := cfg.ModelSettings[modelID]; ok {
			base := defaultImageModelSetting(cfg, modelID)
			if setting.DisplayName != "" {
				base.DisplayName = setting.DisplayName
			}
			if setting.RequestModelID != "" {
				base.RequestModelID = setting.RequestModelID
			}
			if setting.DefaultResolution != "" {
				base.DefaultResolution = setting.DefaultResolution
			}
			if setting.DefaultAspectRatio != "" {
				base.DefaultAspectRatio = setting.DefaultAspectRatio
			}
			if setting.RequestEndpoint != "" {
				base.RequestEndpoint = setting.RequestEndpoint
			}
			if setting.ModelType != "" {
				base.ModelType = setting.ModelType
			}
			if len(setting.Resolutions) > 0 {
				base.Resolutions = setting.Resolutions
			}
			if len(setting.AspectRatios) > 0 {
				base.AspectRatios = setting.AspectRatios
			}
			if len(setting.Durations) > 0 {
				base.Durations = setting.Durations
			}
			if setting.MaxImageCount > 0 {
				base.MaxImageCount = setting.MaxImageCount
			}
			if setting.RpmLimit > 0 {
				base.RpmLimit = setting.RpmLimit
			}
			base.RpmEnabled = setting.RpmEnabled
			return base, true
		}
		// 配置了独立模型但找不到指定模型时，严格拒绝
		return system_setting.ImageGenerationModelSetting{}, false
	}

	if len(cfg.EnabledModels) > 0 && !containsExact(cfg.EnabledModels, modelID) {
		return system_setting.ImageGenerationModelSetting{}, false
	}

	return defaultImageModelSetting(cfg, modelID), true
}

// getMaxAttempts 获取最大重试次数
func (s *ImageTaskService) getMaxAttempts() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxAttempts
}

// SetMaxAttempts 设置最大重试次数
func (s *ImageTaskService) SetMaxAttempts(attempts int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if attempts >= 0 && attempts <= 10 {
		s.maxAttempts = attempts
	}
}
