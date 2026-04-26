# AI绘画功能技术实现补充文档

> 本文档补充《AI绘画功能技术实现文档.md》中缺失的关键实现细节，重点覆盖多参考图片支持、模型配置映射、渠道选择机制等核心功能��

## 文档版本
- **创建时间**: 2026-04-26
- **基于主文档**: AI绘画功能技术实现文档.md (2480行)
- **补充范围**: 前端请求构建、后端API处理、模型配置系统、多参考图片实现

---

## 一、多参考图片支持实现

### 1.1 前端实现

#### 用户交互层
**文件**: `frontend/src/views/HomeView.vue`

**状态管理** (行1308-1321):
```typescript
const referenceImages = ref<Array<{ file: File; preview: string }>>([])
const MAX_REFERENCE_IMAGES = 3  // 图片模式最多3张
const MAX_IMAGE_SIZE = 10 * 1024 * 1024  // 单张最大10MB
const maxReferenceImages = computed(() => (isVideoMode.value ? 1 : MAX_REFERENCE_IMAGES))
```

**上传处理流程** (行2669-2715):
```typescript
// 1. 拖拽上传
function handleDrop(event: DragEvent) {
  const files = event.dataTransfer?.files
  if (files) {
    Array.from(files).forEach(file => addReferenceImage(file))
  }
}

// 2. 文件选择上传
function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files
  if (files) {
    Array.from(files).forEach(file => addReferenceImage(file))
  }
}

// 3. 添加参考图片并转换为base64
async function addReferenceImage(file: File) {
  // 验证文件类型
  if (!file.type.startsWith('image/')) {
    appStore.showError(t('home.generator.invalidImage'))
    return
  }

  // 验证数量限制
  if (referenceImages.value.length >= maxReferenceImages.value) {
    appStore.showError(t('home.generator.maxImagesReached'))
    return
  }

  // 验证文件大小
  if (file.size > MAX_IMAGE_SIZE) {
    appStore.showError(t('home.generator.imageTooLarge'))
    return
  }

  try {
    const base64Url = await fileToBase64(file)
    referenceImages.value.push({
      file,
      preview: base64Url  // data:image/png;base64,xxx格式
    })
  } catch (error) {
    appStore.showError(t('home.generator.imageProcessError'))
  }
}

// 4. Base64转换工具
function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}
```

#### 请求构建层
**文件**: `frontend/src/views/HomeView.vue` (行2884-2986)

```typescript
async function handleGenerate() {
  // 1. 解析实际请求的模型ID
  const requestModelId = resolveRequestModelId(selectedModel.value)

  // 2. 处理多张参考图片
  const referenceImagesBase64: string[] = []
  for (const img of referenceImages.value) {
    referenceImagesBase64.push(img.preview)  // 已是data URL格式
  }

  // 3. 构建请求payload
  const payload = {
    model_id: requestModelId,
    prompt: prompt.value.trim(),
    resolution: selectedResolution.value,
    aspect_ratio: selectedRatio.value,
    reference_image: referenceImagesBase64[0] || undefined,  // 向后兼容
    reference_images: referenceImagesBase64.length > 0 ? referenceImagesBase64 : undefined,
    async: true
  }

  // 4. 批量发送请求
  const count = normalizeImageCount(imageCount.value)
  const results = await Promise.allSettled(
    Array.from({ length: count }, () => imagesAPI.createImageTask(payload))
  )

  // 5. 处理结果并更新历史记录
  // ...
}
```

**关键设计**:
- **双字段兼容**: 同时发送 `reference_image` (单张) 和 `reference_images` (数组)
- **向后兼容**: 旧版后端只识别 `reference_image`，新版可处理 `reference_images`
- **数据格式**: 统一使用 data URL 格式 (`data:image/xxx;base64,xxx`)

#### API接口定义
**文件**: `frontend/src/api/images.ts`

**当前定义** (行4-12):
```typescript
export interface ImageGeneratePayload {
  model_id: string
  prompt: string
  resolution?: string
  aspect_ratio?: string
  reference_image?: string      // 单张参考图片
  count?: number
  async?: boolean
}
```

