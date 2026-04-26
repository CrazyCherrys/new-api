package gemini

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// parseDataURL 解析 Data URL 字符串，返回 mimeType 和纯 base64 数据。
// 同时兼容 "data:image/png;base64,xxx" 和裸 base64 字符串。
func parseDataURL(s string) (mimeType string, base64Data string) {
	if strings.HasPrefix(s, "data:") {
		// 格式: data:<mimeType>;base64,<data>
		s = strings.TrimPrefix(s, "data:")
		if idx := strings.Index(s, ","); idx >= 0 {
			header := s[:idx]
			base64Data = s[idx+1:]
			// header 形如 "image/png;base64"
			if semi := strings.Index(header, ";"); semi >= 0 {
				mimeType = header[:semi]
			} else {
				mimeType = header
			}
		}
	} else {
		// 裸 base64，默认 MIME 类型为 image/png
		mimeType = "image/png"
		base64Data = s
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return
}

type Adaptor struct {
}

// useImagenPredictAPI 判断该模型应当走 Imagen 旧版 :predict 接口
// （body 为 instances/parameters；响应为 predictions[]），
// 否则一律走新版 :generateContent 接口
// （body 为 contents/generationConfig.imageConfig；响应为 candidates[].content.parts[].inlineData）。
//
// 判定规则：
//  1. 模型名以 "imagen" 开头 → :predict（如 imagen-3.0-generate-001）。
//  2. 否则 → :generateContent（覆盖 gemini-2.5-flash-image / gemini-3-pro-image 等所有
//     "Gemini 原生图像生成" 模型）。
func useImagenPredictAPI(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "imagen")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if len(request.Contents) > 0 {
		for i, content := range request.Contents {
			if i == 0 {
				if request.Contents[0].Role == "" {
					request.Contents[0].Role = "user"
				}
			}
			for _, part := range content.Parts {
				if part.FileData != nil {
					if part.FileData.MimeType == "" && strings.Contains(part.FileData.FileUri, "www.youtube.com") {
						part.FileData.MimeType = "video/webm"
					}
				}
			}
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	oaiReq, err := adaptor.ConvertClaudeRequest(c, info, req)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, oaiReq.(*dto.GeneralOpenAIRequest))
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	// 解析通用参数：把 OpenAI 风格的 size/quality/n 翻译成 Gemini 的 aspectRatio/imageSize/sampleCount。
	// 优先级：
	//   1. ImageRequest.AspectRatio / .Resolution（JSON 字段，序列化安全，覆盖所有调用路径）
	//   2. RawParams["aspect_ratio"] / ["resolution"]（进程内直调路径的遗留兼容）
	//   3. Size / Quality（OpenAI 兼容回退）
	aspectRatio := ""
	imageSize := ""

	if request.AspectRatio != "" {
		aspectRatio = normalizeAspectRatio(request.AspectRatio)
	}
	if request.Resolution != "" {
		imageSize = normalizeImageSize(request.Resolution)
	}

	if aspectRatio == "" && request.RawParams != nil {
		if ar, ok := request.RawParams["aspect_ratio"].(string); ok && ar != "" {
			aspectRatio = normalizeAspectRatio(ar)
		}
	}
	if imageSize == "" && request.RawParams != nil {
		if res, ok := request.RawParams["resolution"].(string); ok && res != "" {
			imageSize = normalizeImageSize(res)
		}
	}

	// 回退到 Size/Quality（向后兼容）
	if aspectRatio == "" && request.Size != "" {
		aspectRatio = normalizeAspectRatio(request.Size)
	}
	if imageSize == "" && request.Quality != "" {
		imageSize = normalizeImageSize(request.Quality)
	}

	sampleCount := 1
	if request.N != nil && *request.N > 0 {
		sampleCount = int(*request.N)
	}

	// 1. Imagen 系列：使用旧版 :predict 接口，body 为 instances/parameters。
	// Imagen 支持的 imageSize：仅 "1K" / "2K"（normalizeImageSize 已做降级处理）
	// Imagen 支持的 aspectRatio：仅 "1:1" / "3:4" / "4:3" / "9:16" / "16:9"（normalizeAspectRatioForImagen 映射）
	if useImagenPredictAPI(info.UpstreamModelName) {
		return dto.GeminiImageRequest{
			Instances: []dto.GeminiImageInstance{
				{Prompt: request.Prompt},
			},
			Parameters: dto.GeminiImageParameters{
				SampleCount: sampleCount,
				AspectRatio: normalizeAspectRatioForImagen(aspectRatio),
				ImageSize:   imageSize,
			},
		}, nil
	}

	// 2. Gemini 原生图像生成：使用 :generateContent，body 为 contents + generationConfig.imageConfig。
	//
	// Gemini 原生模型（gemini-2.5-flash-image / gemini-3-pro-image 等）支持：
	//   - imageConfig.aspectRatio：支持的比例（10 种）：21:9 / 16:9 / 4:3 / 3:2 / 1:1 / 9:16 / 3:4 / 2:3 / 5:4 / 4:5
	//   - imageConfig.imageSize：支持的档位："1K" / "2K" / "4K"（部分模型支持）
	//     参考：https://developers.googleblog.com/en/gemini-2-5-flash-image-now-ready-for-production-with-new-aspect-ratios
	imageConfig := make(map[string]interface{})
	if aspectRatio != "" {
		imageConfig["aspectRatio"] = aspectRatio
	}
	if imageSize != "" {
		imageConfig["imageSize"] = imageSize
	}

	// 构建用户 parts：先放文本 prompt，再依次追加参考图片（inlineData）
	userParts := []dto.GeminiPart{
		{Text: request.Prompt},
	}
	for _, imgStr := range request.ReferenceImages {
		if imgStr == "" {
			continue
		}
		mimeType, base64Data := parseDataURL(imgStr)
		if base64Data == "" {
			continue
		}
		userParts = append(userParts, dto.GeminiPart{
			InlineData: &dto.GeminiInlineData{
				MimeType: mimeType,
				Data:     base64Data,
			},
		})
	}

	// 根据是否有参考图片决定 responseModalities：
	// - 无参考图：可能需要文本描述 + 图像，使用 ["TEXT", "IMAGE"]
	// - 有参考图：纯图生图场景，只需要 ["IMAGE"]
	responseModalities := []string{"TEXT", "IMAGE"}
	if len(imageReq.ReferenceImages) > 0 {
		responseModalities = []string{"IMAGE"}
	}

	geminiRequest := dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role:  "user",
				Parts: userParts,
			},
		},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			// 必须显式声明返回模态，否则 gemini-*-image 模型不会输出图像。
			ResponseModalities: responseModalities,
		},
	}

	if sampleCount > 1 {
		geminiRequest.GenerationConfig.CandidateCount = &sampleCount
	}

	if len(imageConfig) > 0 {
		imageConfigJSON, err := common.Marshal(imageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal imageConfig: %w", err)
		}
		geminiRequest.GenerationConfig.ImageConfig = imageConfigJSON
	}

	return geminiRequest, nil
}

