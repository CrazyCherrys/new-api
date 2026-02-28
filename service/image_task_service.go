package service

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// ImageTaskService Worker 任务处理服务
type ImageTaskService struct {
	workerCount        int
	pollInterval       time.Duration
	taskTimeout        time.Duration
	maxRetries         int
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	generationService  *ImageGenerationService
	rpmService         *ModelRPMService
}

// NewImageTaskService 创建任务处理服务实例
func NewImageTaskService(genService *ImageGenerationService, rpmService *ModelRPMService) *ImageTaskService {
	// 从配置读取参数，使用默认值
	workerCount := 2
	pollInterval := 2 * time.Second
	taskTimeout := 3 * time.Minute
	maxRetries := 3

	// 尝试从配置读取 worker 数量
	common.OptionMapRWMutex.RLock()
	if workerCountStr, ok := common.OptionMap["ImageWorkerCount"]; ok && workerCountStr != "" {
		if count := common.String2Int(workerCountStr); count > 0 {
			workerCount = count
		}
	}
	// 尝试从配置读取轮询间隔
	if pollIntervalStr, ok := common.OptionMap["ImageWorkerPollInterval"]; ok && pollIntervalStr != "" {
		if duration, err := time.ParseDuration(pollIntervalStr); err == nil && duration > 0 {
			pollInterval = duration
		}
	}
	// 尝试从配置读取任务超时
	if taskTimeoutStr, ok := common.OptionMap["ImageTaskTimeout"]; ok && taskTimeoutStr != "" {
		if duration, err := time.ParseDuration(taskTimeoutStr); err == nil && duration > 0 {
			taskTimeout = duration
		}
	}
	// 尝试从配置读取最大重试次数
	if maxRetriesStr, ok := common.OptionMap["ImageTaskMaxRetries"]; ok && maxRetriesStr != "" {
		if retries := common.String2Int(maxRetriesStr); retries > 0 {
			maxRetries = retries
		}
	}
	common.OptionMapRWMutex.RUnlock()

	return &ImageTaskService{
		workerCount:       workerCount,
		pollInterval:      pollInterval,
		taskTimeout:       taskTimeout,
		maxRetries:        maxRetries,
		stopChan:          make(chan struct{}),
		generationService: genService,
		rpmService:        rpmService,
	}
}

// Start 启动 Worker 服务
func (s *ImageTaskService) Start() {
	common.SysLog(fmt.Sprintf("Starting ImageTaskService with %d workers, poll interval: %v, task timeout: %v, max retries: %d",
		s.workerCount, s.pollInterval, s.taskTimeout, s.maxRetries))

	// 启动时重置僵尸任务
	resetCount, err := model.ResetZombieRunning(s.taskTimeout * 2)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to reset zombie tasks on startup: %v", err))
	} else if resetCount > 0 {
		common.SysLog(fmt.Sprintf("Reset %d zombie tasks on startup", resetCount))
	}

	// 启动僵尸任务检测循环
	s.wg.Add(1)
	go s.zombieDetectorLoop()

	// 启动 worker 协程
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.workerLoop(i + 1)
	}

	common.SysLog("ImageTaskService started successfully")
}

// Stop 停止 Worker 服务
func (s *ImageTaskService) Stop() {
	common.SysLog("Stopping ImageTaskService...")
	close(s.stopChan)
	s.wg.Wait()
	common.SysLog("ImageTaskService stopped")
}

// workerLoop Worker 主循环
func (s *ImageTaskService) workerLoop(workerID int) {
	defer s.wg.Done()

	common.SysLog(fmt.Sprintf("Worker #%d started", workerID))

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			common.SysLog(fmt.Sprintf("Worker #%d stopping", workerID))
			return
		case <-ticker.C:
			// 尝试获取下一个待处理任务
			task, err := model.ClaimNextPending()
			if err != nil {
				common.SysLog(fmt.Sprintf("Worker #%d: failed to claim task: %v", workerID, err))
				continue
			}

			if task == nil {
				// 没有待处理任务，继续等待
				continue
			}

			// 处理任务
			common.SysLog(fmt.Sprintf("Worker #%d: processing task %s (attempt %d)", workerID, task.ID, task.Attempts+1))
			s.processTask(task)
		}
	}
}

