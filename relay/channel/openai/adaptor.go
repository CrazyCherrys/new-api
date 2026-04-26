package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/ai360"
	"github.com/QuantumNous/new-api/relay/channel/lingyiwanwu"

	//"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/openrouter"
	"github.com/QuantumNous/new-api/relay/channel/xinference"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/common_handler"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	ChannelType    int
	ResponseFormat string
}

// parseReasoningEffortFromModelSuffix 从模型名称中解析推理级别
// support OAI models: o1-mini/o3-mini/o4-mini/o1/o3 etc...
// minimal effort only available in gpt-5
func parseReasoningEffortFromModelSuffix(model string) (string, string) {
	effortSuffixes := []string{"-high", "-minimal", "-low", "-medium", "-none", "-xhigh"}
	for _, suffix := range effortSuffixes {
		if strings.HasSuffix(model, suffix) {
			effort := strings.TrimPrefix(suffix, "-")
			originModel := strings.TrimSuffix(model, suffix)
			return effort, originModel
		}
	}
	return "", model
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	// 使用 service.GeminiToOpenAIRequest 转换请求格式
	openaiRequest, err := service.GeminiToOpenAIRequest(request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openaiRequest)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	//if !strings.Contains(request.Model, "claude") {
	//	return nil, fmt.Errorf("you are using openai channel type with path /v1/messages, only claude model supported convert, but got %s", request.Model)
	//}
	//if common.DebugEnabled {
	//	bodyBytes := []byte(common.GetJsonString(request))
	//	err := os.WriteFile(fmt.Sprintf("claude_request_%s.txt", c.GetString(common.RequestIdKey)), bodyBytes, 0644)
	//	if err != nil {
	//		println(fmt.Sprintf("failed to save request body to file: %v", err))
	//	}
	//}
	aiRequest, err := service.ClaudeToOpenAIRequest(*request, info)
	if err != nil {
		return nil, err
	}
	//if common.DebugEnabled {
	//	println(fmt.Sprintf("convert claude to openai request result: %s", common.GetJsonString(aiRequest)))
	//	// Save request body to file for debugging
	//	bodyBytes := []byte(common.GetJsonString(aiRequest))
	//	err = os.WriteFile(fmt.Sprintf("claude_to_openai_request_%s.txt", c.GetString(common.RequestIdKey)), bodyBytes, 0644)
	//	if err != nil {
	//		println(fmt.Sprintf("failed to save request body to file: %v", err))
	//	}
	//}
	if info.SupportStreamOptions && info.IsStream {
		aiRequest.StreamOptions = &dto.StreamOptions{
			IncludeUsage: true,
		}
	}
	return a.ConvertOpenAIRequest(c, info, aiRequest)
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType

	// initialize ThinkingContentInfo when thinking_to_content is enabled
	if info.ChannelSetting.ThinkingToContent {
		info.ThinkingContentInfo = relaycommon.ThinkingContentInfo{
			IsFirstThinkingContent:  true,
			SendLastThinkingContent: false,
			HasSentThinkingContent:  false,
		}
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode == relayconstant.RelayModeRealtime {
		if strings.HasPrefix(info.ChannelBaseUrl, "https://") {
			baseUrl := strings.TrimPrefix(info.ChannelBaseUrl, "https://")
			baseUrl = "wss://" + baseUrl
			info.ChannelBaseUrl = baseUrl
		} else if strings.HasPrefix(info.ChannelBaseUrl, "http://") {
			baseUrl := strings.TrimPrefix(info.ChannelBaseUrl, "http://")
			baseUrl = "ws://" + baseUrl
			info.ChannelBaseUrl = baseUrl
		}
	}
	switch info.ChannelType {
	case constant.ChannelTypeAzure:
		apiVersion := info.ApiVersion
		if apiVersion == "" {
			apiVersion = constant.AzureDefaultAPIVersion
		}
		// https://learn.microsoft.com/en-us/azure/cognitive-services/openai/chatgpt-quickstart?pivots=rest-api&tabs=command-line#rest-api
		requestURL := strings.Split(info.RequestURLPath, "?")[0]
		requestURL = fmt.Sprintf("%s?api-version=%s", requestURL, apiVersion)
		task := strings.TrimPrefix(requestURL, "/v1/")

		if info.RelayFormat == types.RelayFormatClaude {
			task = strings.TrimPrefix(task, "messages")
			task = "chat/completions" + task
		}

		// 特殊处理 responses API
		if info.RelayMode == relayconstant.RelayModeResponses {
			responsesApiVersion := "preview"

			subUrl := "/openai/v1/responses"
			if strings.Contains(info.ChannelBaseUrl, "cognitiveservices.azure.com") {
				subUrl = "/openai/responses"
				responsesApiVersion = apiVersion
			}

			if info.ChannelOtherSettings.AzureResponsesVersion != "" {
				responsesApiVersion = info.ChannelOtherSettings.AzureResponsesVersion
			}

			requestURL = fmt.Sprintf("%s?api-version=%s", subUrl, responsesApiVersion)
			return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestURL, info.ChannelType), nil
		}

		model_ := info.UpstreamModelName
		// 2025年5月10日后创建的渠道不移除.
		if info.ChannelCreateTime < constant.AzureNoRemoveDotTime {
			model_ = strings.Replace(model_, ".", "", -1)
		}
		// https://github.com/songquanpeng/one-api/issues/67
		requestURL = fmt.Sprintf("/openai/deployments/%s/%s", model_, task)
		if info.RelayMode == relayconstant.RelayModeRealtime {
			requestURL = fmt.Sprintf("/openai/realtime?deployment=%s&api-version=%s", model_, apiVersion)
		}
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestURL, info.ChannelType), nil
	//case constant.ChannelTypeMiniMax:
	//	return minimax.GetRequestURL(info)
	case constant.ChannelTypeCustom:
		url := info.ChannelBaseUrl
		url = strings.Replace(url, "{model}", info.UpstreamModelName, -1)
		return url, nil
	default:
		if (info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatGemini) &&
			info.RelayMode != relayconstant.RelayModeResponses &&
			info.RelayMode != relayconstant.RelayModeResponsesCompact {
			return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
		}
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	if info.ChannelType == constant.ChannelTypeAzure {
		header.Set("api-key", info.ApiKey)
		return nil
	}
	if info.ChannelType == constant.ChannelTypeOpenAI && "" != info.Organization {
		header.Set("OpenAI-Organization", info.Organization)
	}
	// 检查 Header Override 是否已设置 Authorization，如果已设置则跳过默认设置
	// 这样可以避免在 Header Override 应用时被覆盖（虽然 Header Override 会在之后应用，但这里作为额外保护）
	hasAuthOverride := false
	if len(info.HeadersOverride) > 0 {
		for k := range info.HeadersOverride {
			if strings.EqualFold(k, "Authorization") {
				hasAuthOverride = true
				break
			}
		}
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		swp := c.Request.Header.Get("Sec-WebSocket-Protocol")
		if swp != "" {
			items := []string{
				"realtime",
				"openai-insecure-api-key." + info.ApiKey,
				"openai-beta.realtime-v1",
			}
			header.Set("Sec-WebSocket-Protocol", strings.Join(items, ","))
			//req.Header.Set("Sec-WebSocket-Key", c.Request.Header.Get("Sec-WebSocket-Key"))
			//req.Header.Set("Sec-Websocket-Extensions", c.Request.Header.Get("Sec-Websocket-Extensions"))
			//req.Header.Set("Sec-Websocket-Version", c.Request.Header.Get("Sec-Websocket-Version"))
		} else {
			header.Set("openai-beta", "realtime=v1")
			if !hasAuthOverride {
				header.Set("Authorization", "Bearer "+info.ApiKey)
			}
		}
	} else {
		if !hasAuthOverride {
			header.Set("Authorization", "Bearer "+info.ApiKey)
		}
	}
	if info.ChannelType == constant.ChannelTypeOpenRouter {
		if header.Get("HTTP-Referer") == "" {
			header.Set("HTTP-Referer", "https://www.newapi.ai")
		}
		if header.Get("X-OpenRouter-Title") == "" {
			header.Set("X-OpenRouter-Title", "New API")
		}
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if info.ChannelType != constant.ChannelTypeOpenAI && info.ChannelType != constant.ChannelTypeAzure {
		request.StreamOptions = nil
	}
	if info.ChannelType == constant.ChannelTypeOpenRouter {
		if len(request.Usage) == 0 {
			request.Usage = json.RawMessage(`{"include":true}`)
		}
		// 适配 OpenRouter 的 thinking 后缀
		if !model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) &&
			strings.HasSuffix(info.UpstreamModelName, "-thinking") {
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
			request.Model = info.UpstreamModelName
			if len(request.Reasoning) == 0 {
				reasoning := map[string]any{
					"enabled": true,
				}
				if request.ReasoningEffort != "" && request.ReasoningEffort != "none" {
					reasoning["effort"] = request.ReasoningEffort
				}
				marshal, err := common.Marshal(reasoning)
				if err != nil {
					return nil, fmt.Errorf("error marshalling reasoning: %w", err)
				}
				request.Reasoning = marshal
			}
			// 清空多余的ReasoningEffort
			request.ReasoningEffort = ""
		} else {
			if len(request.Reasoning) == 0 {
				// 适配 OpenAI 的 ReasoningEffort 格式
				if request.ReasoningEffort != "" {
					reasoning := map[string]any{
						"enabled": true,
					}
					if request.ReasoningEffort != "none" {
						reasoning["effort"] = request.ReasoningEffort
						marshal, err := common.Marshal(reasoning)
						if err != nil {
							return nil, fmt.Errorf("error marshalling reasoning: %w", err)
						}
						request.Reasoning = marshal
					}
				}
			}
			request.ReasoningEffort = ""
		}

		// https://docs.anthropic.com/en/api/openai-sdk#extended-thinking-support
		// 没有做排除3.5Haiku等，要出问题再加吧，最佳兼容性（不是
		if request.THINKING != nil && strings.HasPrefix(info.UpstreamModelName, "anthropic") {
			var thinking dto.Thinking // Claude标准Thinking格式
			if err := json.Unmarshal(request.THINKING, &thinking); err != nil {
				return nil, fmt.Errorf("error Unmarshal thinking: %w", err)
			}

			// 只有当 thinking.Type 是 "enabled" 时才处理
			if thinking.Type == "enabled" {
				// 检查 BudgetTokens 是否为 nil
				if thinking.BudgetTokens == nil {
					return nil, fmt.Errorf("BudgetTokens is nil when thinking is enabled")
				}

				reasoning := openrouter.RequestReasoning{
					Enabled:   true,
					MaxTokens: *thinking.BudgetTokens,
				}

				marshal, err := common.Marshal(reasoning)
				if err != nil {
					return nil, fmt.Errorf("error marshalling reasoning: %w", err)
				}

				request.Reasoning = marshal
			}

			// 清空 THINKING
			request.THINKING = nil
		}

	}
	if strings.HasPrefix(info.UpstreamModelName, "o") || strings.HasPrefix(info.UpstreamModelName, "gpt-5") {
		if lo.FromPtrOr(request.MaxCompletionTokens, uint(0)) == 0 && lo.FromPtrOr(request.MaxTokens, uint(0)) != 0 {
			request.MaxCompletionTokens = request.MaxTokens
			request.MaxTokens = nil
		}

		if strings.HasPrefix(info.UpstreamModelName, "o") {
			request.Temperature = nil
		}

		// gpt-5系列模型适配 归零不再支持的参数
		if strings.HasPrefix(info.UpstreamModelName, "gpt-5") {
			request.Temperature = nil
			request.TopP = nil
			request.LogProbs = nil
		}

		// 转换模型推理力度后缀
		effort, originModel := parseReasoningEffortFromModelSuffix(info.UpstreamModelName)
		if effort != "" {
			request.ReasoningEffort = effort
			info.UpstreamModelName = originModel
			request.Model = originModel
		}

		info.ReasoningEffort = request.ReasoningEffort

		// o系列模型developer适配（o1-mini除外）
		if !strings.HasPrefix(info.UpstreamModelName, "o1-mini") && !strings.HasPrefix(info.UpstreamModelName, "o1-preview") {
			//修改第一个Message的内容，将system改为developer
			if len(request.Messages) > 0 && request.Messages[0].Role == "system" {
				request.Messages[0].Role = "developer"
			}
		}
	}

	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	a.ResponseFormat = request.ResponseFormat
	if info.RelayMode == relayconstant.RelayModeAudioSpeech {
		jsonData, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("error marshalling object: %w", err)
		}
		return bytes.NewReader(jsonData), nil
	} else {
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		writer.WriteField("model", request.Model)

		formData, err2 := common.ParseMultipartFormReusable(c)
		if err2 != nil {
			return nil, fmt.Errorf("error parsing multipart form: %w", err2)
		}

		// 打印类似 curl 命令格式的信息
		logger.LogDebug(c.Request.Context(), fmt.Sprintf("--form 'model=\"%s\"'", request.Model))

		// 遍历表单字段并打印输出
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, value := range values {
				writer.WriteField(key, value)
				logger.LogDebug(c.Request.Context(), fmt.Sprintf("--form '%s=\"%s\"'", key, value))
			}
		}

		// 从 formData 中获取文件
		fileHeaders := formData.File["file"]
		if len(fileHeaders) == 0 {
			return nil, errors.New("file is required")
		}

		// 使用 formData 中的第一个文件
		fileHeader := fileHeaders[0]
		logger.LogDebug(c.Request.Context(), fmt.Sprintf("--form 'file=@\"%s\"' (size: %d bytes, content-type: %s)",
			fileHeader.Filename, fileHeader.Size, fileHeader.Header.Get("Content-Type")))

		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("error opening audio file: %v", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", fileHeader.Filename)
		if err != nil {
			return nil, errors.New("create form file failed")
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, errors.New("copy file failed")
		}

		// 关闭 multipart 编写器以设置分界线
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		logger.LogDebug(c.Request.Context(), fmt.Sprintf("--header 'Content-Type: %s'", writer.FormDataContentType()))
		return &requestBody, nil
	}
}