// normalizeAspectRatio 接受三种输入：
//   - 已经是 Gemini 风格的 "16:9"/"1:1" 等，直接返回；
//   - OpenAI 风格 "1024x1024"/"1792x1024"，按常见分辨率映射；
//   - 空串，返回空串（让上游使用默认值，不强行注入）。
func normalizeAspectRatio(size string) string {
	size = strings.TrimSpace(size)
	if size == "" {
		return ""
	}
	if strings.Contains(size, ":") {
		return size
	}
	switch size {
	case "256x256", "512x512", "1024x1024":
		return "1:1"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "1024x1792":
		return "9:16"
	case "1792x1024":
		return "16:9"
	default:
		return ""
	}
}

// normalizeImageSize 把 resolution/quality 字符串映射到 Gemini/Imagen imageSize 参数。
//
// Gemini 原生模型（generateContent）支持："1K" / "2K" / "4K"（部分模型）
// Imagen API（:predict）支持：仅 "1K"（默认）和 "2K"，"4K" 会降级为 "2K"
func normalizeImageSize(quality string) string {
	q := strings.TrimSpace(quality)
	if q == "" {
		return ""
	}
	switch strings.ToLower(q) {
	case "1k", "standard", "medium", "low", "auto":
		return "1K"
	case "2k", "hd", "high":
		return "2K"
	case "4k":
		// Gemini 原生模型部分支持 4K，Imagen 不支持会降级为 2K
		// 这里统一返回 "4K"，让上游 API 自行处理（支持则用，不支持则降级）
		return "4K"
	}
	// 兜底：常见分辨率字符串
	switch q {
	case "512", "512x512", "1024x1024", "1024":
		return "1K"
	case "2048x2048", "2048":
		return "2K"
	case "4096x4096", "4096":
		return "4K"
	}
	return ""
}