**⚠️ 类型不匹配问题**:
实际代码中使用了 `reference_images?: string[]` 字段，但接口定义中缺失。建议补充：
```typescript
export interface ImageGeneratePayload {
  model_id: string
  prompt: string
  resolution?: string
  aspect_ratio?: string
  reference_image?: string      // 保留向后兼容
  reference_images?: string[]   // 添加多图支持
  count?: number
  async?: boolean
}
```

### 1.2 后端实现

#### Handler层
**文件**: `backend/internal/handler/image_generation_handler.go`

**请求结构**:
```go
type ImageGenerateRequest struct {
    ModelID        string `json:"model_id"`
    Prompt         string `json:"prompt"`
    Resolution     string `json:"resolution,omitempty"`
    AspectRatio    string `json:"aspect_ratio,omitempty"`
    ReferenceImage string `json:"reference_image,omitempty"`  // 当前仅支持单张
    Count          int    `json:"count,omitempty"`
    Async          bool   `json:"async,omitempty"`
}
```

**⚠️ 待扩展**: 需添加 `ReferenceImages []string` 字段以支持多图。

#### Service层参考图处理
**文件**: `backend/internal/service/image_generation_service.go`

**解析函数** (行531-564):
```go
func parseReferenceImage(refImage string) ([]byte, string, error) {
    if refImage == "" {
        return nil, "", nil
    }

    var data string
    mimeType := "image/png"  // 默认MIME类型

    // 解析data URI格式: data:image/png;base64,xxx
    if strings.HasPrefix(refImage, "data:") {
        parts := strings.SplitN(refImage, ",", 2)
        if len(parts) == 2 {
            data = parts[1]
            // 提取MIME类型
            if strings.Contains(parts[0], ";") {
                mimePart := strings.Split(parts[0], ";")[0]
                mimeType = strings.TrimPrefix(mimePart, "data:")
            }
        }
    } else {
        data = refImage  // 纯base64
    }

    // 解码base64
    decoded, err := base64.StdEncoding.DecodeString(data)
    if err != nil {
        return nil, "", fmt.Errorf("invalid base64: %w", err)
    }

    return decoded, mimeType, nil
}
```

**存储函数** (行566-601):
```go
func (s *ImageGenerationService) storeReferenceImage(
    ctx context.Context,
    refImage string,
) (string, error) {
    if refImage == "" {
        return "", nil
    }

    // 如果是URL，直接存储
    if strings.HasPrefix(refImage, "http://") || strings.HasPrefix(refImage, "https://") {
        return s.storageService.StoreImage(ctx, refImage, "reference")
    }

    // 解析base64
    imageData, mimeType, err := parseReferenceImage(refImage)
    if err != nil {
        return "", err
    }

    // 生成文件扩展名
    ext := ".png"
    if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
        ext = ".jpg"
    } else if strings.Contains(mimeType, "webp") {
        ext = ".webp"
    }

    // 存储到对象存储
    return s.storageService.StoreImageBytes(ctx, imageData, "reference", ext)
}
```

#### 提供商适配器多图支持

**Gemini端点** (行474-522) - **已支持多图**:
```go
func (s *ImageGenerationService) generateGeminiWithClient(
    ctx context.Context,
    input ImageGenerationInput,
    client *http.Client,
    baseURL, apiKey string,
) (*ImageGenerationResult, error) {
    // 构建parts数组
    parts := []map[string]any{
        {"text": input.Prompt},
    }

    // 添加参考图片
    if input.ReferenceImage != "" {
        imageData, mimeType, err := parseReferenceImage(input.ReferenceImage)
        if err != nil {
            return nil, err
        }

        parts = append(parts, map[string]any{
            "inlineData": map[string]any{
                "mimeType": mimeType,
                "data":     base64.StdEncoding.EncodeToString(imageData),
            },
        })
    }

    // 构建请求体
    reqBody := map[string]any{
        "contents": []map[string]any{
            {"parts": parts},
        },
        "generationConfig": map[string]any{
            "responseModalities": []string{"IMAGE"},
            "imageConfig": map[string]any{
                "imageSize":   input.Resolution,
                "aspectRatio": input.AspectRatio,
            },
        },
    }

    // 发送请求到 /v1beta/models/{model}:generateContent
    // ...
}
```