// calculateOpenAIPixelSize 将 resolution + aspect_ratio 映射到 gpt-image-2 / gpt-image-1
// 的合法预设尺寸。
//
// 合法预设（ref: https://docs.newapi.ai/zh/docs/api/ai-model/images/openai/post-v1-images-generations）：
//
//	1024x1024 · 1536x1024 · 1024x1536 · 2048x2048 · 2048x1152 · 3840x2160 · 2160x3840
//
// 此函数仅用于标准 "openai" 端点，不适用于 openai_mod（原样透传）或 gemini（独立处理）。
func calculateOpenAIPixelSize(resolution, aspectRatio string) string {
	if resolution == "" || aspectRatio == "" {
		return ""
	}

	res := strings.ToUpper(strings.TrimSpace(resolution))
	ar := strings.TrimSpace(aspectRatio)

	// 直查表：只输出官方预设尺寸，拒绝一切非预设值。
	// 横向优先比例（landscape）→ 横版预设；纵向优先比例 → 竖版预设；方形 → 正方预设。
	switch res {
	case "1K":
		switch ar {
		case "1:1", "4:3", "3:4":
			return "1024x1024"
		case "3:2", "16:9", "21:9":
			return "1536x1024"
		case "2:3", "9:16", "9:21":
			return "1024x1536"
		}
	case "2K":
		switch ar {
		case "1:1", "4:3":
			return "2048x2048"
		case "3:2", "16:9", "21:9":
			return "2048x1152"
		case "2:3", "9:16", "3:4", "9:21":
			return "2160x3840"
		}
	case "4K":
		switch ar {
		case "1:1", "4:3", "3:4":
			return "2048x2048" // 无 4K 正方预设，回退到最大正方预设
		case "3:2", "16:9", "21:9":
			return "3840x2160"
		case "2:3", "9:16", "9:21":
			return "2160x3840"
		}
	}

	// 不支持的组合返回空字符串，上游将使用默认 auto 尺寸
	return ""
}

