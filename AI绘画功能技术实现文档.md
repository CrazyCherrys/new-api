# AI绘画功能技术实现文档

## 目录

1. [系统架构概览](#系统架构概览)
2. [核心功能：异步生图机制](#核心功能异步生图机制)
3. [前端实现](#前端实现)
4. [后端实现](#后端实现)
5. [存储系统](#存储系统)
6. [配置管理](#配置管理)
7. [模型管理与限流](#模型管理与限流)
8. [数据库设计](#数据库设计)
9. [关键代码示例](#关键代码示例)
10. [部署与配置](#部署与配置)

---

## 系统架构概览

### 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                          前端 (Vue.js)                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  HomeView    │  │  History     │  │  Gallery     │          │
│  │  生图界面     │  │  历史记录     │  │  图库管理     │          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
│         │                 │                 │                   │
│         └─────────────────┴─────────────────┘                   │
│                           │                                     │
│                    API Client (Axios)                           │
└───────────────────────────┼─────────────────────────────────────┘
                            │ HTTP/REST
┌───────────────────────────┼─────────────────────────────────────┐
│                    后端 (Go/Gin)                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              ImageGenerationHandler                      │   │
│  │  - POST /api/v1/images/generate (创建任务)               │   │
│  │  - GET  /api/v1/images/history (查询历史)                │   │
│  │  - GET  /api/v1/images/history/:id (任务详情)            │   │
│  └────────────────┬────────────────────────────────────────┘   │
│                   │                                             │
│  ┌────────────────┴────────────────┬──────────────────────┐   │
│  │                                 │                       │   │
│  │  ImageTaskService               │  ImageGenerationService│   │
│  │  (异步任务管理)                  │  (实际生图逻辑)         │   │
│  │  - 创建任务                      │  - 调用AI模型          │   │
│  │  - 后台Worker轮询                │  - 多平台适配          │   │
│  │  - 重试机制                      │  - 参数处理            │   │
│  │  - 超时控制                      │                       │   │
│  └────────┬────────────────────────┴───────┬───────────────┘   │
│           │                                │                   │
│  ┌────────┴────────┐              ┌────────┴────────┐          │
│  │ StorageService  │              │ ModelRPMService │          │
│  │ (存储管理)       │              │ (模型限流)       │          │
│  │ - 本地存储       │              │ - RPM限制        │          │
│  │ - S3存储         │              │ - 等待队列       │          │
│  └─────────────────┘              └─────────────────┘          │
│           │                                                     │
└───────────┼─────────────────────────────────────────────────────┘
            │
┌───────────┼─────────────────────────────────────────────────────┐
│           │              数据层                                  │
│  ┌────────┴────────┐              ┌──────────────────┐          │
│  │   PostgreSQL    │              │  Redis (缓存)     │          │
│  │ - 任务表         │              │ - RPM计数器       │          │
│  │ - 用户表         │              │ - 会话缓存        │          │
│  │ - 配置表         │              │                  │          │
│  └─────────────────┘              └──────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
            │
┌───────────┼─────────────────────────────────────────────────────┐
│           │              存储层                                  │
│  ┌────────┴────────┐              ┌──────────────────┐          │
│  │  本地文件系统    │              │  S3兼容存储       │          │
│  │  data/uploads/  │              │  (可选)           │          │
│  └─────────────────┘              └──────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

### 核心特性

1. **异步任务机制**：用户提交请求后立即返回任务ID，可关闭网页
2. **后台自动处理**：Worker进程持续轮询待处理任务
3. **智能重试**：失败任务自动重试，支持指数退避
4. **灵活存储**：支持本地文件系统和S3兼容对象存储
5. **模型限流**：单模型RPM限制，防止滥用
6. **多平台支持**：OpenAI、Gemini、自定义端点

---

## 核心功能：异步生图机制

### 工作流程

```
用户操作                后端处理                    Worker进程
    │                      │                          │
    │  1. 提交生图请求      │                          │
    ├──────────────────────>│                          │
    │                      │                          │
    │                      │ 2. 创建任务记录           │
    │                      │    status: pending       │
    │                      │                          │
    │  3. 返回任务ID        │                          │
    │<──────────────────────┤                          │
    │                      │                          │
    │  (用户可关闭网页)      │                          │
    │                      │                          │
    │                      │  4. Worker轮询            │
    │                      │<─────────────────────────┤
    │                      │                          │
    │                      │  5. 获取pending任务       │
    │                      │  更新status: running     │
    │                      ├─────────────────────────>│
    │                      │                          │
    │                      │                          │ 6. 调用AI模型
    │                      │                          │    生成图片
    │                      │                          │
    │                      │  7. 下载图片              │
    │                      │<─────────────────────────┤
    │                      │                          │
    │                      │  8. 存储到本地/S3         │
    │                      │                          │
    │                      │  9. 更新任务状态          │
    │                      │    status: succeeded     │
    │                      │    image_urls: [...]     │
    │                      │<─────────────────────────┤
    │                      │                          │
    │  10. 轮询查询任务状态  │                          │
    ├──────────────────────>│                          │
    │                      │                          │
    │  11. 返回完成结果      │                          │
    │<──────────────────────┤                          │
    │                      │                          │
```

### 关键设计点

#### 1. 任务状态机

```go
const (
    ImageTaskStatusPending   = "pending"    // 等待处理
    ImageTaskStatusRunning   = "running"    // 正在处理
    ImageTaskStatusSucceeded = "succeeded"  // 成功完成
    ImageTaskStatusFailed    = "failed"     // 失败
)
```

#### 2. Worker配置

```go
const (
    imageTaskPollInterval       = 2 * time.Second   // 轮询间隔
    imageTaskWorkerCount        = 2                 // Worker数量
    imageTaskTimeout            = 5 * time.Minute   // 单任务超时
    imageTaskStaleAfter         = 10 * time.Minute  // 僵尸任务判定
    imageTaskStaleCheckInterval = 30 * time.Second  // 僵尸任务检查间隔
    imageTaskMaxAttempts        = 3                 // 最大重试次数
)
```

#### 3. 重试策略

```go
var imageTaskRetryBackoff = []time.Duration{
    10 * time.Second,  // 第1次重试：10秒后
    30 * time.Second,  // 第2次重试：30秒后
    2 * time.Minute,   // 第3次重试：2分钟后
}
```

**可重试的错误类型：**
- 网络超时 (`context.DeadlineExceeded`)
- 临时网络错误 (`net.Error` with `Timeout()`)
- 网关超时 (`504 Gateway Timeout`)

---

## 前端实现

### 文件位置
- 主界面：`frontend/src/views/HomeView.vue`
- API客户端：`frontend/src/api/images.ts`
- 类型定义：`frontend/src/types/index.ts`

### 生图请求流程

#### 1. 用户提交请求

```typescript
// frontend/src/views/HomeView.vue:358-402
async function handleGenerate() {
  generating.value = true
  const requestModelId = resolveRequestModelId(selectedModel.value)

  try {
    // 构建请求参数
    const payload = {
      model_id: requestModelId,
      prompt: prompt.value.trim(),
      resolution: selectedResolution.value,
      aspect_ratio: selectedRatio.value,
      reference_image: referenceImagesBase64[0] || undefined,
      async: true  // 关键：异步模式
    }

    // 批量创建任务
    const count = normalizeImageCount(imageCount.value)
    const results = await Promise.allSettled(
      Array.from({ length: count }, () => imagesAPI.createImageTask(payload))
    )

    // 处理结果
    const succeeded = results.filter(r => r.status === 'fulfilled')
    if (succeeded.length > 0) {
      // 切换到历史记录标签
      activeTab.value = 'history'
      await loadHistory({ silent: true })
      appStore.showSuccess(t('home.generator.generateSuccess'))
    }
  } catch (error) {
    appStore.showError(error.message)
  } finally {
    generating.value = false
  }
}
```

#### 2. API客户端

```typescript
// frontend/src/api/images.ts:31-38
export async function createImageTask(
  payload: ImageGeneratePayload
): Promise<ImageGenerationTask> {
  const { data } = await apiClient.post<ImageGenerationTask>(
    '/images/generate',
    { ...payload, async: true },  // 强制异步模式
    { timeout: 120000 }            // 2分钟超时
  )
  return data
}
```

#### 3. 历史记录轮询

```typescript
// 前端通过定时轮询获取任务状态
async function loadHistory({ silent = false } = {}) {
  try {
    const response = await imagesAPI.listImageHistory({
      page: currentPage.value,
      page_size: pageSize.value,
      status: filterStatus.value,
      model: filterModel.value
    })

    historyTasks.value = response.items
    totalTasks.value = response.total

    // 如果有pending/running任务，继续轮询
    const hasActiveTask = response.items.some(
      task => task.status === 'pending' || task.status === 'running'
    )
    if (hasActiveTask) {
      setTimeout(() => loadHistory({ silent: true }), 3000)
    }
  } catch (error) {
    if (!silent) {
      appStore.showError(error.message)
    }
  }
}
```


### 请求参数说明

```typescript
interface ImageGeneratePayload {
  model_id: string          // 模型ID（必填）
  prompt: string            // 提示词（必填）
  resolution?: string       // 分辨率：1K/2K/4K
  aspect_ratio?: string     // 宽高比：1:1/16:9/9:16/3:4/4:3
  reference_image?: string  // 参考图片（base64）
  count?: number            // 生成数量（1-4）
  async?: boolean           // 是否异步（默认true）
}
```

---

## 后端实现

### 文件结构

```
backend/
├── internal/
│   ├── handler/
│   │   ├── image_generation_handler.go    # HTTP处理器
│   │   └── model_rpm_helper.go            # RPM限流辅助
│   ├── service/
│   │   ├── image_generation_service.go    # 生图服务
│   │   ├── image_task_service.go          # 任务管理服务
│   │   ├── storage_service.go             # 存储服务
│   │   └── model_rpm_service.go           # 模型限流服务
│   └── repository/
│       └── image_task_repo.go             # 数据访问层
└── migrations/
    ├── 047_image_generation_tasks.sql     # 任务表
    └── 050_image_generation_tasks_retry.sql # 重试字段
```

### 1. HTTP处理器

**文件：** `backend/internal/handler/image_generation_handler.go`

#### 创建生图任务

```go
// POST /api/v1/images/generate
func (h *ImageGenerationHandler) Generate(c *gin.Context) {
    subject, ok := middleware.GetAuthSubjectFromContext(c)
    if !ok || subject.UserID <= 0 {
        response.Unauthorized(c, "Unauthorized")
        return
    }

    var req ImageGenerateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request: "+err.Error())
        return
    }

    // 1. 检查模型RPM限制
    if err := h.modelRPMHelper.WaitForModelRPM(
        c, subject.UserID, req.ModelID, false, nil
    ); err != nil {
        if _, ok := err.(*ModelRPMError); ok {
            response.Error(c, http.StatusTooManyRequests, 
                "Model RPM limit reached, please retry later")
        } else {
            response.InternalError(c, "Failed to apply model RPM limit")
        }
        return
    }

    input := service.ImageGenerationInput{
        UserID:         subject.UserID,
        ModelID:        req.ModelID,
        Prompt:         req.Prompt,
        Resolution:     req.Resolution,
        AspectRatio:    req.AspectRatio,
        ReferenceImage: req.ReferenceImage,
        Count:          req.Count,
    }

    // 2. 异步模式：创建任务并立即返回
    if req.Async {
        task, err := h.taskService.CreateTask(c.Request.Context(), input)
        if err != nil {
            response.ErrorFrom(c, err)
            return
        }
        response.Success(c, dto.ImageGenerationTaskFromService(task))
        return
    }

    // 3. 同步模式：等待生成完成（不推荐）
    result, err := h.imageService.Generate(c.Request.Context(), input)
    if err != nil {
        response.ErrorFrom(c, err)
        return
    }

    response.Success(c, result)
}
```

#### 查询历史记录

```go
// GET /api/v1/images/history
func (h *ImageGenerationHandler) ListHistory(c *gin.Context) {
    subject, ok := middleware.GetAuthSubjectFromContext(c)
    if !ok || subject.UserID <= 0 {
        response.Unauthorized(c, "Unauthorized")
        return
    }

    // 解析分页参数
    page, pageSize := response.ParsePagination(c)
    params := pagination.PaginationParams{Page: page, PageSize: pageSize}

    // 解析过滤条件
    filters, err := parseImageHistoryFilters(c)
    if err != nil {
        response.BadRequest(c, "Invalid filters: "+err.Error())
        return
    }

    // 查询任务列表
    tasks, result, err := h.taskService.ListByUser(
        c.Request.Context(), subject.UserID, params, filters
    )
    if err != nil {
        response.ErrorFrom(c, err)
        return
    }

    // 构建响应
    out := make([]dto.ImageGenerationTask, 0, len(tasks))
    for i := range tasks {
        if mapped := dto.ImageGenerationTaskFromService(&tasks[i]); mapped != nil {
            out = append(out, *mapped)
        }
    }

    response.Paginated(c, out, result.Total, result.Page, result.PageSize)
}
```

### 2. 任务管理服务

**文件：** `backend/internal/service/image_task_service.go`

#### Worker启动

```go
func (s *ImageTaskService) Start() {
    s.startOnce.Do(func() {
        // 1. 启动时重置所有running状态的任务为pending
        if err := s.repo.ResetStaleRunning(
            context.Background(), 
            ImageTaskStatusRunning, 
            ImageTaskStatusPending, 
            time.Now()
        ); err != nil {
            log.Printf("image tasks: reset stale running failed: %v", err)
        }

        // 2. 启动僵尸任务检测循环
        go s.staleResetLoop()

        // 3. 启动配置刷新循环
        go s.configRefreshLoop()

        // 4. 启动多个Worker进程
        for i := 0; i < imageTaskWorkerCount; i++ {
            go s.workerLoop()
        }
    })
}
```

#### Worker主循环

```go
func (s *ImageTaskService) workerLoop() {
    for {
        select {
        case <-s.stopCh:
            return
        default:
        }

        // 1. 从数据库获取下一个待处理任务（原子操作）
        task, err := s.repo.ClaimNextPending(
            context.Background(), 
            ImageTaskStatusPending, 
            ImageTaskStatusRunning
        )
        if err != nil {
            log.Printf("image tasks: claim pending failed: %v", err)
            time.Sleep(imageTaskPollInterval)
            continue
        }

        // 2. 没有任务时等待
        if task == nil {
            time.Sleep(imageTaskPollInterval)
            continue
        }

        // 3. 处理任务
        s.processTask(task)
    }
}
```

#### 任务处理逻辑

```go
func (s *ImageTaskService) processTask(task *ImageGenerationTask) {
    if task == nil {
        return
    }

    // 1. Panic恢复
    defer func() {
        if r := recover(); r != nil {
            message := fmt.Sprintf("internal error: %v", r)
            s.repo.UpdateResult(
                context.Background(), 
                task.ID, 
                ImageTaskStatusFailed, 
                nil, 
                &message, 
                timePtr(time.Now())
            )
        }
    }()

    // 2. 检查重试次数
    maxAttempts := s.getMaxAttempts()
    if task.Attempts > maxAttempts {
        message := "retry limit reached"
        s.repo.UpdateResult(
            context.Background(), 
            task.ID, 
            ImageTaskStatusFailed, 
            nil, 
            &message, 
            timePtr(time.Now())
        )
        return
    }

    // 3. 获取超时配置
    timeoutSettings, err := s.settingService.GetGenerationTimeoutSettings(
        context.Background()
    )
    if err != nil {
        log.Printf("Failed to get timeout settings, using default: %v", err)
        timeoutSettings = DefaultGenerationTimeoutSettings()
    }

    // 4. 创建带超时的上下文
    timeout := time.Duration(timeoutSettings.ImageTimeoutSeconds) * time.Second
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // 5. 调用生图服务
    result, err := s.imageService.Generate(ctx, ImageGenerationInput{
        UserID:         task.UserID,
        ModelID:        task.ModelID,
        Prompt:         task.Prompt,
        Resolution:     task.Resolution,
        AspectRatio:    task.AspectRatio,
        ReferenceImage: task.ReferenceImage,
        Count:          task.Count,
    })

    // 6. 处理错误
    if err != nil {
        message := sanitizeTaskError(err)
        
        // 6.1 可重试错误：更新重试信息
        if isRetryableImageError(err) && task.Attempts < maxAttempts {
            nextAttemptAt := time.Now().Add(pickImageTaskRetryDelay(task.Attempts))
            s.repo.UpdateRetry(
                context.Background(), 
                task.ID, 
                ImageTaskStatusPending, 
                nextAttemptAt, 
                &message
            )
            return
        }
        
        // 6.2 不可重试错误：标记失败
        s.repo.UpdateResult(
            context.Background(), 
            task.ID, 
            ImageTaskStatusFailed, 
            nil, 
            &message, 
            timePtr(time.Now())
        )
        return
    }

    // 7. 成功：提取图片URL并更新状态
    imageURLs := extractImageURLs(result)
    completedAt := time.Now()
    s.repo.UpdateResult(
        context.Background(), 
        task.ID, 
        ImageTaskStatusSucceeded, 
        imageURLs, 
        nil, 
        &completedAt
    )
}
```

#### 重试判断逻辑

```go
func isRetryableImageError(err error) bool {
    if err == nil {
        return false
    }
    
    // 1. 上下文超时
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }
    
    // 2. 网络超时
    var netErr net.Error
    if errors.As(err, &netErr) {
        if netErr.Timeout() {
            return true
        }
    }
    
    // 3. 网关超时
    if infraerrors.IsGatewayTimeout(err) {
        return true
    }
    
    return false
}
```

### 3. 生图服务

**文件：** `backend/internal/service/image_generation_service.go`

#### 主生成流程

```go
func (s *ImageGenerationService) Generate(
    ctx context.Context, 
    input ImageGenerationInput
) (*ImageGenerationResult, error) {
    // 1. 参数验证
    if input.UserID <= 0 {
        return nil, ErrImageGenerationInvalid
    }
    modelID := strings.TrimSpace(input.ModelID)
    prompt := strings.TrimSpace(input.Prompt)
    if modelID == "" || prompt == "" {
        return nil, ErrImageGenerationInvalid
    }

    // 2. 获取超时配置
    timeoutSettings, err := s.settingService.GetGenerationTimeoutSettings(ctx)
    if err != nil {
        log.Printf("Failed to get timeout settings, using default: %v", err)
        timeoutSettings = DefaultGenerationTimeoutSettings()
    }

    // 3. 设置超时
    timeout := time.Duration(timeoutSettings.ImageTimeoutSeconds) * time.Second
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // 4. 从渠道系统选择账号
    creds, err := s.channelSelector.SelectForModel(ctx, input.UserID, modelID)
    if err != nil {
        return nil, fmt.Errorf("select channel for model %s: %w", modelID, err)
    }

    // 5. 解析请求端点类型
    requestEndpoint, err := s.resolveRequestEndpoint(ctx, input.UserID, modelID)
    if err != nil {
        return nil, err
    }

    // 6. 创建HTTP客户端
    httpClient := &http.Client{Timeout: timeout}

    // 7. 根据端点类型调用不同的生成方法
    var result *ImageGenerationResult
    switch requestEndpoint {
    case requestEndpointGemini:
        result, err = s.generateGeminiWithClient(
            ctx, httpClient, creds.BaseURL, creds.AccessKey, 
            modelID, prompt, input
        )
    case requestEndpointOpenAIMod:
        result, err = s.generateOpenAIModWithClient(
            ctx, httpClient, creds.BaseURL, creds.AccessKey, 
            modelID, prompt, input
        )
    default:
        result, err = s.generateOpenAIWithClient(
            ctx, httpClient, creds.BaseURL, creds.AccessKey, 
            modelID, prompt, input
        )
    }
    if err != nil {
        return nil, err
    }

    // 8. 存储生成的图片
    if s.storageService != nil && len(result.Images) > 0 {
        stored, err := s.storageService.StoreGeneratedImagesWithAuth(
            ctx, result.Images, creds.AccessKey
        )
        if err != nil {
            log.Printf("storage: failed to store images: %v", err)
        } else {
            result.Images = stored
        }
    }

    // 9. 确保所有图片都有URL
    result.Images = ensureGeneratedImageURLs(result.Images)

    // 10. 保存到图库
    s.persistGalleryRecords(ctx, input, result.Images)

    return result, nil
}
```

#### OpenAI格式生成

```go
func (s *ImageGenerationService) generateOpenAIWithClient(
    ctx context.Context,
    client *http.Client,
    baseURL string,
    accessKey string,
    modelID string,
    prompt string,
    input ImageGenerationInput,
) (*ImageGenerationResult, error) {
    // 1. 解析参考图片
    reference, err := parseReferenceImage(input.ReferenceImage)
    if err != nil {
        return nil, err
    }

    // 2. 构建尺寸参数
    size := buildImageSize(input.Resolution, input.AspectRatio)
    count := normalizeImageCount(input.Count)

    // 3. 构建HTTP请求
    var req *http.Request
    if reference == nil {
        // 3.1 文生图
        req, err = buildOpenAIImagesRequest(
            ctx, baseURL, accessKey, modelID, prompt, size, count
        )
    } else {
        // 3.2 图生图（编辑模式）
        req, err = buildOpenAIImageEditRequest(
            ctx, baseURL, accessKey, modelID, prompt, size, count, reference
        )
    }
    if err != nil {
        return nil, err
    }

    // 4. 发送请求
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request image generation: %w", err)
    }
    defer resp.Body.Close()

    // 5. 读取响应
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read image response: %w", err)
    }

    // 6. 检查状态码
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, infraerrors.ServiceUnavailable(
            "IMAGE_GENERATION_FAILED", 
            formatNewAPIError(resp.StatusCode, body)
        )
    }

    // 7. 解析响应
    result := parseOpenAIImageResponse(body)
    if len(result.Images) == 0 {
        return nil, infraerrors.ServiceUnavailable(
            "IMAGE_GENERATION_FAILED", 
            "no image returned"
        )
    }

    return result, nil
}
```

#### Gemini格式生成

```go
func (s *ImageGenerationService) generateGeminiWithClient(
    ctx context.Context,
    client *http.Client,
    baseURL string,
    accessKey string,
    modelID string,
    prompt string,
    input ImageGenerationInput,
) (*ImageGenerationResult, error) {
    // 1. 解析参考图片
    reference, err := parseReferenceImage(input.ReferenceImage)
    if err != nil {
        return nil, err
    }

    // 2. 构建Gemini请求
    req, err := buildGeminiGenerateContentRequest(
        ctx, baseURL, accessKey, modelID, prompt,
        input.Resolution, input.AspectRatio, reference,
    )
    if err != nil {
        return nil, err
    }

    // 3. 发送请求
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request gemini generation: %w", err)
    }
    defer resp.Body.Close()

    // 4. 读取响应
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read gemini response: %w", err)
    }

    // 5. 检查状态码
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, infraerrors.ServiceUnavailable(
            "IMAGE_GENERATION_FAILED", 
            formatNewAPIError(resp.StatusCode, body)
        )
    }

    // 6. 解析响应
    result, err := parseGeminiImageResponse(body)
    if err != nil {
        return nil, err
    }

    return result, nil
}
```


---

## 存储系统

### 文件位置
- 存储服务：`backend/internal/service/storage_service.go`
- 配置管理：`backend/internal/service/storage_settings.go`

### 存储架构

```
┌─────────────────────────────────────────────────────────────┐
│                    StorageService                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  StoreGeneratedImages(images []GeneratedImage)       │   │
│  │    ↓                                                  │   │
│  │  1. 判断存储后端（本地/S3）                            │   │
│  │  2. 下载图片（如果是URL）                              │   │
│  │  3. 存储到目标位置                                     │   │
│  │  4. 返回访问URL                                       │   │
│  └──────────────────────────────────────────────────────┘   │
│           │                           │                      │
│           ├───────────────────────────┤                      │
│           ↓                           ↓                      │
│  ┌─────────────────┐        ┌─────────────────┐            │
│  │  本地文件系统    │        │   S3兼容存储     │            │
│  │                 │        │                 │            │
│  │ storeLocal()    │        │ storeS3()       │            │
│  │ - 生成UUID      │        │ - MinIO客户端   │            │
│  │ - 按日期分目录   │        │ - 上传对象      │            │
│  │ - 写入文件      │        │ - 生成URL       │            │
│  └─────────────────┘        └─────────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

### 1. 本地存储实现

```go
// 存储到本地文件系统
func (s *StorageService) storeLocal(data []byte, mimeType string) (string, error) {
    // 1. 生成对象键（按日期分目录）
    key := buildObjectKey(mimeType)
    // 示例：2024/01/15/uuid.png
    
    // 2. 构建完整路径
    fullPath := filepath.Join(s.localRoot, filepath.FromSlash(key))
    
    // 3. 创建目录
    if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
        return "", ErrStorageFailed.WithCause(err)
    }
    
    // 4. 写入文件
    if err := os.WriteFile(fullPath, data, 0o644); err != nil {
        return "", ErrStorageFailed.WithCause(err)
    }
    
    // 5. 返回访问URL
    return buildLocalObjectURL(key), nil
}

// 构建对象键
func buildObjectKey(mimeType string) string {
    ext := extensionForMimeType(mimeType)
    now := time.Now().UTC()
    return path.Join(
        fmt.Sprintf("%04d", now.Year()),
        fmt.Sprintf("%02d", int(now.Month())),
        fmt.Sprintf("%02d", now.Day()),
        uuid.NewString()+ext,  // 使用UUID避免冲突
    )
}

// 构建本地访问URL
func buildLocalObjectURL(key string) string {
    clean := strings.TrimPrefix(key, "/")
    return localStorageURLPrefix + "/" + clean
    // 示例：/api/v1/storage/2024/01/15/uuid.png
}
```

**本地存储目录结构：**
```
data/uploads/
├── 2024/
│   ├── 01/
│   │   ├── 15/
│   │   │   ├── a1b2c3d4-e5f6-7890-abcd-ef1234567890.png
│   │   │   ├── b2c3d4e5-f6a7-8901-bcde-f12345678901.jpg
│   │   │   └── ...
│   │   ├── 16/
│   │   └── ...
│   ├── 02/
│   └── ...
└── ...
```

### 2. S3存储实现

```go
// 存储到S3兼容对象存储
func (s *StorageService) storeS3(
    ctx context.Context, 
    settings *StorageSettings, 
    data []byte, 
    mimeType string
) (string, error) {
    // 1. 验证配置
    if settings == nil || 
       settings.S3Endpoint == "" || 
       settings.S3Bucket == "" || 
       settings.S3AccessKey == "" || 
       settings.S3SecretKey == "" {
        return "", ErrStorageInvalid
    }

    // 2. 获取S3客户端（带缓存）
    client, err := s.getS3Client(settings)
    if err != nil {
        return "", ErrStorageFailed.WithCause(err)
    }

    // 3. 生成对象键
    key := buildObjectKey(mimeType)
    
    // 4. 上传对象
    reader := bytes.NewReader(data)
    _, err = client.PutObject(
        ctx, 
        settings.S3Bucket, 
        key, 
        reader, 
        int64(len(data)), 
        minio.PutObjectOptions{
            ContentType: mimeType,
        }
    )
    if err != nil {
        return "", ErrStorageFailed.WithCause(err)
    }

    // 5. 构建访问URL
    url, err := buildS3ObjectURL(settings, key)
    if err != nil {
        return "", ErrStorageFailed.WithCause(err)
    }
    
    return url, nil
}

// 构建S3访问URL
func buildS3ObjectURL(settings *StorageSettings, key string) (string, error) {
    key = strings.TrimPrefix(key, "/")
    
    // 1. 优先使用自定义公开URL
    base := strings.TrimRight(settings.S3PublicURL, "/")
    if base != "" {
        return base + "/" + key, nil
    }

    // 2. 解析端点
    endpoint, secure, err := normalizeS3Endpoint(
        settings.S3Endpoint, 
        settings.S3UseSSL
    )
    if err != nil {
        return "", err
    }

    scheme := "http"
    if secure {
        scheme = "https"
    }

    // 3. 根据路径样式构建URL
    if settings.S3PathStyle {
        // 路径样式：https://endpoint/bucket/key
        return fmt.Sprintf("%s://%s/%s/%s", 
            scheme, endpoint, settings.S3Bucket, key), nil
    }
    
    // 虚拟主机样式：https://bucket.endpoint/key
    return fmt.Sprintf("%s://%s.%s/%s", 
        scheme, settings.S3Bucket, endpoint, key), nil
}
```

### 3. S3客户端缓存

```go
// 获取S3客户端（带配置缓存）
func (s *StorageService) getS3Client(settings *StorageSettings) (*minio.Client, error) {
    if settings == nil {
        return nil, ErrStorageInvalid
    }

    s.s3Mu.Lock()
    defer s.s3Mu.Unlock()

    // 1. 检查缓存的客户端是否可用
    if s.s3Client != nil && 
       s.s3Config != nil && 
       sameS3Config(s.s3Config, settings) {
        return s.s3Client, nil
    }

    // 2. 解析端点
    endpoint, secure, err := normalizeS3Endpoint(
        settings.S3Endpoint, 
        settings.S3UseSSL
    )
    if err != nil {
        return nil, err
    }

    // 3. 创建新客户端
    opts := &minio.Options{
        Creds:  credentials.NewStaticV4(
            settings.S3AccessKey, 
            settings.S3SecretKey, 
            ""
        ),
        Secure: secure,
        Region: settings.S3Region,
    }
    
    if settings.S3PathStyle {
        opts.BucketLookup = minio.BucketLookupPath
    }

    client, err := minio.New(endpoint, opts)
    if err != nil {
        return nil, err
    }

    // 4. 缓存客户端和配置
    s.s3Client = client
    copied := *settings
    s.s3Config = &copied
    
    return client, nil
}
```

### 4. 图片下载与处理

```go
// 解析图片数据（支持多种来源）
func (s *StorageService) resolveImageData(
    ctx context.Context, 
    image GeneratedImage, 
    authHeader string
) ([]byte, string, error) {
    // 1. Base64数据
    if image.Base64 != "" {
        decoded, err := base64.StdEncoding.DecodeString(
            strings.TrimSpace(image.Base64)
        )
        if err != nil {
            return nil, "", ErrStorageInvalid.WithCause(err)
        }
        return decoded, normalizeMimeType(image.MimeType, decoded), nil
    }

    urlValue := strings.TrimSpace(image.URL)
    if urlValue == "" {
        return nil, "", ErrStorageInvalid
    }

    // 2. Data URL
    if strings.HasPrefix(urlValue, "data:") {
        decoded, mimeType, err := parseDataURL(urlValue)
        if err != nil {
            return nil, "", ErrStorageInvalid.WithCause(err)
        }
        return decoded, normalizeMimeType(mimeType, decoded), nil
    }

    // 3. HTTP URL（需要下载）
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlValue, nil)
    if err != nil {
        return nil, "", ErrStorageInvalid.WithCause(err)
    }
    
    // 添加认证头（如果需要）
    if headerValue := normalizeAuthHeader(authHeader); headerValue != "" {
        req.Header.Set("Authorization", headerValue)
    }
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, "", ErrStorageFailed.WithCause(err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, "", ErrStorageFailed.WithCause(
            fmt.Errorf("download failed with status %d", resp.StatusCode)
        )
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, "", ErrStorageFailed.WithCause(err)
    }
    
    return body, normalizeMimeType(resp.Header.Get("Content-Type"), body), nil
}
```

### 5. 存储健康检查

```go
// 检查存储系统健康状态
func (s *StorageService) CheckHealth(ctx context.Context) StorageHealth {
    status := StorageHealth{Backend: "unknown", Ready: false}
    
    if s == nil || s.settingService == nil {
        status.Error = "storage service not configured"
        return status
    }

    settings, err := s.settingService.GetStorageSettings(ctx)
    if err != nil {
        status.Error = fmt.Sprintf("load storage settings: %v", err)
        return status
    }

    // S3存储检查
    if settings.S3Enabled {
        status.Backend = "s3"
        status.Endpoint = settings.S3Endpoint
        status.Bucket = settings.S3Bucket
        
        missing := make([]string, 0, 4)
        if settings.S3Endpoint == "" {
            missing = append(missing, "endpoint")
        }
        if settings.S3Bucket == "" {
            missing = append(missing, "bucket")
        }
        if settings.S3AccessKey == "" {
            missing = append(missing, "access_key")
        }
        if settings.S3SecretKey == "" {
            missing = append(missing, "secret_key")
        }
        
        if len(missing) > 0 {
            status.Error = "missing s3 settings: " + strings.Join(missing, ", ")
            return status
        }
        
        status.Ready = true
        return status
    }

    // 本地存储检查
    status.Backend = "local"
    root := strings.TrimSpace(s.localRoot)
    status.LocalRoot = root
    
    if root == "" {
        status.Error = "local storage root not configured"
        return status
    }
    
    info, err := os.Stat(root)
    if err != nil {
        status.Error = fmt.Sprintf("local storage path unavailable: %v", err)
        return status
    }
    
    if !info.IsDir() {
        status.Error = "local storage path is not a directory"
        return status
    }
    
    status.Ready = true
    return status
}
```

---

## 配置管理

### 配置项定义

**文件：** `backend/internal/service/domain_constants.go`

```go
const (
    // 存储配置
    SettingKeyStorageS3Enabled   = "storage_s3_enabled"
    SettingKeyStorageS3Endpoint  = "storage_s3_endpoint"
    SettingKeyStorageS3Region    = "storage_s3_region"
    SettingKeyStorageS3Bucket    = "storage_s3_bucket"
    SettingKeyStorageS3AccessKey = "storage_s3_access_key"
    SettingKeyStorageS3SecretKey = "storage_s3_secret_key"
    SettingKeyStorageS3PublicURL = "storage_s3_public_url"
    SettingKeyStorageS3UseSSL    = "storage_s3_use_ssl"
    SettingKeyStorageS3PathStyle = "storage_s3_path_style"

    // 图片生成配置
    SettingKeyImageMaxRetryAttempts = "image_generation.max_retry_attempts"
    
    // 超时配置（在GenerationTimeoutSettings中）
    // ImageTimeoutSeconds - 图片生成超时时间（秒）
)
```

### 存储配置结构

```go
type StorageSettings struct {
    S3Enabled   bool   `json:"s3_enabled"`
    S3Endpoint  string `json:"s3_endpoint"`
    S3Region    string `json:"s3_region"`
    S3Bucket    string `json:"s3_bucket"`
    S3AccessKey string `json:"s3_access_key"`
    S3SecretKey string `json:"s3_secret_key"`
    S3PublicURL string `json:"s3_public_url"`
    S3UseSSL    bool   `json:"s3_use_ssl"`
    S3PathStyle bool   `json:"s3_path_style"`
}
```

### 超时配置

```go
type GenerationTimeoutSettings struct {
    ImageTimeoutSeconds int `json:"image_timeout_seconds"`
    VideoTimeoutSeconds int `json:"video_timeout_seconds"`
}

func DefaultGenerationTimeoutSettings() *GenerationTimeoutSettings {
    return &GenerationTimeoutSettings{
        ImageTimeoutSeconds: 180,  // 3分钟
        VideoTimeoutSeconds: 600,  // 10分钟
    }
}
```

### 配置示例

**YAML配置文件：** `deploy/config.example.yaml`

```yaml
# =============================================================================
# Storage Settings (存储设置)
# =============================================================================
storage:
  # 启用S3存储（false则使用本地存储）
  s3_enabled: false
  
  # S3端点（支持MinIO、阿里云OSS、腾讯云COS等）
  s3_endpoint: "https://s3.amazonaws.com"
  
  # S3区域
  s3_region: "us-east-1"
  
  # 存储桶名称
  s3_bucket: "dreamstudio-images"
  
  # 访问密钥
  s3_access_key: "your-access-key"
  s3_secret_key: "your-secret-key"
  
  # 公开访问URL（可选，用于CDN）
  s3_public_url: "https://cdn.example.com"
  
  # 是否使用SSL
  s3_use_ssl: true
  
  # 路径样式（MinIO通常需要true）
  s3_path_style: false

# =============================================================================
# Generation Settings (生成设置)
# =============================================================================
generation:
  # 图片生成超时（秒）
  image_timeout: 180
  
  # 视频生成超时（秒）
  video_timeout: 600
  
  # 最大重试次数
  max_retry_attempts: 3
```

---

## 模型管理与限流

### 文件位置
- RPM服务：`backend/internal/service/model_rpm_service.go`
- RPM辅助：`backend/internal/handler/model_rpm_helper.go`
- RPM缓存：`backend/internal/repository/model_rpm_cache.go`

### RPM限流架构

```
┌─────────────────────────────────────────────────────────────┐
│                   ModelRPMService                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  1. ResolveLimit(userID, modelID)                    │   │
│  │     → 查询用户模型配置                                 │   │
│  │     → 返回RPM限制和启用状态                            │   │
│  │                                                       │   │
│  │  2. Acquire(userID, modelID, rpm)                    │   │
│  │     → 尝试获取RPM槽位                                  │   │
│  │     → 返回是否成功和重试时间                           │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           ↓                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              ModelRPMCache (Redis)                    │   │
│  │  - 滑动窗口计数器                                      │   │
│  │  - 原子操作保证并发安全                                │   │
│  │  - 自动过期清理                                        │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 1. RPM限制解析

```go
// 解析用户模型的RPM限制
func (s *ModelRPMService) ResolveLimit(
    ctx context.Context, 
    userID int64, 
    modelID string
) (rpm int, enabled bool, err error) {
    if userID <= 0 {
        return 0, false, fmt.Errorf("invalid user id")
    }

    trimmedModel := strings.TrimSpace(modelID)
    if trimmedModel == "" {
        return 0, false, nil
    }

    if s.settingService == nil {
        return 0, false, nil
    }

    // 获取用户模型配置
    items, err := s.settingService.GetUserModelSettings(ctx, userID)
    if err != nil {
        return 0, false, err
    }

    // 查找匹配的模型配置
    for _, item := range items {
        if item.ModelID != trimmedModel {
            continue
        }
        rpm = normalizeRPM(item.RPM)
        enabled = normalizeRPMEnabled(item.RPMEnabled, rpm)
        return rpm, enabled, nil
    }

    return 0, false, nil
}
```

### 2. RPM槽位获取

```go
// 尝试获取RPM槽位
func (s *ModelRPMService) Acquire(
    ctx context.Context, 
    userID int64, 
    modelID string, 
    rpm int
) (*ModelRPMAcquireResult, error) {
    // 未配置限制或限制为0，直接通过
    if rpm <= 0 || s.cache == nil {
        return &ModelRPMAcquireResult{Acquired: true}, nil
    }

    // 生成请求ID（用于去重）
    requestID := generateRequestID()
    
    // 调用缓存层获取槽位
    acquired, retryAfter, err := s.cache.Acquire(
        ctx, userID, modelID, rpm, requestID
    )
    if err != nil {
        // 缓存失败时降级为允许通过
        log.Printf("Warning: model RPM cache acquire failed: %v", err)
        return &ModelRPMAcquireResult{Acquired: true}, nil
    }

    return &ModelRPMAcquireResult{
        Acquired:   acquired,
        RetryAfter: retryAfter,
    }, nil
}
```

### 3. 等待机制

```go
// 等待RPM槽位可用
func (h *ModelRPMHelper) WaitForModelRPM(
    c *gin.Context, 
    userID int64, 
    modelID string, 
    isStream bool, 
    streamStarted *bool
) error {
    if h == nil || h.rpmService == nil {
        return nil
    }

    trimmedModel := strings.TrimSpace(modelID)
    if trimmedModel == "" {
        return nil
    }

    // 1. 解析RPM限制
    rpm, enabled, err := h.rpmService.ResolveLimit(
        c.Request.Context(), userID, trimmedModel
    )
    if err != nil {
        return err
    }
    
    // 未启用限制，直接通过
    if !enabled || rpm <= 0 {
        return nil
    }

    // 2. 等待槽位（带Ping保活）
    return h.waitForSlotWithPing(
        c, userID, trimmedModel, rpm, isStream, streamStarted
    )
}

func (h *ModelRPMHelper) waitForSlotWithPing(
    c *gin.Context, 
    userID int64, 
    modelID string, 
    rpm int, 
    isStream bool, 
    streamStarted *bool
) error {
    ctx := c.Request.Context()
    
    // 1. 首次尝试获取
    result, err := h.rpmService.Acquire(ctx, userID, modelID, rpm)
    if err != nil {
        return err
    }
    if result.Acquired {
        return nil
    }

    // 2. 设置Ping机制（流式响应保活）
    needPing := isStream && h.pingFormat != ""
    var flusher http.Flusher
    if needPing {
        var ok bool
        flusher, ok = c.Writer.(http.Flusher)
        if !ok {
            return fmt.Errorf("streaming not supported")
        }
    }

    var pingCh <-chan time.Time
    if needPing {
        pingTicker := time.NewTicker(h.pingInterval)
        defer pingTicker.Stop()
        pingCh = pingTicker.C
    }

    // 3. 指数退避重试
    backoff := initialBackoff
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))
    wait := nextRPMWait(result.RetryAfter, &backoff, rng)
    timer := time.NewTimer(wait)
    defer timer.Stop()

    for {
        select {
        case <-ctx.Done():
            return &ModelRPMError{
                IsTimeout: ctx.Err() == context.DeadlineExceeded
            }

        case <-pingCh:
            // 发送Ping保持连接
            started := false
            if streamStarted != nil {
                started = *streamStarted
            }
            if !started {
                c.Header("Content-Type", "text/event-stream")
                c.Header("Cache-Control", "no-cache")
                c.Header("Connection", "keep-alive")
                c.Header("X-Accel-Buffering", "no")
                if streamStarted != nil {
                    *streamStarted = true
                }
            }
            fmt.Fprint(c.Writer, string(h.pingFormat))
            flusher.Flush()

        case <-timer.C:
            // 重试获取槽位
            result, err = h.rpmService.Acquire(ctx, userID, modelID, rpm)
            if err != nil {
                return err
            }
            if result.Acquired {
                return nil
            }
            wait = nextRPMWait(result.RetryAfter, &backoff, rng)
            timer.Reset(wait)
        }
    }
}
```

### 4. 用户模型配置

```go
type UserModelSetting struct {
    ModelID         string  `json:"model_id"`          // 模型ID
    ModelType       string  `json:"model_type"`        // 模型类型
    ModelName       string  `json:"model_name"`        // 模型名称
    RequestEndpoint string  `json:"request_endpoint"`  // 请求端点
    Resolution      string  `json:"resolution"`        // 分辨率
    AspectRatio     string  `json:"aspect_ratio"`      // 宽高比
    RPM             int     `json:"rpm"`               // RPM限制
    RPMEnabled      bool    `json:"rpm_enabled"`       // 是否启用RPM限制
}
```

**配置示例：**
```json
{
  "model_id": "dall-e-3",
  "model_type": "image",
  "model_name": "DALL-E 3",
  "request_endpoint": "openai",
  "resolution": "1K",
  "aspect_ratio": "1:1",
  "rpm": 5,
  "rpm_enabled": true
}
```


---

## 数据库设计

### 任务表结构

**文件：** `backend/migrations/047_image_generation_tasks.sql`

```sql
CREATE TABLE IF NOT EXISTS image_generation_tasks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_id VARCHAR(200) NOT NULL,
    prompt TEXT NOT NULL,
    resolution VARCHAR(32),
    aspect_ratio VARCHAR(32),
    reference_image TEXT,
    count INT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    image_urls JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- 索引优化
CREATE INDEX IF NOT EXISTS idx_image_generation_tasks_user_created_at
    ON image_generation_tasks (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_image_generation_tasks_status_created_at
    ON image_generation_tasks (status, created_at);
```

**重试字段扩展：** `backend/migrations/050_image_generation_tasks_retry.sql`

```sql
ALTER TABLE image_generation_tasks
    ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error TEXT,
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;

-- 重试任务索引
CREATE INDEX IF NOT EXISTS idx_image_generation_tasks_retry
    ON image_generation_tasks (status, next_attempt_at)
    WHERE deleted_at IS NULL;
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键，自增ID |
| user_id | BIGINT | 用户ID，外键关联users表 |
| model_id | VARCHAR(200) | 模型ID（如：dall-e-3） |
| prompt | TEXT | 提示词 |
| resolution | VARCHAR(32) | 分辨率（1K/2K/4K） |
| aspect_ratio | VARCHAR(32) | 宽高比（1:1/16:9等） |
| reference_image | TEXT | 参考图片（base64或URL） |
| count | INT | 生成数量（1-4） |
| status | VARCHAR(20) | 任务状态（pending/running/succeeded/failed） |
| error_message | TEXT | 错误信息（失败时） |
| image_urls | JSONB | 生成的图片URL数组 |
| attempts | INT | 已重试次数 |
| last_error | TEXT | 最后一次错误信息 |
| next_attempt_at | TIMESTAMPTZ | 下次重试时间 |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 更新时间 |
| completed_at | TIMESTAMPTZ | 完成时间 |

### 任务状态流转

```
┌─────────┐
│ pending │ ←─────────────────┐
└────┬────┘                   │
     │                        │
     │ Worker获取             │ 重试
     ↓                        │
┌─────────┐                   │
│ running │                   │
└────┬────┘                   │
     │                        │
     ├──────────┬─────────────┤
     │          │             │
     ↓          ↓             │
┌──────────┐ ┌────────┐      │
│succeeded │ │ failed │──────┘
└──────────┘ └────────┘
```

### 原子操作：获取待处理任务

```sql
-- 原子操作：获取下一个待处理任务并标记为running
UPDATE image_generation_tasks
SET status = 'running',
    updated_at = NOW(),
    attempts = attempts + 1
WHERE id = (
    SELECT id
    FROM image_generation_tasks
    WHERE status = 'pending'
      AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
      AND deleted_at IS NULL
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

**关键点：**
- `FOR UPDATE SKIP LOCKED`：跳过已被其他Worker锁定的行
- `ORDER BY created_at ASC`：先进先出（FIFO）
- `attempts = attempts + 1`：自动增加重试计数

### 重置僵尸任务

```sql
-- 重置超时的running任务为pending
UPDATE image_generation_tasks
SET status = 'pending',
    updated_at = NOW()
WHERE status = 'running'
  AND updated_at < NOW() - INTERVAL '10 minutes'
  AND deleted_at IS NULL;
```

---

## 关键代码示例

### 1. 完整的生图请求流程

**前端代码：**

```typescript
// frontend/src/views/HomeView.vue
async function handleGenerate() {
  if (!isAuthenticated.value) {
    appStore.showError(t('home.generator.loginHint'))
    return
  }
  
  if (!prompt.value.trim()) {
    appStore.showError(t('home.generator.promptRequired'))
    return
  }
  
  if (generating.value) return
  
  generating.value = true
  const requestModelId = resolveRequestModelId(selectedModel.value)
  
  try {
    // 1. 构建请求参数
    const referenceImagesBase64: string[] = []
    for (const img of referenceImages.value) {
      referenceImagesBase64.push(img.preview)
    }

    const payload = {
      model_id: requestModelId,
      prompt: prompt.value.trim(),
      resolution: selectedResolution.value,
      aspect_ratio: selectedRatio.value,
      reference_image: referenceImagesBase64[0] || undefined,
      async: true  // 异步模式
    }
    
    // 2. 批量创建任务
    const count = normalizeImageCount(imageCount.value)
    const results = await Promise.allSettled(
      Array.from({ length: count }, () => imagesAPI.createImageTask(payload))
    )
    
    // 3. 处理结果
    const succeeded = results.filter(r => r.status === 'fulfilled')
    const failed = results.filter(r => r.status === 'rejected')
    
    if (succeeded.length > 0) {
      // 添加到历史记录
      for (const result of succeeded) {
        if (result.status === 'fulfilled') {
          historyTasks.value = [
            result.value, 
            ...historyTasks.value.filter(t => t.id !== result.value.id)
          ]
        }
      }
      
      // 切换到历史标签
      activeTab.value = 'history'
      await loadHistory({ silent: true })
      
      appStore.showSuccess(
        t('home.generator.generateSuccess', { count: succeeded.length })
      )
    }
    
    if (failed.length > 0) {
      const error = (failed[0] as PromiseRejectedResult).reason
      appStore.showError(
        t('home.generator.generateFailed') + ': ' + 
        (error.message || t('common.unknownError'))
      )
    }
  } catch (error: any) {
    appStore.showError(
      t('home.generator.generateFailed') + ': ' + 
      (error.message || t('common.unknownError'))
    )
  } finally {
    generating.value = false
  }
}
```

### 2. 参数构建辅助函数

```go
// backend/internal/service/image_generation_service.go

// 构建图片尺寸（分辨率 + 宽高比 → 像素尺寸）
func buildImageSize(resolution, aspectRatio string) string {
    resolution = strings.TrimSpace(strings.ToUpper(resolution))
    aspectRatio = strings.TrimSpace(aspectRatio)
    
    if resolution == "" || aspectRatio == "" {
        return ""
    }

    // 解析基准尺寸
    base := 0
    switch resolution {
    case "1K":
        base = 1024
    case "2K":
        base = 2048
    case "4K":
        base = 4096
    default:
        return ""
    }

    // 解析宽高比
    ratioParts := strings.Split(aspectRatio, ":")
    if len(ratioParts) != 2 {
        return ""
    }
    
    widthRatio, err := strconv.Atoi(strings.TrimSpace(ratioParts[0]))
    if err != nil || widthRatio <= 0 {
        return ""
    }
    
    heightRatio, err := strconv.Atoi(strings.TrimSpace(ratioParts[1]))
    if err != nil || heightRatio <= 0 {
        return ""
    }

    // 计算实际尺寸
    maxRatio := widthRatio
    if heightRatio > maxRatio {
        maxRatio = heightRatio
    }

    width := base * widthRatio / maxRatio
    height := base * heightRatio / maxRatio
    
    if width <= 0 || height <= 0 {
        return ""
    }
    
    return fmt.Sprintf("%dx%d", width, height)
}

// 示例：
// buildImageSize("1K", "16:9") → "1024x576"
// buildImageSize("2K", "1:1")  → "2048x2048"
// buildImageSize("4K", "3:4")  → "3072x4096"
```

### 3. OpenAI请求构建

```go
// 构建OpenAI图片生成请求
func buildOpenAIImagesRequest(
    ctx context.Context,
    baseURL string,
    accessKey string,
    modelID string,
    prompt string,
    size string,
    count int,
) (*http.Request, error) {
    // 1. 构建端点URL
    endpoint, err := buildOpenAIImageURL(baseURL, "images/generations")
    if err != nil {
        return nil, err
    }

    // 2. 构建请求体
    payload := map[string]any{
        "model":  modelID,
        "prompt": prompt,
        "n":      count,
    }
    if size != "" {
        payload["size"] = size
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("encode image request: %w", err)
    }

    // 3. 创建HTTP请求
    req, err := http.NewRequestWithContext(
        ctx, 
        http.MethodPost, 
        endpoint, 
        bytes.NewReader(body)
    )
    if err != nil {
        return nil, fmt.Errorf("create image request: %w", err)
    }

    // 4. 设置请求头
    applyNewAPIHeaders(req, accessKey)
    req.Header.Set("Content-Type", "application/json")
    
    return req, nil
}

// 应用认证头
func applyNewAPIHeaders(req *http.Request, accessKey string) {
    if req == nil {
        return
    }
    
    authHeader := strings.TrimSpace(accessKey)
    if authHeader != "" && 
       !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
        authHeader = "Bearer " + authHeader
    }
    
    if authHeader != "" {
        req.Header.Set("Authorization", authHeader)
    }
    
    req.Header.Set("Accept", "application/json")
}
```

### 4. Gemini请求构建

```go
// 构建Gemini生成请求
func buildGeminiGenerateContentRequest(
    ctx context.Context,
    baseURL string,
    accessKey string,
    modelID string,
    prompt string,
    resolution string,
    aspectRatio string,
    reference *referenceImageData,
) (*http.Request, error) {
    // 1. 构建端点URL
    endpoint, err := buildGeminiGenerateContentURL(baseURL, modelID, accessKey)
    if err != nil {
        return nil, err
    }

    // 2. 构建内容部分
    parts := make([]map[string]any, 0, 2)
    parts = append(parts, map[string]any{"text": prompt})
    
    // 添加参考图片
    if reference != nil && reference.Base64 != "" {
        parts = append(parts, map[string]any{
            "inlineData": map[string]any{
                "mimeType": reference.MimeType,
                "data":     reference.Base64,
            },
        })
    }

    // 3. 构建生成配置
    generationConfig := map[string]any{
        "responseModalities": []string{"IMAGE"},
    }
    
    if imageConfig := buildGeminiImageConfig(resolution, aspectRatio); imageConfig != nil {
        generationConfig["imageConfig"] = imageConfig
    }

    // 4. 构建完整请求体
    payload := map[string]any{
        "contents": []map[string]any{
            {
                "parts": parts,
            },
        },
        "generationConfig": generationConfig,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("encode gemini request: %w", err)
    }

    // 5. 创建HTTP请求
    req, err := http.NewRequestWithContext(
        ctx, 
        http.MethodPost, 
        endpoint, 
        bytes.NewReader(body)
    )
    if err != nil {
        return nil, fmt.Errorf("create gemini request: %w", err)
    }

    applyNewAPIHeaders(req, accessKey)
    req.Header.Set("Content-Type", "application/json")
    
    return req, nil
}

// 构建Gemini图片配置
func buildGeminiImageConfig(resolution, aspectRatio string) map[string]any {
    config := map[string]any{}

    resolution = strings.TrimSpace(strings.ToUpper(resolution))
    switch resolution {
    case "1K", "2K", "4K":
        config["imageSize"] = resolution
    }

    ratio := strings.TrimSpace(aspectRatio)
    if ratio != "" && !strings.EqualFold(ratio, "auto") {
        config["aspectRatio"] = ratio
    }

    if len(config) == 0 {
        return nil
    }
    
    return config
}
```

### 5. 响应解析

```go
// OpenAI响应结构
type openAIImageResponse struct {
    Data []struct {
        URL     string `json:"url"`
        Base64  string `json:"b64_json"`
        Revised string `json:"revised_prompt"`
    } `json:"data"`
}

// 解析OpenAI响应
func parseOpenAIImageResponse(body []byte) *ImageGenerationResult {
    var resp openAIImageResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return &ImageGenerationResult{Images: []GeneratedImage{}}
    }
    
    images := make([]GeneratedImage, 0, len(resp.Data))
    for _, item := range resp.Data {
        image := GeneratedImage{
            URL:    strings.TrimSpace(item.URL),
            Base64: strings.TrimSpace(item.Base64),
        }
        
        if image.Base64 != "" {
            image.MimeType = defaultImageMimeType
        }
        
        if image.URL == "" && image.Base64 == "" {
            continue
        }
        
        images = append(images, image)
    }
    
    return &ImageGenerationResult{Images: images}
}

// Gemini响应结构
type geminiGenerateResponse struct {
    Candidates []struct {
        Content struct {
            Parts []struct {
                InlineData *struct {
                    MimeType string `json:"mime_type"`
                    Data     string `json:"data"`
                } `json:"inline_data"`
                FileData *struct {
                    MimeType string `json:"mime_type"`
                    FileURI  string `json:"file_uri"`
                } `json:"file_data"`
            } `json:"parts"`
        } `json:"content"`
    } `json:"candidates"`
}

// 解析Gemini响应
func parseGeminiImageResponse(body []byte) (*ImageGenerationResult, error) {
    var resp geminiGenerateResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, fmt.Errorf("parse gemini response: %w", err)
    }
    
    images := make([]GeneratedImage, 0)
    for _, candidate := range resp.Candidates {
        for _, part := range candidate.Content.Parts {
            // 处理内联数据
            if part.InlineData != nil {
                data, mimeType := normalizeGeminiInlineData(
                    part.InlineData.MimeType, 
                    part.InlineData.Data
                )
                if data != "" {
                    images = append(images, GeneratedImage{
                        Base64:   data,
                        MimeType: mimeType,
                    })
                }
                continue
            }
            
            // 处理文件URI
            if part.FileData != nil {
                url := strings.TrimSpace(part.FileData.FileURI)
                if url != "" {
                    images = append(images, GeneratedImage{
                        URL:      url,
                        MimeType: strings.TrimSpace(part.FileData.MimeType),
                    })
                }
            }
        }
    }
    
    if len(images) == 0 {
        return nil, infraerrors.ServiceUnavailable(
            "IMAGE_GENERATION_FAILED", 
            "no image returned from gemini"
        )
    }
    
    return &ImageGenerationResult{Images: images}, nil
}
```

---

## 部署与配置

### Docker部署

**文件：** `deploy/docker-compose.yml`

```yaml
version: '3.8'

services:
  dreamstudio:
    build:
      context: ..
      dockerfile: Dockerfile
    image: dreamstudio-local:latest
    container_name: dreamstudio
    restart: unless-stopped
    ports:
      - "${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080"
    volumes:
      # 数据持久化
      - dreamstudio_data:/app/data
      # 可选：挂载自定义配置
      # - ./config.yaml:/app/data/config.yaml:ro
    environment:
      - AUTO_SETUP=true
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=8080
      - SERVER_MODE=release
      - RUN_MODE=standard
      - DATABASE_HOST=postgres
      - DATABASE_PORT=5432
      - DATABASE_NAME=dreamstudio
      - DATABASE_USER=dreamstudio
      - DATABASE_PASSWORD=dreamstudio_password
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    container_name: dreamstudio-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_DB=dreamstudio
      - POSTGRES_USER=dreamstudio
      - POSTGRES_PASSWORD=dreamstudio_password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    container_name: dreamstudio-redis
    restart: unless-stopped
    volumes:
      - redis_data:/data

volumes:
  dreamstudio_data:
  postgres_data:
  redis_data:
```

### 配置文件示例

**文件：** `config.yaml`

```yaml
# =============================================================================
# Server Configuration
# =============================================================================
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"  # debug/release

# =============================================================================
# Database Configuration
# =============================================================================
database:
  host: "localhost"
  port: 5432
  name: "dreamstudio"
  user: "dreamstudio"
  password: "your_password"
  ssl_mode: "disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 300  # seconds

# =============================================================================
# Redis Configuration
# =============================================================================
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

# =============================================================================
# Storage Configuration
# =============================================================================
storage:
  # 本地存储根目录
  local_root: "./data/uploads"
  
  # S3配置（可选）
  s3:
    enabled: false
    endpoint: "https://s3.amazonaws.com"
    region: "us-east-1"
    bucket: "dreamstudio-images"
    access_key: "your_access_key"
    secret_key: "your_secret_key"
    public_url: ""  # CDN URL（可选）
    use_ssl: true
    path_style: false

# =============================================================================
# Image Generation Configuration
# =============================================================================
image_generation:
  # Worker配置
  worker_count: 2
  poll_interval: 2  # seconds
  
  # 超时配置
  timeout: 180  # seconds (3 minutes)
  
  # 重试配置
  max_attempts: 3
  retry_backoff:
    - 10   # 第1次重试：10秒
    - 30   # 第2次重试：30秒
    - 120  # 第3次重试：2分钟
  
  # 僵尸任务检测
  stale_after: 600  # seconds (10 minutes)
  stale_check_interval: 30  # seconds

# =============================================================================
# Model Configuration
# =============================================================================
models:
  # 默认模型配置
  default:
    resolution: "1K"
    aspect_ratio: "1:1"
    rpm: 0  # 0表示不限制
    rpm_enabled: false
  
  # 模型列表
  list:
    - model_id: "dall-e-3"
      model_type: "image"
      model_name: "DALL-E 3"
      request_endpoint: "openai"
      resolution: "1K"
      aspect_ratio: "1:1"
      rpm: 5
      rpm_enabled: true
    
    - model_id: "gemini-3.0-pro-image-preview"
      model_type: "image"
      model_name: "Gemini 3.0 Pro Image"
      request_endpoint: "gemini"
      resolution: "2K"
      aspect_ratio: "16:9"
      rpm: 10
      rpm_enabled: true
```

### 环境变量配置

```bash
# .env 文件示例

# 服务器配置
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_MODE=release

# 数据库配置
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=dreamstudio
DATABASE_USER=dreamstudio
DATABASE_PASSWORD=your_password

# Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# 存储配置
STORAGE_LOCAL_ROOT=./data/uploads
STORAGE_S3_ENABLED=false
STORAGE_S3_ENDPOINT=https://s3.amazonaws.com
STORAGE_S3_BUCKET=dreamstudio-images
STORAGE_S3_ACCESS_KEY=your_access_key
STORAGE_S3_SECRET_KEY=your_secret_key

# 生成配置
IMAGE_GENERATION_TIMEOUT=180
IMAGE_GENERATION_MAX_ATTEMPTS=3
IMAGE_GENERATION_WORKER_COUNT=2
```

### 启动命令

```bash
# 1. 使用Docker Compose
cd deploy
docker-compose up -d

# 2. 查看日志
docker-compose logs -f dreamstudio

# 3. 停止服务
docker-compose down

# 4. 重启服务
docker-compose restart dreamstudio

# 5. 查看Worker状态
docker-compose exec dreamstudio ps aux | grep worker
```

### 健康检查

```bash
# 检查服务状态
curl http://localhost:8080/health

# 检查存储状态
curl http://localhost:8080/api/v1/admin/storage/health

# 检查数据库连接
curl http://localhost:8080/api/v1/admin/health/db

# 检查Redis连接
curl http://localhost:8080/api/v1/admin/health/redis
```

---

## 总结

本文档详细介绍了AI绘画功能的完整技术实现，包括：

### 核心特性

1. **异步任务机制**
   - 用户提交后立即返回，可关闭网页
   - 后台Worker自动处理
   - 支持任务状态查询

2. **智能重试**
   - 自动识别可重试错误
   - 指数退避策略
   - 最大重试次数限制

3. **灵活存储**
   - 本地文件系统
   - S3兼容对象存储
   - 自动下载和转换

4. **模型限流**
   - 单模型RPM限制
   - 滑动窗口计数
   - 等待队列机制

5. **多平台支持**
   - OpenAI格式
   - Gemini格式
   - 自定义端点

### 关键技术点

- **数据库原子操作**：`FOR UPDATE SKIP LOCKED` 保证并发安全
- **僵尸任务检测**：定期重置超时任务
- **配置热更新**：定期刷新配置，无需重启
- **Panic恢复**：Worker崩溃自动恢复
- **存储抽象**：统一接口支持多种存储后端

### 可扩展性

- **水平扩展**：增加Worker数量提高并发
- **存储扩展**：轻松切换存储后端
- **模型扩展**：添加新模型只需配置
- **端点扩展**：支持自定义请求格式

### 监控与维护

- 任务状态监控
- 存储健康检查
- 错误日志记录
- 性能指标收集

---

**文档版本：** 1.0  
**最后更新：** 2026-02-28  
**适用版本：** DreamStudio v1.0+