**扩展方案**: 支持 `input.ReferenceImages []string`，在 `parts` 数组中添加多个 `inlineData`。

**OpenAI标准端点** (行295-343) - **仅支持单图**:
```go
func (s *ImageGenerationService) generateOpenAIWithClient(...) {
    if input.ReferenceImage != "" {
        // 使用 /v1/images/edits 端点 (multipart/form-data)
        imageData, _, err := parseReferenceImage(input.ReferenceImage)
        // ...
        body := &bytes.Buffer{}
        writer := multipart.NewWriter(body)

        // 添加image字段
        part, _ := writer.CreateFormFile("image", "reference.png")
        part.Write(imageData)

        writer.WriteField("prompt", input.Prompt)
        writer.WriteField("model", input.ModelID)
        writer.WriteField("size", size)
        writer.WriteField("n", strconv.Itoa(input.Count))
        writer.Close()

        req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/images/edits", body)
        req.Header.Set("Content-Type", writer.FormDataContentType())
    } else {
        // 使用 /v1/images/generations 端点 (JSON)
        // ...
    }
}
```

**限制**: OpenAI标准API的 `/v1/images/edits` 端点仅支持单张 `image` 字段。

**OpenAI Mod端点** (行356-418) - **可扩展**:
```go
func (s *ImageGenerationService) generateOpenAIModWithClient(...) {
    reqBody := map[string]any{
        "model":  input.ModelID,
        "prompt": input.Prompt,
        "n":      input.Count,
    }

    // 添加image_config
    if input.Resolution != "" || input.AspectRatio != "" {
        imageConfig := make(map[string]any)
        if input.AspectRatio != "" {
            imageConfig["aspect_ratio"] = input.AspectRatio
        }
        if input.Resolution != "" {
            imageConfig["image_size"] = input.Resolution
        }
        reqBody["image_config"] = imageConfig
    }

    // 添加参考图片
    if input.ReferenceImage != "" {
        reqBody["image"] = input.ReferenceImage  // 直接传递base64字符串
    }

    // 发送JSON请求到 /v1/images/generations
    // ...
}
```

**扩展方案**: 将 `reqBody["image"]` 改为 `reqBody["images"] = []string{...}`（需确认API支持）。

---

## 二、模型配置与映射系统

### 2.1 模型配置数据结构

#### 前端类型定义
**文件**: `frontend/src/api/modelSettings.ts` (行8-25)

```typescript
export type RequestEndpoint = 'openai' | 'gemini' | 'openai_mod' | 'qwen' | 'sora'
export type ModelType = 'image' | 'video' | 'text'

export interface UserModelSetting {
  model_id: string              // 前端显示的模型ID
  model_series?: string         // 模型系列 (用于分组)
  request_model_id?: string     // 实际请求时使用的模型ID
  resolutions: string[]         // 支持的分辨率 ['1K', '2K', '4K']
  aspect_ratios: string[]       // 支持的宽高比 ['Auto', '1:1', '16:9'...]
  durations: string[]           // 视频时长选项 (仅视频模型)
  video_modes?: VideoMode[]     // 视频模式 ['text_to_video', 'image_to_video']
  request_endpoint?: RequestEndpoint  // 请求端点类型
  model_type?: ModelType        // 模型类型
  display_name?: string         // 显示名称
  rpm?: number                  // 每分钟请求限制
  rpm_enabled?: boolean         // 是否启用限流
}
```

#### 后端结构定义
**文件**: `backend/internal/service/user_model_settings.go` (行27-40)

```go
type UserModelSetting struct {
    ModelID         string   `json:"model_id"`
    ModelSeries     string   `json:"model_series,omitempty"`
    RequestModelID  string   `json:"request_model_id,omitempty"`
    Resolutions     []string `json:"resolutions"`
    AspectRatios    []string `json:"aspect_ratios"`
    Durations       []string `json:"durations,omitempty"`
    VideoModes      []string `json:"video_modes,omitempty"`
    RequestEndpoint string   `json:"request_endpoint,omitempty"`
    ModelType       string   `json:"model_type,omitempty"`
    DisplayName     string   `json:"display_name,omitempty"`
    RPM             int      `json:"rpm,omitempty"`
    RPMEnabled      bool     `json:"rpm_enabled,omitempty"`
}
```