// StandardOpenAIImageRequest 是「openai」标准端点（经 /console/model-mapping 配置）
// 发往上游的精简 JSON 结构，仅包含 4 个字段：
//
//	{ prompt, model, size, image[] }
//
// - size：从合法预设中选取（1024x1024 / 1536x1024 / 1024x1536 / 2048x2048 /
//   2048x1152 / 3840x2160 / 2160x3840 / auto），由 calculateOpenAIPixelSize 映射。
// - image：参考图数组（URL 或 base64 data URL）。无参考图时省略整个字段。
//
// 该结构与 openai_mod / gemini 端点的请求体完全独立，互不影响。
type StandardOpenAIImageRequest struct {
	Prompt string   `json:"prompt"`
	Model  string   `json:"model"`
	Size   string   `json:"size,omitempty"`
	Image  []string `json:"image,omitempty"`
}

// convertOpenAIStandardJSONImageRequest 处理 request_endpoint == "openai" 的图片请求。
//
// 行为：
//  1. 始终输出 application/json，不使用 multipart/form-data。
//  2. body 仅包含 prompt/model/size/image 四个字段，其它（n、quality、response_format、
//     extra_fields、watermark 等）一律剔除。
//  3. size 解析顺序：客户端显式 Size > (resolution, aspect_ratio) 映射 > "auto"。
//  4. image 来源：
//     - 优先 ReferenceImages（service 层组装的 base64 data URL / http URL）；
//     - 兼容直接以 multipart 上传的 file → 转为 base64 data URL；
//     - 兼容 ImageRequest.Image（json 字段）为字符串 / 字符串数组的写法。
//     无任何参考图时 image 字段省略。
func (a *Adaptor) convertOpenAIStandardJSONImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	out := StandardOpenAIImageRequest{
		Prompt: request.Prompt,
		Model:  request.Model,
	}

	out.Size = resolveStandardOpenAIImageSize(request)

	images, err := collectStandardOpenAIImageInputs(c, request)
	if err != nil {
		return nil, err
	}
	if len(images) > 0 {
		out.Image = images
	}

	if c != nil && c.Request != nil {
		c.Request.Header.Set("Content-Type", "application/json")
	}

	return out, nil
}

