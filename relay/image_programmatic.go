package relay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GenerateImageProgrammatic 程序化图像生成接口
// 用于从 service 层调用，不依赖 HTTP 请求上下文
func GenerateImageProgrammatic(ctx context.Context, userID int, req *dto.ImageRequest) (*dto.ImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("image request is nil")
	}

	// 获取用户信息
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		ginCtx := createMockGinContext(ctx, userID, user.Group)

		retryParam := &service.RetryParam{
			Ctx:        ginCtx,
			TokenGroup: user.Group,
			ModelName:  req.Model,
		}

		channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
		if err != nil {
			lastErr = fmt.Errorf("failed to select channel: %w", err)
			continue
		}
		if channel == nil {
			lastErr = fmt.Errorf("no available channel for model: %s", req.Model)
			continue
		}

		info := &relaycommon.RelayInfo{
			UserId:          userID,
			TokenGroup:      user.Group,
			UsingGroup:      selectGroup,
			UserGroup:       user.Group,
			RelayMode:       relayconstant.RelayModeImagesGenerations,
			OriginModelName: req.Model,
			Request:         req,
		}

		info.ChannelType = channel.Type
		info.ChannelId = channel.Id
		info.ApiType = channel.Type
		info.ApiKey = channel.Key
		info.ChannelBaseUrl = channel.GetBaseURL()

		adaptor := GetAdaptor(info.ApiType)
		if adaptor == nil {
			lastErr = fmt.Errorf("invalid api type: %d", info.ApiType)
			continue
		}
		adaptor.Init(info)

		convertedRequest, err := adaptor.ConvertImageRequest(ginCtx, info, *req)
		if err != nil {
			lastErr = fmt.Errorf("failed to convert request: %w", err)
			continue
		}

		var requestBody io.Reader
		switch convertedRequest.(type) {
		case *bytes.Buffer:
			requestBody = convertedRequest.(io.Reader)
		default:
			jsonData, err := common.Marshal(convertedRequest)
			if err != nil {
				lastErr = fmt.Errorf("failed to marshal request: %w", err)
				continue
			}
			requestBody = bytes.NewBuffer(jsonData)
		}

		resp, err := adaptor.DoRequest(ginCtx, info, requestBody)
		if err != nil {
			lastErr = fmt.Errorf("failed to do request: %w", err)
			continue
		}

		result, err := processImageResponse(resp)
		if err != nil {
			lastErr = err
			continue
		}

		return result, nil
	}

	return nil, fmt.Errorf("all %d attempts failed, last error: %w", maxRetries, lastErr)
}

func processImageResponse(resp any) (*dto.ImageResponse, error) {

	httpResp, ok := resp.(*http.Response)
	if !ok {
		return nil, fmt.Errorf("invalid response type")
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if err := common.Unmarshal(bodyBytes, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (status %d): %s", httpResp.StatusCode, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("request failed with status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(bodyBytes, &imageResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(imageResp.Data) == 0 {
		return nil, fmt.Errorf("no images generated in response")
	}

	return &imageResp, nil
}

// createMockGinContext 创建一个模拟的 gin.Context 用于程序化调用
func createMockGinContext(ctx context.Context, userID int, userGroup string) *gin.Context {
	ginCtx, _ := gin.CreateTestContext(nil)
	ginCtx.Request = &http.Request{
		Header: make(http.Header),
	}
	ginCtx.Request = ginCtx.Request.WithContext(ctx)

	// 设置必要的上下文键（转换为 string 类型）
	ginCtx.Set(string(constant.ContextKeyUserId), userID)
	ginCtx.Set(string(constant.ContextKeyUserGroup), userGroup)

	return ginCtx
}