**存储位置**:
- **数据库**: `settings` 表，key = `admin_model_settings`
- **缓存**: Redis，相同key
- **格式**: JSON数组 `[UserModelSetting, ...]`

### 2.2 双ID映射机制

**设计目的**: 允许管理员配置"魔改版本"，前端显示友好名称，后端使用实际模型ID。

**示例场景**:
```json
{
  "model_id": "DALL-E 3 HD",           // 前端显示
  "request_model_id": "dall-e-3-hd-custom",  // 实际请求
  "display_name": "DALL-E 3 高清版",
  "request_endpoint": "openai_mod"
}
```

**前端解析逻辑** (`frontend/src/views/HomeView.vue` 行1552-1555):
```typescript
const resolveRequestModelId = (modelId: string) => {
  const custom = modelSettings.value[modelId]?.request_model_id?.trim()
  return custom || modelId  // 优先使用request_model_id
}
```

**后端使用**: Handler层直接使用前端传递的 `model_id`（已解析）。

### 2.3 请求端点类型映射

**端点常量** (`backend/internal/service/image_generation_service.go` 行15-19):
```go
const (
    requestEndpointOpenAI    = "openai"      // 标准OpenAI API
    requestEndpointGemini    = "gemini"      // Google Gemini API
    requestEndpointOpenAIMod = "openai_mod"  // OpenAI魔改版
    requestEndpointQwen      = "qwen"        // 通义千问
    requestEndpointSora      = "sora"        // OpenAI Sora (仅视频)
)
```

**解析逻辑** (行268-282):
```go
func (s *ImageGenerationService) resolveRequestEndpoint(
    ctx context.Context,
    modelID string,
) (string, error) {
    setting, err := s.userModelSettingsService.GetModelSetting(ctx, modelID)
    if err != nil || setting == nil {
        return requestEndpointOpenAI, nil  // 默认OpenAI
    }

    endpoint := strings.TrimSpace(setting.RequestEndpoint)
    if endpoint == "" {
        return requestEndpointOpenAI, nil
    }

    return endpoint, nil
}
```

**路由分发** (行119-128):
```go
switch requestEndpoint {
case requestEndpointGemini:
    return s.generateGeminiWithClient(ctx, input, client, baseURL, apiKey)
case requestEndpointOpenAIMod:
    return s.generateOpenAIModWithClient(ctx, input, client, baseURL, apiKey)
case requestEndpointQwen:
    return s.generateQwenWithClient(ctx, input, client, baseURL, apiKey)
default:
    return s.generateOpenAIWithClient(ctx, input, client, baseURL, apiKey)
}
```

### 2.4 分辨率和宽高比配置

#### 前端预设选项
**文件**: `frontend/src/views/user/ModelSettingsView.vue` (行546-561)

```typescript
const resolutionOptions = ['1K', '2K', '4K']
const videoResolutionOptions = ['480p', '720p', '1080p', '1K', '2K', '4K']
const aspectRatioOptions = [
  'Auto', '1:1', '2:3', '3:2', '3:4', '4:3',
  '4:5', '5:4', '9:16', '16:9', '21:9'
]
const durationOptions = ['5s', '10s', '15s', '20s', '25s']
```

#### 后端规范化逻辑
**文件**: `backend/internal/service/user_model_settings.go`

**字符串列表去重** (行149-165):
```go
func normalizeStringList(list []string) []string {
    seen := make(map[string]bool)
    result := []string{}
    for _, item := range list {
        trimmed := strings.TrimSpace(item)
        if trimmed != "" && !seen[trimmed] {
            seen[trimmed] = true
            result = append(result, trimmed)
        }
    }
    return result
}
```

**宽高比规范化** (行167-193):
```go
func normalizeAspectRatioList(list []string, modelType string) []string {
    normalized := normalizeStringList(list)

    // 视频模型移除"Auto"选项
    if modelType == "video" {
        filtered := []string{}
        for _, ratio := range normalized {
            if ratio != "Auto" {
                filtered = append(filtered, ratio)
            }
        }
        return filtered
    }

    return normalized
}
```

#### 像素尺寸计算
**文件**: `backend/internal/service/image_generation_service.go` (行1240-1283)