// normalizeAspectRatioForImagen 将长宽比映射到 Imagen API 支持的 5 种有效值之一。
//
// Imagen 支持（https://ai.google.dev/gemini-api/docs/imagen）：
//
//	"1:1"（默认）、"3:4"、"4:3"、"9:16"、"16:9"
//
// 不在支持列表中的值映射到最接近的有效比例；空串返回空串（使用 Imagen 默认值 "1:1"）。
func normalizeAspectRatioForImagen(aspectRatio string) string {
	switch aspectRatio {
	case "1:1", "16:9", "9:16", "4:3", "3:4":
		return aspectRatio // 直接有效
	case "21:9":
		return "16:9" // 超宽屏 → 宽屏
	case "3:2":
		return "4:3" // 3:2 接近 4:3（均为横向）
	case "2:3":
		return "3:4" // 竖向
	case "5:4", "4:5":
		return "1:1" // 接近正方形
	default:
		return "" // 未知比例，让 Imagen 使用默认值
	}
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {

	if model_setting.GetGeminiSettings().ThinkingAdapterEnabled &&
		!model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
		// 新增逻辑：处理 -thinking-<budget> 格式
		if strings.Contains(info.UpstreamModelName, "-thinking-") {
			parts := strings.Split(info.UpstreamModelName, "-thinking-")
			info.UpstreamModelName = parts[0]
		} else if strings.HasSuffix(info.UpstreamModelName, "-thinking") { // 旧的适配
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
		} else if strings.HasSuffix(info.UpstreamModelName, "-nothinking") {
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-nothinking")
		} else if baseModel, level, ok := reasoning.TrimEffortSuffix(info.UpstreamModelName); ok && level != "" {
			info.UpstreamModelName = baseModel
		}
	}

	version := model_setting.GetGeminiVersionSetting(info.UpstreamModelName)

	// 检查是否是图片生成请求：
	//   - imagen-* 系列模型走旧版 :predict；
	//   - gemini-*-image / gemini-*-image-preview 等走新版 :generateContent。
	// 注意：RelayModeImagesEdits 是带参考图片的图片生成，也需要走图片生成路径。
	if info.RelayMode == constant.RelayModeImagesGenerations || info.RelayMode == constant.RelayModeImagesEdits {
		if useImagenPredictAPI(info.UpstreamModelName) {
			return fmt.Sprintf("%s/%s/models/%s:predict", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
		}
		return fmt.Sprintf("%s/%s/models/%s:generateContent", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
	}

	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		action := "embedContent"
		if info.IsGeminiBatchEmbedding {
			action = "batchEmbedContents"
		}
		return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
	}

	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent?alt=sse"
		if info.RelayMode == constant.RelayModeGemini {
			info.DisablePing = true
		}
	}
	return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("x-goog-api-key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	geminiRequest, err := CovertOpenAI2Gemini(c, *request, info)
	if err != nil {
		return nil, err
	}

	return geminiRequest, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	if request.Input == nil {
		return nil, errors.New("input is required")
	}

	inputs := request.ParseInput()
	if len(inputs) == 0 {
		return nil, errors.New("input is empty")
	}
	// We always build a batch-style payload with `requests`, so ensure we call the
	// batch endpoint upstream to avoid payload/endpoint mismatches.
	info.IsGeminiBatchEmbedding = true
	// process all inputs
	geminiRequests := make([]map[string]interface{}, 0, len(inputs))
	for _, input := range inputs {
		geminiRequest := map[string]interface{}{
			"model": fmt.Sprintf("models/%s", info.UpstreamModelName),
			"content": dto.GeminiChatContent{
				Parts: []dto.GeminiPart{
					{
						Text: input,
					},
				},
			},
		}

		// set specific parameters for different models
		// https://ai.google.dev/api/embeddings?hl=zh-cn#method:-models.embedcontent
		switch info.UpstreamModelName {
		case "text-embedding-004", "gemini-embedding-exp-03-07", "gemini-embedding-001":
			// Only newer models introduced after 2024 support OutputDimensionality
			dimensions := lo.FromPtrOr(request.Dimensions, 0)
			if dimensions > 0 {
				geminiRequest["outputDimensionality"] = dimensions
			}
		}
		geminiRequests = append(geminiRequests, geminiRequest)
	}

	return map[string]interface{}{
		"requests": geminiRequests,
	}, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeGemini {
		if strings.Contains(info.RequestURLPath, ":embedContent") ||
			strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
			return NativeGeminiEmbeddingHandler(c, resp, info)
		}
		if info.IsStream {
			return GeminiTextGenerationStreamHandler(c, info, resp)
		} else {
			return GeminiTextGenerationHandler(c, info, resp)
		}
	}

	// 图片生成响应分两种格式：
	//   - imagen-* (走 :predict) → predictions[].bytesBase64Encoded
	//   - gemini-*-image (走 :generateContent) → candidates[].content.parts[].inlineData.data
	// 注意：RelayModeImagesEdits 是带参考图片的图片生成，也需要走图片响应解析。
	if info.RelayMode == constant.RelayModeImagesGenerations || info.RelayMode == constant.RelayModeImagesEdits {
		if useImagenPredictAPI(info.UpstreamModelName) {
			return GeminiImageHandler(c, info, resp)
		}
		return GeminiNativeImageHandler(c, info, resp)
	}
	// 兼容旧逻辑：直接通过模型前缀走 imagen 处理（例如调用方未走 RelayModeImagesGenerations）。
	if useImagenPredictAPI(info.UpstreamModelName) {
		return GeminiImageHandler(c, info, resp)
	}

	// check if the model is an embedding model
	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		return GeminiEmbeddingHandler(c, info, resp)
	}

	if info.IsStream {
		return GeminiChatStreamHandler(c, info, resp)
	} else {
		return GeminiChatHandler(c, info, resp)
	}

}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