// resolveStandardOpenAIImageSize 解析最终发往上游的 size 字符串：
//   - 客户端显式 size 优先（已是合法预设由调用方负责）；
//   - 否则用 (resolution, aspect_ratio) 通过 calculateOpenAIPixelSize 映射；
//   - aspect_ratio 显式为 "auto" 时返回 "auto"；
//   - 都没有命中合法预设则返回 "auto"（满足上游 schema 中 size 的 default = auto）。
func resolveStandardOpenAIImageSize(request dto.ImageRequest) string {
	if size := strings.TrimSpace(request.Size); size != "" {
		return size
	}

	aspectRatio := strings.TrimSpace(request.AspectRatio)
	resolution := strings.TrimSpace(request.Resolution)
	if request.RawParams != nil {
		if aspectRatio == "" {
			if v, ok := request.RawParams["aspect_ratio"].(string); ok {
				aspectRatio = strings.TrimSpace(v)
			}
		}
		if resolution == "" {
			if v, ok := request.RawParams["resolution"].(string); ok {
				resolution = strings.TrimSpace(v)
			}
		}
	}

	if strings.EqualFold(aspectRatio, "auto") {
		return "auto"
	}
	if aspectRatio != "" && resolution != "" {
		if mapped := calculateOpenAIPixelSize(resolution, aspectRatio); mapped != "" {
			return mapped
		}
	}

	return "auto"
}