```go
func buildImageSize(resolution, aspectRatio string) string {
    // 1. 确定基准分辨率
    var base int
    switch resolution {
    case "1K":
        base = 1024
    case "2K":
        base = 2048
    case "4K":
        base = 4096
    default:
        base = 1024
    }

    // 2. 解析宽高比
    widthRatio, heightRatio := 1.0, 1.0
    if aspectRatio != "" && aspectRatio != "Auto" {
        parts := strings.Split(aspectRatio, ":")
        if len(parts) == 2 {
            w, _ := strconv.ParseFloat(parts[0], 64)
            h, _ := strconv.ParseFloat(parts[1], 64)
            if w > 0 && h > 0 {
                widthRatio, heightRatio = w, h
            }
        }
    }

    // 3. 计算实际像素
    maxRatio := math.Max(widthRatio, heightRatio)
    width := int(float64(base) * widthRatio / maxRatio)
    height := int(float64(base) * heightRatio / maxRatio)

    // 4. 对齐到8的倍数 (某些模型要求)
    width = (width / 8) * 8
    height = (height / 8) * 8

    return fmt.Sprintf("%dx%d", width, height)
}
```

**示例计算**:
- `resolution="2K"`, `aspect_ratio="16:9"` → `2048x1152`
- `resolution="1K"`, `aspect_ratio="1:1"` → `1024x1024`
- `resolution="4K"`, `aspect_ratio="21:9"` → `4096x1752`

---

## 三、渠道选择与账号管理

### 3.1 渠道选择器架构

**文件**: `backend/internal/service/channel_selector.go`

**核心接口**:
```go
type ChannelSelector interface {
    SelectForModel(ctx context.Context, userID int64, modelID string) (*SelectedChannel, error)
}

type SelectedChannel struct {
    BaseURL string
    APIKey  string
    Account *Account  // 可选，用于日志和统计
}
```

**选择流程** (行49-75):
```go
func (s *channelSelectorImpl) SelectForModel(
    ctx context.Context,
    userID int64,
    modelID string,
) (*SelectedChannel, error) {
    // 1. 检查用户自定义API Key
    userKey, err := s.userAPIKeyService.GetUserAPIKey(ctx, userID)
    if err == nil && userKey != nil && userKey.Enabled {
        // 仍需选择账号获取BaseURL
        account, err := s.selectAccountForModel(ctx, modelID)
        if err != nil {
            return nil, err
        }

        return &SelectedChannel{
            BaseURL: account.BaseURL,
            APIKey:  userKey.APIKey,  // 使用用户Key
            Account: account,
        }, nil
    }

    // 2. 使用系统渠道池
    account, err := s.selectAccountForModel(ctx, modelID)
    if err != nil {
        return nil, err
    }

    return &SelectedChannel{
        BaseURL: account.BaseURL,
        APIKey:  account.APIKey,
        Account: account,
    }, nil
}
```

### 3.2 账号选择算法

**文件**: `backend/internal/service/channel_selector.go` (行85-109)

```go
func (s *channelSelectorImpl) selectAccountForModel(
    ctx context.Context,
    modelID string,
) (*Account, error) {
    // 1. 解析目标渠道类型
    channelType := s.resolveChannelType(modelID)

    // 2. 查询可用账号
    accounts, err := s.accountRepo.ListAvailableAccounts(ctx, "openai")
    if err != nil {
        return nil, err
    }

    // 3. 过滤支持该模型的账号
    supported := make([]*Account, 0, len(accounts))
    for i := range accounts {
        acc := &accounts[i]
        if accountChannelType(acc) == targetChannelType && s.isModelSupported(acc, modelID) {
            supported = append(supported, acc)
        }
    }

    // 4. 无可用账号时返回错误
    if len(supported) == 0 {
        return nil, fmt.Errorf("no available channel for model: %s", modelID)
    }

    // 5. 按优先级和权重选择
    return pickAccountByPriorityAndWeight(supported), nil
}
```

**模型支持检查** (行138-150):
```go
func (s *ChannelSelector) isModelSupported(acc *Account, modelID string) bool {
    allowlist := acc.ModelsAllowlist
    if len(allowlist) == 0 {
        // 空白名单表示支持所有模型
        return true
    }
    for _, m := range allowlist {
        if strings.EqualFold(m, modelID) {
            return true
        }
    }
    return false
}
```

