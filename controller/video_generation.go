package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func GetVideoGenerationModels(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	mappings, _, err := service.GetActiveVideoModelMappings(0, 1000)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	models := make([]gin.H, 0, len(mappings))
	for _, mapping := range mappings {
		models = append(models, gin.H{
			"request_model":    mapping.RequestModel,
			"display_name":     mapping.DisplayName,
			"model_series":     mapping.ModelSeries,
			"request_endpoint": mapping.RequestEndpoint,
			"resolutions":      mapping.Resolutions,
			"aspect_ratios":    mapping.AspectRatios,
		})
	}
	common.ApiSuccess(c, models)
}

func CreateVideoGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	var req service.CanvasVideoTaskCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.ModelID) == "" {
		common.ApiErrorMsg(c, "model_id 不能为空")
		return
	}
	if len(req.ReferenceImages) == 0 {
		common.ApiErrorMsg(c, "reference_images 不能为空")
		return
	}

	mapping, err := model.GetActiveModelMappingByRequestModel(req.ModelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if mapping == nil || mapping.ModelType != 3 {
		common.ApiErrorMsg(c, "视频模型映射不存在")
		return
	}

	requestEndpoint := service.NormalizeVideoEndpoint(req.RequestEndpoint)
	mappingEndpoint := service.NormalizeVideoEndpoint(mapping.RequestEndpoint)
	if requestEndpoint == "" {
		requestEndpoint = mappingEndpoint
	}
	if mappingEndpoint != requestEndpoint {
		common.ApiErrorMsg(c, fmt.Sprintf("request endpoint mismatch: expected %s, got %s", mappingEndpoint, requestEndpoint))
		return
	}

	channelTypes, err := service.ChannelTypesForVideoEndpoint(requestEndpoint)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channelID, err := service.SelectVideoChannelForModel(mapping.ActualModel, userId, channelTypes)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, mapping.ActualModel); setupErr != nil {
		common.ApiError(c, setupErr.Err)
		return
	}

	taskReq := relaycommon.TaskSubmitReq{
		Model:    mapping.ActualModel,
		Prompt:   strings.TrimSpace(req.Prompt),
		Duration: req.Duration,
		Size:     strings.TrimSpace(req.Size),
		Metadata: map[string]interface{}{},
	}
	if requestEndpoint == string(constant.EndpointTypeOpenAIVideo) {
		taskReq.InputReference = strings.TrimSpace(req.ReferenceImages[0])
	} else {
		taskReq.Images = append([]string(nil), req.ReferenceImages...)
	}
	if taskReq.Prompt == "" {
		taskReq.Prompt = "image to video"
	}
	if taskReq.Duration <= 0 {
		taskReq.Duration = 5
	}

	jsonData, err := common.Marshal(taskReq)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	bodyStorage, err := common.CreateBodyStorage(jsonData)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Set(common.KeyBodyStorage, bodyStorage)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Body = io.NopCloser(bytes.NewReader(jsonData))
	c.Request.ContentLength = int64(len(jsonData))
	c.Request.URL.Path = videoRequestPathForEndpoint(requestEndpoint)
	c.Request.URL.RawPath = c.Request.URL.Path
	common.SetContextKey(c, constant.ContextKeyOriginalModel, mapping.ActualModel)

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var (
		result  *relay.TaskSubmitResult
		taskErr *dto.TaskError
	)
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		if !taskErr.LocalError {
			processChannelError(
				c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode),
			)
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
		common.SysError("settle task billing error: " + settleErr.Error())
	}
	service.LogTaskConsumption(c, relayInfo)

	task := model.InitTask(result.Platform, relayInfo)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      relayInfo.PriceData.ModelPrice,
		GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      relayInfo.PriceData.ModelRatio,
		OtherRatios:     relayInfo.PriceData.OtherRatios,
		OriginModelName: relayInfo.OriginModelName,
		PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
	}
	task.Quota = result.Quota
	task.Data = result.TaskData
	task.Action = relayInfo.Action
	task.Properties.Input = taskReq.Prompt
	task.Properties.OriginModelName = req.ModelID
	if len(task.Data) > 0 {
		var dataMap map[string]any
		if err := common.Unmarshal(task.Data, &dataMap); err == nil {
			dataMap["canvas_source"] = true
			dataMap["request_model"] = req.ModelID
			if dataBytes, marshalErr := common.Marshal(dataMap); marshalErr == nil {
				task.Data = dataBytes
			}
		}
	}
	if insertErr := task.Insert(); insertErr != nil {
		logger.LogError(c, "insert task error: "+insertErr.Error())
		common.ApiError(c, insertErr)
		return
	}

	common.ApiSuccess(c, service.BuildCanvasVideoTaskItem(task))
}

func GetVideoGenerationTasks(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	pageInfo := common.GetPageQuery(c)
	status := strings.TrimSpace(c.Query("status"))
	modelId := strings.TrimSpace(c.Query("model_id"))
	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)

	items, total, err := service.ListCanvasVideoTasks(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), status, modelId, startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, gin.H{
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
		"total":     pageInfo.Total,
		"items":     pageInfo.Items,
	})
}

func GetVideoGenerationTaskDetail(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		common.ApiErrorMsg(c, "task_id 不能为空")
		return
	}

	item, err := service.GetCanvasVideoTaskDetail(userId, taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func videoRequestPathForEndpoint(endpoint string) string {
	switch service.NormalizeVideoEndpoint(endpoint) {
	case string(constant.EndpointTypeOpenAIVideo):
		return "/v1/videos"
	default:
		return "/v1/video/generations"
	}
}