// collectStandardOpenAIImageInputs 汇总参考图数组，输出顺序：
//  1. ReferenceImages（service 层主路径）
//  2. multipart/form-data 中的 image / image[] / image[N] file → base64 data URL
//  3. ImageRequest.Image（兼容 string / []string 两种写法）
//
// 所有数据 URL / URL 字符串原样保留，不做尺寸或格式校验（上游负责）。
func collectStandardOpenAIImageInputs(c *gin.Context, request dto.ImageRequest) ([]string, error) {
	images := append([]string(nil), request.ReferenceImages...)

	if c != nil && c.Request != nil {
		mf := c.Request.MultipartForm
		if mf == nil {
			contentType := c.Request.Header.Get("Content-Type")
			if strings.Contains(contentType, "multipart/form-data") {
				if err := c.Request.ParseMultipartForm(32 << 20); err == nil {
					mf = c.Request.MultipartForm
				}
			}
		}
		if mf != nil && mf.File != nil {
			collected, err := collectMultipartImagesAsDataURL(mf)
			if err != nil {
				return nil, err
			}
			images = append(images, collected...)
		}
	}

	if len(request.Image) > 0 {
		var single string
		if err := common.Unmarshal(request.Image, &single); err == nil && single != "" {
			images = append(images, single)
		} else {
			var multi []string
			if err := common.Unmarshal(request.Image, &multi); err == nil {
				for _, s := range multi {
					if s = strings.TrimSpace(s); s != "" {
						images = append(images, s)
					}
				}
			}
		}
	}

	return images, nil
}

// collectMultipartImagesAsDataURL 将 multipart 中的 image / image[] / image[N] 文件
// 全部读取为 base64 data URL，便于以 JSON 数组形式发往上游。
func collectMultipartImagesAsDataURL(mf *multipart.Form) ([]string, error) {
	var headers []*multipart.FileHeader
	if files, ok := mf.File["image"]; ok {
		headers = append(headers, files...)
	}
	if files, ok := mf.File["image[]"]; ok {
		headers = append(headers, files...)
	}
	for name, files := range mf.File {
		if strings.HasPrefix(name, "image[") && name != "image[]" {
			headers = append(headers, files...)
		}
	}

	out := make([]string, 0, len(headers))
	for i, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open image file %d: %w", i, err)
		}
		buf, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read image file %d: %w", i, err)
		}
		mimeType := detectImageMimeType(fh.Filename)
		out = append(out, "data:"+mimeType+";base64,"+base64.StdEncoding.EncodeToString(buf))
	}
	return out, nil
}

// OpenAIModImageRequest represents the text-to-image request for OpenAI modified endpoints.
// Only model, prompt and image_config (aspect_ratio / image_size) are sent upstream.
type OpenAIModImageRequest struct {
	Model       string                `json:"model"`
	Prompt      string                `json:"prompt"`
	ImageConfig *OpenAIModImageConfig `json:"image_config,omitempty"`
}

// OpenAIModImageEditRequest represents the image-to-image request for OpenAI modified endpoints.
// image field accepts a single URL/data-URI string or an array of strings.
// Only model, image, prompt and image_config are sent upstream.
type OpenAIModImageEditRequest struct {
	Model       string                `json:"model"`
	Image       interface{}           `json:"image"` // string or []string
	Prompt      string                `json:"prompt"`
	ImageConfig *OpenAIModImageConfig `json:"image_config,omitempty"`
}