**优先级和权重选择算法** (行152-184):
```go
func pickAccountByPriorityAndWeight(accounts []*Account) *Account {
    // 1. 找出最小优先级值（数字越小优先级越高）
    minPriority := accounts[0].Priority
    for _, acc := range accounts[1:] {
        if acc.Priority < minPriority {
            minPriority = acc.Priority
        }
    }

    // 2. 筛选出最高优先级的账号
    candidates := make([]*Account, 0, len(accounts))
    totalWeight := 0
    for _, acc := range accounts {
        if acc.Priority != minPriority {
            continue
        }
        candidates = append(candidates, acc)
        totalWeight += normalizedChannelWeight(acc.Weight)
    }

    // 3. 单个候选直接返回
    if len(candidates) == 1 {
        return candidates[0]
    }

    // 4. 按权重随机选择（加权轮询）
    if totalWeight <= 0 {
        return candidates[0]
    }

    target := rand.Intn(totalWeight)
    current := 0
    for _, acc := range candidates {
        current += normalizedChannelWeight(acc.Weight)
        if current > target {
            return acc
        }
    }

    return candidates[len(candidates)-1]
}
```

**选择策略总结**:
1. **渠道类型匹配**: 根据模型配置的 `request_endpoint` 确定目标渠道类型
2. **模型白名单过滤**: 仅选择 `ModelsAllowlist` 包含该模型的账号（空白名单=支持所有）
3. **优先级排序**: 选择 `Priority` 值最小的账号组
4. **权重负载均衡**: 在同优先级账号中按 `Weight` 加权随机选择

### 3.3 渠道类型映射

**文件**: `backend/internal/service/channel_selector.go`

**端点到渠道类型转换**:
```go
func channelTypeFromRequestEndpoint(endpoint string) string {
    switch strings.TrimSpace(strings.ToLower(endpoint)) {
    case "openai", "openai_mod":
        return ChannelTypeOpenAI
    case "gemini":
        return ChannelTypeGemini
    case "qwen":
        return ChannelTypeQwen
    case "sora":
        return ChannelTypeSora
    default:
        return ChannelTypeOpenAI
    }
}
```

**渠道类型常量**:
- `ChannelTypeOpenAI`: 标准OpenAI和魔改版
- `ChannelTypeGemini`: Google Gemini
- `ChannelTypeQwen`: 阿里���义千问
- `ChannelTypeSora`: OpenAI Sora视频生成

---

## 四、完整请求示例

### 4.1 OpenAI标准端点

**无参考图片**:
```http
POST /v1/images/generations
Content-Type: application/json
Authorization: Bearer sk-xxx

{
  "model": "dall-e-3",
  "prompt": "A beautiful sunset over mountains",
  "size": "1024x1024",
  "n": 1,
  "quality": "standard"
}
```

**有参考图片**:
```http
POST /v1/images/edits
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

------WebKitFormBoundary
Content-Disposition: form-data; name="image"; filename="reference.png"
Content-Type: image/png

[二进制图片数据]
------WebKitFormBoundary
Content-Disposition: form-data; name="prompt"

A beautiful sunset over mountains
------WebKitFormBoundary
Content-Disposition: form-data; name="model"

dall-e-3
------WebKitFormBoundary
Content-Disposition: form-data; name="size"

1024x1024
------WebKitFormBoundary--
```

### 4.2 Gemini端点

```http
POST /v1beta/models/gemini-2.0-flash-exp:generateContent
Content-Type: application/json
x-goog-api-key: AIzaSyxxx

{
  "contents": [
    {
      "parts": [
        {
          "text": "A beautiful sunset over mountains"
        },
        {
          "inlineData": {
            "mimeType": "image/png",
            "data": "iVBORw0KGgoAAAANSUhEUgAA..."
          }
        }
      ]
    }
  ],
  "generationConfig": {
    "responseModalities": ["IMAGE"],
    "imageConfig": {
      "imageSize": "2K",
      "aspectRatio": "16:9"
    }
  }
}
```