// processTask 处理单个任务
func (s *ImageTaskService) processTask(task *model.ImageGenerationTask) {
	// Panic 恢复机制
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic during task processing: %v", r)
			common.SysLog(fmt.Sprintf("Task %s: %s", task.ID, errMsg))

			// 尝试标记任务失败
			if err := task.MarkFailed(task.ID, errMsg); err != nil {
				common.SysLog(fmt.Sprintf("Task %s: failed to mark as failed after panic: %v", task.ID, err))
			}
		}
	}()

	// 检查重试次数
	if task.Attempts >= s.maxRetries {
		errMsg := fmt.Sprintf("max retries (%d) exceeded", s.maxRetries)
		common.SysLog(fmt.Sprintf("Task %s: %s", task.ID, errMsg))
		if err := task.MarkFailed(task.ID, errMsg); err != nil {
			common.SysLog(fmt.Sprintf("Task %s: failed to mark as failed: %v", task.ID, err))
		}
		return
	}

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), s.taskTimeout)
	defer cancel()

	// 调用生成服务
	imageURLs, err := s.generationService.Generate(ctx, task)

	if err != nil {
		// 处理失败
		errMsg := err.Error()
		common.SysLog(fmt.Sprintf("Task %s: generation failed: %v", task.ID, err))

		// 判断是否可重试
		if s.isRetryableImageError(err) && task.Attempts+1 < s.maxRetries {
			// 计算下次重试时间
			delay := s.pickImageTaskRetryDelay(task.Attempts + 1)
			nextAttemptAt := time.Now().Add(delay)

			common.SysLog(fmt.Sprintf("Task %s: scheduling retry in %v (attempt %d/%d)",
				task.ID, delay, task.Attempts+2, s.maxRetries))

			if err := task.ScheduleRetry(task.ID, nextAttemptAt, errMsg); err != nil {
				common.SysLog(fmt.Sprintf("Task %s: failed to schedule retry: %v", task.ID, err))
			}
		} else {
			// 不可重试或已达最大重试次数
			common.SysLog(fmt.Sprintf("Task %s: marking as failed (non-retryable or max retries reached)", task.ID))
			if err := task.MarkFailed(task.ID, errMsg); err != nil {
				common.SysLog(fmt.Sprintf("Task %s: failed to mark as failed: %v", task.ID, err))
			}
		}
		return
	}

	// 成功
	common.SysLog(fmt.Sprintf("Task %s: generation succeeded, %d images generated", task.ID, len(imageURLs)))
	if err := task.MarkSuccess(task.ID, imageURLs); err != nil {
		common.SysLog(fmt.Sprintf("Task %s: failed to mark as succeeded: %v", task.ID, err))
	}
}

// isRetryableImageError 判断错误是否可重试
func (s *ImageTaskService) isRetryableImageError(err error) bool {
	if err == nil {
		return false
	}

	// 检查超时错误
	if err == context.DeadlineExceeded {
		return true
	}

	// 检查网络超时错误
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// 检查 HTTP 状态码（从错误消息中提取）
	errMsg := err.Error()

	// 可重试的 HTTP 状态码
	retryableStatusCodes := []string{
		"status 429", // Too Many Requests
		"status 502", // Bad Gateway
		"status 503", // Service Unavailable
		"status 504", // Gateway Timeout
		"status 408", // Request Timeout
	}

	for _, statusCode := range retryableStatusCodes {
		if contains(errMsg, statusCode) {
			return true
		}
	}

	// 可重试的错误关键词
	retryableKeywords := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"temporary failure",
		"rate limit",
		"too many requests",
	}

	for _, keyword := range retryableKeywords {
		if contains(errMsg, keyword) {
			return true
		}
	}

	// 默认不重试
	return false
}

// pickImageTaskRetryDelay 计算重试延迟（带抖动）
func (s *ImageTaskService) pickImageTaskRetryDelay(attempts int) time.Duration {
	var baseDelay time.Duration

	switch attempts {
	case 0:
		baseDelay = 10 * time.Second
	case 1:
		baseDelay = 30 * time.Second
	default:
		baseDelay = 2 * time.Minute
	}

	// 添加 ±20% 的随机抖动
	jitter := float64(baseDelay) * 0.2
	jitterAmount := time.Duration(rand.Float64()*jitter*2 - jitter)

	return baseDelay + jitterAmount
}

// zombieDetectorLoop 僵尸任务检测循环
func (s *ImageTaskService) zombieDetectorLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			// 重置超时的运行中任务
			resetCount, err := model.ResetZombieRunning(s.taskTimeout * 2)
			if err != nil {
				common.SysLog(fmt.Sprintf("Zombie detector: failed to reset zombie tasks: %v", err))
			} else if resetCount > 0 {
				common.SysLog(fmt.Sprintf("Zombie detector: reset %d zombie tasks", resetCount))
			}
		}
	}
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && containsIgnoreCase(s, substr))
}

// containsIgnoreCase 不区分大小写的字符串包含检查
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// toLower 转换为小写
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