// OpenAIModImageConfig represents the image_config object for OpenAI modified endpoints
type OpenAIModImageConfig struct {
	ImageSize   string `json:"image_size,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

// convertOpenAIModImageRequest converts the standard ImageRequest to OpenAI modified endpoint format.
// Only model, prompt and image_config (aspect_ratio / image_size) are forwarded upstream.
func (a *Adaptor) convertOpenAIModImageRequest(request dto.ImageRequest) (any, error) {
	modRequest := OpenAIModImageRequest{
		Model:  request.Model,
		Prompt: request.Prompt,
	}

	// 优先读取正式 JSON 字段（序列化安全），回退到 RawParams（兼容进程内直调路径）
	resolution := request.Resolution
	aspectRatio := request.AspectRatio

	if resolution == "" && request.RawParams != nil {
		if r, ok := request.RawParams["resolution"].(string); ok {
			resolution = r
		}
	}
	if aspectRatio == "" && request.RawParams != nil {
		if a, ok := request.RawParams["aspect_ratio"].(string); ok {
			aspectRatio = a
		}
	}

	if resolution != "" || aspectRatio != "" {
		modRequest.ImageConfig = &OpenAIModImageConfig{
			ImageSize:   resolution,
			AspectRatio: aspectRatio,
		}
	}

	return modRequest, nil
}

// convertOpenAIModImageEditRequest converts the standard ImageRequest to the OpenAI modified
// image-to-image (edits) endpoint format. The upstream expects JSON with:
//
//	{ model, image: string|string[], prompt, image_config? }
func (a *Adaptor) convertOpenAIModImageEditRequest(request dto.ImageRequest) (any, error) {
	editRequest := OpenAIModImageEditRequest{
		Model:  request.Model,
		Prompt: request.Prompt,
	}

	// image_config
	resolution := request.Resolution
	aspectRatio := request.AspectRatio
	if resolution == "" && request.RawParams != nil {
		if r, ok := request.RawParams["resolution"].(string); ok {
			resolution = r
		}
	}
	if aspectRatio == "" && request.RawParams != nil {
		if ap, ok := request.RawParams["aspect_ratio"].(string); ok {
			aspectRatio = ap
		}
	}
	if resolution != "" || aspectRatio != "" {
		editRequest.ImageConfig = &OpenAIModImageConfig{
			ImageSize:   resolution,
			AspectRatio: aspectRatio,
		}
	}

	// Collect images from ReferenceImages first; fall back to request.Image (json.RawMessage)
	images := request.ReferenceImages
	if len(images) == 0 && len(request.Image) > 0 {
		var imgStr string
		if err := common.Unmarshal(request.Image, &imgStr); err == nil && imgStr != "" {
			images = []string{imgStr}
		}
	}

	if len(images) == 0 {
		return nil, errors.New("image is required for openai_mod image edit")
	}
	if len(images) == 1 {
		editRequest.Image = images[0]
	} else {
		editRequest.Image = images
	}

	return editRequest, nil
}

// ConvertImageRequest 按 request_endpoint 分发到独立的转换逻辑：
//
//   - "openai_mod" + RelayModeImagesEdits → convertOpenAIModImageEditRequest
//     image 字段为 string|[]string，整体以 JSON 发送（非 multipart form）
//
//   - "openai_mod" + RelayModeImagesGenerations → convertOpenAIModImageRequest
//     resolution/aspect_ratio 原样写入 image_config（上游自行解释 "1K"/"2K"/"4K"）
//
//   - "openai" → convertOpenAIStandardJSONImageRequest（精简 JSON 协议）
//     仅发送 {prompt, model, size, image[]} 四个字段，全部走 /v1/images/generations。
//     resolution + aspect_ratio → calculateOpenAIPixelSize → WxH 像素字符串（如 "2048x1152"）；
//     映射不到合法预设时回退 size="auto"。无参考图时省略 image 字段。
//
//   - ""（空）→ convertStandardOpenAIImageRequest（兼容标准 OpenAI SDK 直调）
//     保留 multipart/form-data + 全字段 JSON 的原行为，避免破坏外部 SDK 调用。
//
//   - "gemini" 端点不经过此适配器，由 relay/channel/gemini 独立处理：
//     resolution 原样映射为 Gemini imageSize（"1K"/"2K"/"4K"），上游自行解释像素含义
//
// 三条路径的 resolution 含义相互独立，切勿共用像素换算表。
func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	switch request.RequestEndpoint {
	case "openai_mod":
		if info.RelayMode == relayconstant.RelayModeImagesEdits {
			// 图生图：以 JSON 发送，上游 endpoint 为 /v1/images/edits
			return a.convertOpenAIModImageEditRequest(request)
		}
		// 文生图：resolution/aspect_ratio 原样透传至 image_config，上游负责解释
		return a.convertOpenAIModImageRequest(request)
	case "openai":
		// 标准 OpenAI 端点（由 /console/model-mapping 配置触发）：
		// 始终以精简 JSON 发送到 /v1/images/generations。
		return a.convertOpenAIStandardJSONImageRequest(c, info, request)
	default:
		// 兼容外部 OpenAI SDK 直调：默认行为不变，使用 multipart 或全字段 JSON
		return a.convertStandardOpenAIImageRequest(c, info, request)
	}
}

// convertStandardOpenAIImageRequest 处理「未指定 request_endpoint」的标准 OpenAI 兼容请求。
//
// 该函数现在仅服务于直接调用 /v1/images/{generations,edits} 的外部 OpenAI SDK 路径，
// 不再用于 /image-generation 内部图床流程（后者通过 request_endpoint == "openai" 走
// convertOpenAIStandardJSONImageRequest 的精简 JSON 协议）。
//
// 行为保持不变：
//   - RelayModeImagesEdits：将 multipart/form-data（含 image/image[] 文件 + 文本字段）
//     重新打包为新的 multipart 请求体；
//   - RelayModeImagesGenerations：原样透传 ImageRequest 结构体（含 n / quality 等全字段）。
//
// 若 Size 未设置，则通过 calculateOpenAIPixelSize 将 resolution + aspect_ratio 换算为像素字符串。
// 像素映射表以 gpt-image-2 规格为准，与 openai_mod / gemini 端点的换算表完全独立。
func (a *Adaptor) convertStandardOpenAIImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	// 若 Size 未设置，尝试从 aspect_ratio + resolution 换算（openai 端点专用逻辑）
	if request.Size == "" {
		aspectRatio := request.AspectRatio
		resolution := request.Resolution

		if aspectRatio == "" && request.RawParams != nil {
			aspectRatio, _ = request.RawParams["aspect_ratio"].(string)
		}
		if resolution == "" && request.RawParams != nil {
			resolution, _ = request.RawParams["resolution"].(string)
		}

		if aspectRatio != "" && resolution != "" {
			if calculatedSize := calculateOpenAIPixelSize(resolution, aspectRatio); calculatedSize != "" {
				request.Size = calculatedSize
			}
		}
	}

	switch info.RelayMode {
	case relayconstant.RelayModeImagesEdits:

		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		writer.WriteField("model", request.Model)
		// 使用已解析的 multipart 表单，避免重复解析
		mf := c.Request.MultipartForm
		if mf == nil {
			if _, err := c.MultipartForm(); err != nil && len(request.ReferenceImages) == 0 {
				return nil, errors.New("failed to parse multipart form")
			}
			mf = c.Request.MultipartForm
		}

		if mf != nil {
			// 写入所有非文件字段
			for key, values := range mf.Value {
				if key == "model" {
					continue
				}
				for _, value := range values {
					writer.WriteField(key, value)
				}
			}

			if mf.File != nil {
				// Check if "image" field exists in any form, including array notation
				var imageFiles []*multipart.FileHeader
				var exists bool

				// First check for standard "image" field
				if imageFiles, exists = mf.File["image"]; !exists || len(imageFiles) == 0 {
					// If not found, check for "image[]" field
					if imageFiles, exists = mf.File["image[]"]; !exists || len(imageFiles) == 0 {
						// If still not found, iterate through all fields to find any that start with "image["
						foundArrayImages := false
						for fieldName, files := range mf.File {
							if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
								foundArrayImages = true
								imageFiles = append(imageFiles, files...)
							}
						}

						// If no image fields found at all
						if !foundArrayImages && (len(imageFiles) == 0) {
							return nil, errors.New("image is required")
						}
					}
				}

				// Process all image files
				for i, fileHeader := range imageFiles {
					file, err := fileHeader.Open()
					if err != nil {
						return nil, fmt.Errorf("failed to open image file %d: %w", i, err)
					}

					// If multiple images, use image[] as the field name
					fieldName := "image"
					if len(imageFiles) > 1 {
						fieldName = "image[]"
					}

					// Determine MIME type based on file extension
					mimeType := detectImageMimeType(fileHeader.Filename)

					// Create a form file with the appropriate content type
					h := make(textproto.MIMEHeader)
					h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileHeader.Filename))
					h.Set("Content-Type", mimeType)

					part, err := writer.CreatePart(h)
					if err != nil {
						return nil, fmt.Errorf("create form part failed for image %d: %w", i, err)
					}

					if _, err := io.Copy(part, file); err != nil {
						return nil, fmt.Errorf("copy file failed for image %d: %w", i, err)
					}

					// 复制完立即关闭，避免在循环内使用 defer 占用资源
					_ = file.Close()
				}

				// Handle mask file if present
				if maskFiles, exists := mf.File["mask"]; exists && len(maskFiles) > 0 {
					maskFile, err := maskFiles[0].Open()
					if err != nil {
						return nil, errors.New("failed to open mask file")
					}
					// 复制完立即关闭，避免在循环内使用 defer 占用资源

					// Determine MIME type for mask file
					mimeType := detectImageMimeType(maskFiles[0].Filename)

					// Create a form file with the appropriate content type
					h := make(textproto.MIMEHeader)
					h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="mask"; filename="%s"`, maskFiles[0].Filename))
					h.Set("Content-Type", mimeType)

					maskPart, err := writer.CreatePart(h)
					if err != nil {
						return nil, errors.New("create form file failed for mask")
					}

					if _, err := io.Copy(maskPart, maskFile); err != nil {
						return nil, errors.New("copy mask file failed")
					}
					_ = maskFile.Close()
				}
			} else {
				return nil, errors.New("no multipart form data found")
			}
		} else if len(request.ReferenceImages) > 0 {
			// JSON 内部调用路径：service 层以 JSON body 发送，参考图为 base64 data URL。
			// 将其转换为 multipart/form-data 以满足 gpt-image-2 /v1/images/edits 规范。
			if request.Prompt != "" {
				writer.WriteField("prompt", request.Prompt)
			}
			if request.Size != "" {
				writer.WriteField("size", request.Size)
			}
			if request.Quality != "" {
				writer.WriteField("quality", request.Quality)
			}
			// 将 json.RawMessage 字段（output_format / output_compression / background）写为文本字段
			if s := rawMessageToString(request.OutputFormat); s != "" {
				writer.WriteField("output_format", s)
			}
			if s := rawMessageToString(request.OutputCompression); s != "" {
				writer.WriteField("output_compression", s)
			}
			if s := rawMessageToString(request.Background); s != "" {
				writer.WriteField("background", s)
			}

			fieldName := "image"
			if len(request.ReferenceImages) > 1 {
				fieldName = "image[]"
			}
			for i, dataURL := range request.ReferenceImages {
				imgBytes, mimeType, err := parseDataURLToBytes(dataURL)
				if err != nil {
					return nil, fmt.Errorf("invalid reference image %d: %w", i, err)
				}
				ext := mimeTypeToExt(mimeType)
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="image%d.%s"`, fieldName, i, ext))
				h.Set("Content-Type", mimeType)
				part, err := writer.CreatePart(h)
				if err != nil {
					return nil, fmt.Errorf("create form part failed for image %d: %w", i, err)
				}
				if _, err := part.Write(imgBytes); err != nil {
					return nil, fmt.Errorf("write image data failed for image %d: %w", i, err)
				}
			}
		} else {
			return nil, errors.New("no multipart form data found")
		}

		// 关闭 multipart 编写器以设置分界线
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &requestBody, nil

	default:
		return request, nil
	}
}

// detectImageMimeType determines the MIME type based on the file extension
func detectImageMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		// Try to detect from extension if possible
		if strings.HasPrefix(ext, ".jp") {
			return "image/jpeg"
		}
		// Default to png as a fallback
		return "image/png"
	}
}

// parseDataURLToBytes 解析 "data:image/png;base64,..." 格式的 data URL，
// 返回原始字节和 MIME type。若传入的是纯 base64 字符串则默认视为 image/png。
func parseDataURLToBytes(dataURL string) ([]byte, string, error) {
	if !strings.HasPrefix(dataURL, "data:") {
		decoded, err := base64.StdEncoding.DecodeString(dataURL)
		if err != nil {
			return nil, "", fmt.Errorf("invalid base64 string: %w", err)
		}
		return decoded, "image/png", nil
	}
	rest := strings.TrimPrefix(dataURL, "data:")
	semicolon := strings.Index(rest, ";")
	if semicolon == -1 {
		return nil, "", fmt.Errorf("invalid data URL: missing semicolon")
	}
	mimeType := rest[:semicolon]
	rest = rest[semicolon+1:]
	comma := strings.Index(rest, ",")
	if comma == -1 {
		return nil, "", fmt.Errorf("invalid data URL: missing comma")
	}
	if rest[:comma] != "base64" {
		return nil, "", fmt.Errorf("unsupported data URL encoding: %s", rest[:comma])
	}
	b64 := rest[comma+1:]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, "", fmt.Errorf("base64 decode failed: %w", err)
	}
	return decoded, mimeType, nil
}

// mimeTypeToExt 将常见图片 MIME type 转为文件扩展名。
func mimeTypeToExt(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

// rawMessageToString 将 json.RawMessage 字段（如 `"jpeg"` 或 `85`）转为无引号字符串，
// 用于把 dto.ImageRequest 的 RawMessage 字段写入 multipart form 文本字段。
func rawMessageToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	// 去除 JSON 字符串两端的引号
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	//  转换模型推理力度后缀
	effort, originModel := parseReasoningEffortFromModelSuffix(request.Model)
	if effort != "" {
		if request.Reasoning == nil {
			request.Reasoning = &dto.Reasoning{
				Effort: effort,
			}
		} else {
			request.Reasoning.Effort = effort
		}
		request.Model = originModel
	}
	if info != nil && request.Reasoning != nil && request.Reasoning.Effort != "" {
		info.ReasoningEffort = request.Reasoning.Effort
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation {
		return channel.DoFormRequest(a, c, info, requestBody)
	} else if info.RelayMode == relayconstant.RelayModeImagesEdits {
		// openai_mod 图生图以 JSON 发送；标准 OpenAI edits 以 multipart form 发送。
		// convertStandardOpenAIImageRequest 会显式将 Content-Type 设为 multipart/form-data，
		// 而 convertOpenAIModImageEditRequest 不会修改 Content-Type（保持 application/json）。
		if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
			return channel.DoFormRequest(a, c, info, requestBody)
		}
		return channel.DoApiRequest(a, c, info, requestBody)
	} else if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	} else {
		return channel.DoApiRequest(a, c, info, requestBody)
	}
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeRealtime:
		err, usage = OpenaiRealtimeHandler(c, info)
	case relayconstant.RelayModeAudioSpeech:
		usage = OpenaiTTSHandler(c, resp, info)
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err, usage = OpenaiSTTHandler(c, resp, info, a.ResponseFormat)
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		usage, err = OpenaiHandlerWithUsage(c, info, resp)
	case relayconstant.RelayModeRerank:
		usage, err = common_handler.RerankHandler(c, info, resp)
	case relayconstant.RelayModeResponses:
		if info.IsStream {
			usage, err = OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = OaiResponsesHandler(c, info, resp)
		}
	case relayconstant.RelayModeResponsesCompact:
		usage, err = OaiResponsesCompactionHandler(c, resp)
	default:
		if info.IsStream {
			usage, err = OaiStreamHandler(c, info, resp)
		} else {
			usage, err = OpenaiHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	switch a.ChannelType {
	case constant.ChannelType360:
		return ai360.ModelList
	case constant.ChannelTypeLingYiWanWu:
		return lingyiwanwu.ModelList
	//case constant.ChannelTypeMiniMax:
	//	return minimax.ModelList
	case constant.ChannelTypeXinference:
		return xinference.ModelList
	case constant.ChannelTypeOpenRouter:
		return openrouter.ModelList
	default:
		return ModelList
	}
}

func (a *Adaptor) GetChannelName() string {
	switch a.ChannelType {
	case constant.ChannelType360:
		return ai360.ChannelName
	case constant.ChannelTypeLingYiWanWu:
		return lingyiwanwu.ChannelName
	//case constant.ChannelTypeMiniMax:
	//	return minimax.ChannelName
	case constant.ChannelTypeXinference:
		return xinference.ChannelName
	case constant.ChannelTypeOpenRouter:
		return openrouter.ChannelName
	default:
		return ChannelName
	}
}