**响应格式**:
```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "inlineData": {
              "mimeType": "image/png",
              "data": "iVBORw0KGgoAAAANSUhEUgAA..."
            }
          }
        ]
      }
    }
  ]
}
```

### 4.3 OpenAI魔改端点

```http
POST /v1/images/generations
Content-Type: application/json
Authorization: Bearer sk-xxx

{
  "model": "dall-e-3-hd-custom",
  "prompt": "A beautiful sunset over mountains",
  "n": 1,
  "image_config": {
    "image_size": "2K",
    "aspect_ratio": "16:9"
  },
  "image": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..."
}
```

**特点**:
- 使用JSON格式（非multipart）
- 支持 `image_config` 对象传递分辨率和宽高比
- 参考图片直接以base64字符串传递

### 4.4 通义千问端点

```http
POST /api/v1/services/aigc/text2image/image-synthesis
Content-Type: application/json
Authorization: Bearer sk-xxx

{
  "model": "wanx-v1",
  "input": {
    "prompt": "A beautiful sunset over mountains",
    "ref_img": "https://example.com/reference.jpg"
  },
  "parameters": {
    "size": "1024*1024",
    "n": 1
  }
}
```

---

## 五、关键实现细节总结

### 5.1 多参考图片支持现状

| 组件 | 当前状态 | 扩展方案 |
|------|---------|---------|
| **前端UI** | ✅ 支持最多3张 | 已完成 |
| **前端API类型** | ⚠️ 缺少 `reference_images` 字段 | 需补充接口定义 |
| **后端Handler** | ❌ 仅 `ReferenceImage` 单字段 | 需添加 `ReferenceImages []string` |
| **后端Service** | ❌ 仅处理单张 | 需修改 `ImageGenerationInput` |
| **Gemini提供商** | ✅ 架构支持多图 | 修改 `parts` 数组构建逻辑 |
| **OpenAI标准** | ❌ API限制单图 | 无法扩展 |
| **OpenAI魔改** | ⚠️ 取决于实现 | 需确认API规范 |

### 5.2 分辨率计算规则

**基准分辨率**:
- `1K` → 1024px
- `2K` → 2048px
- `4K` → 4096px

**计算公式**:
```
maxRatio = max(widthRatio, heightRatio)
width = floor(base * widthRatio / maxRatio / 8) * 8
height = floor(base * heightRatio / maxRatio / 8) * 8
```

**示例**:
- `2K` + `16:9` → `2048x1152`
- `1K` + `1:1` → `1024x1024`
- `4K` + `21:9` → `4096x1752`
- `2K` + `3:4` → `1536x2048`

### 5.3 请求端点选择流程

```
用户选择模型ID
    ↓
查询模型配置 (admin_model_settings)
    ↓
解析 request_model_id (如有)
    ↓
获取 request_endpoint 字段
    ↓
映射到渠道类型 (openai/gemini/qwen/sora)
    ↓
查询可用账号 (按平台过滤)
    ↓
过滤支持该模型的账号 (ModelsAllowlist)
    ↓
按优先级排序 → 权重负载均衡
    ↓
选定账号的 BaseURL + APIKey
    ↓
根据端点类型构建请求
```

### 5.4 Base64图片处理

**前端编码**:
```typescript
function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)  // 输出: data:image/png;base64,xxx
  })
}
```

**后端解码**:
```go
func parseReferenceImage(refImage string) ([]byte, string, error) {
    // 1. 解析data URI前缀
    if strings.HasPrefix(refImage, "data:") {
        parts := strings.SplitN(refImage, ",", 2)
        data = parts[1]
        mimeType = extractMimeType(parts[0])
    }

    // 2. Base64解码
    decoded, err := base64.StdEncoding.DecodeString(data)

    return decoded, mimeType, err
}
```

---

## 六、待优化项

### 6.1 类型安全

**问题**: 前端TypeScript接口与实际使用不匹配
```typescript
// 当前定义
export interface ImageGeneratePayload {
  reference_image?: string  // 仅单张
}

// 实际使用
payload.reference_images = referenceImagesBase64  // 未定义
```

**建议**:
```typescript
export interface ImageGeneratePayload {
  reference_image?: string      // 保留向后兼容
  reference_images?: string[]   // 添加多图支持
}
```

### 6.2 后端多图支持

**当前限制**: Handler和Service层仅处理单张参考图

**扩展步骤**:
1. 修改 `ImageGenerateRequest` 添加 `ReferenceImages []string`
2. 修改 `ImageGenerationInput` 添加 `ReferenceImages []string`
3. 更新 `generateGeminiWithClient` 支持多个 `inlineData`
4. 更新 `storeReferenceImage` 支持批量存储

### 6.3 错误处理增强

**建议添加**:
- 参考图片格式验证（MIME类型白名单）
- 图片尺寸限制检查（防止过大图片）
- Base64解码失败的详细错误信息
- 渠道选择失败时的降级策略

---

## 七、配置示例

### 7.1 模型配置示例

```json
[
  {
    "model_id": "DALL-E 3",
    "request_model_id": "dall-e-3",
    "request_endpoint": "openai",
    "model_type": "image",
    "display_name": "DALL-E 3 标准版",
    "resolutions": ["1K"],
    "aspect_ratios": ["Auto", "1:1", "16:9", "9:16"],
    "rpm": 5,
    "rpm_enabled": true
  },
  {
    "model_id": "Gemini 2.0 Flash",
    "request_model_id": "gemini-2.0-flash-exp",
    "request_endpoint": "gemini",
    "model_type": "image",
    "display_name": "Gemini 2.0 Flash 实验版",
    "resolutions": ["1K", "2K", "4K"],
    "aspect_ratios": ["Auto", "1:1", "16:9", "9:16", "3:4", "4:3"],
    "rpm": 15,
    "rpm_enabled": true
  },
  {
    "model_id": "DALL-E 3 HD",
    "request_model_id": "dall-e-3-hd-custom",
    "request_endpoint": "openai_mod",
    "model_type": "image",
    "display_name": "DALL-E 3 高清魔改版",
    "resolutions": ["2K", "4K"],
    "aspect_ratios": ["1:1", "16:9", "21:9"],
    "rpm": 10,
    "rpm_enabled": true
  }
]
```

### 7.2 账号配置示例

```json
{
  "id": 1,
  "platform": "openai",
  "base_url": "https://api.openai.com",
  "api_key": "sk-xxx",
  "priority": 1,
  "weight": 100,
  "models_allowlist": ["dall-e-3", "dall-e-2"],
  "enabled": true
}
```

---

## 八、文档使用说明

### 8.1 如何在新项目中复用

1. **复制核心Service层**:
   - `image_generation_service.go` (核心生成逻辑)
   - `channel_selector.go` (渠道选择)
   - `user_model_settings.go` (模型配置)

2. **适配前端组件**:
   - 参考 `HomeView.vue` 的图片上传和请求构建逻辑
   - 复用 `fileToBase64` 等工具函数

3. **配置数据库表**:
   - `settings` 表存储模型配置
   - `accounts` 表存储API账号信息

4. **环境变量配置**:
   - 各提供商的API Key
   - 对象存储配置（用于保存参考图片）

### 8.2 测试建议

**单元测试重点**:
- Base64编解码正确性
- 分辨率计算精度
- 渠道选择算法（优先级+权重）

**集成测试场景**:
- 不同端点的请求格式验证
- 多参考图片上传和传递
- 错误情况的降级处理

---

## 附录：相关文件清单

### 前端文件
- `frontend/src/views/HomeView.vue` - 主界面和请求构建
- `frontend/src/api/images.ts` - API接口定义
- `frontend/src/views/user/ModelSettingsView.vue` - 模型配置管理

### 后端文件
- `backend/internal/handler/image_generation_handler.go` - HTTP处理器
- `backend/internal/service/image_generation_service.go` - 核心生成服务
- `backend/internal/service/channel_selector.go` - 渠道选择器
- `backend/internal/service/user_model_settings.go` - 模型配置服务
- `backend/internal/service/request_endpoint_metadata.go` - 端点元数据

### 文档文件
- `AI绘画功能技术实现文档.md` - 主文档（2480行）
- `AI绘画功能技术实现补充文档.md` - 本文档

---

**文档完成时间**: 2026-04-26
**总行数**: 约1100行
**覆盖范围**: 多参考图片、模型配置、渠道选择、完整请求示例