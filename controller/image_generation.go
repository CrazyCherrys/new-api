package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/worker_setting"

	"github.com/gin-gonic/gin"
)

func sanitizeImageGenerationTaskParams(task *model.ImageGenerationTask) {
	if task == nil {
		return
	}
	service.FillImageGenerationTaskSummary(task)
	task.Params = service.SanitizeImageGenerationParamsForResponse(task.Params)
}

func sanitizeImageGenerationAssetParams(asset *model.ImageGenerationAsset) {
	if asset == nil {
		return
	}
	asset.Params = service.SanitizeImageGenerationParamsForResponse(asset.Params)
}

func sanitizeImageCreativeAssetParams(asset *model.ImageCreativeAsset) {
	if asset == nil {
		return
	}
	asset.Params = service.SanitizeImageGenerationParamsForResponse(asset.Params)
}

func sanitizeImageCreativeAdminSubmissionParams(submission *model.ImageCreativeAdminSubmission) {
	if submission == nil {
		return
	}
	submission.Params = service.SanitizeImageGenerationParamsForResponse(submission.Params)
}

// CreateImageGenerationTask 创建图片生成任务
func CreateImageGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	var req struct {
		ModelId         string `json:"model_id" binding:"required"`
		Prompt          string `json:"prompt" binding:"required"`
		RequestEndpoint string `json:"request_endpoint" binding:"required"`
		Params          string `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 调用服务层创建任务（会自动启动异步处理）
	task, err := service.CreateImageGenerationTask(userId, req.ModelId, req.Prompt, req.RequestEndpoint, req.Params)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	sanitizeImageGenerationTaskParams(task)
	common.ApiSuccess(c, task)
}

// GetImageGenerationSettings 获取用户侧图片生成设置
func GetImageGenerationSettings(c *gin.Context) {
	cfg := worker_setting.GetWorkerSetting()
	maxImageSize := cfg.MaxImageSize
	if maxImageSize <= 0 {
		maxImageSize = 10
	}
	common.ApiSuccess(c, gin.H{
		"max_image_size":               maxImageSize,
		"user_custom_key_enabled":      cfg.UserCustomKeyEnabled,
		"user_custom_base_url_allowed": cfg.UserCustomBaseURLAllowed,
	})
}

// GetImageGenerationTasks 获取任务列表（分页+筛选）
func GetImageGenerationTasks(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	pageInfo := common.GetPageQuery(c)

	status := c.Query("status")
	modelId := c.Query("model_id")
	requestEndpoint := c.Query("request_endpoint")
	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	queryParams := model.ImageTaskQueryParams{
		Status:          status,
		ModelId:         modelId,
		RequestEndpoint: requestEndpoint,
		StartTime:       startTime,
		EndTime:         endTime,
		SortBy:          sortBy,
		SortOrder:       sortOrder,
	}

	tasks, total, err := model.GetImageTasksByUserID(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, task := range tasks {
		sanitizeImageGenerationTaskParams(task)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasks)
	common.ApiSuccess(c, pageInfo)
}

// GetImageGenerationTaskDetail 获取任务详情
func GetImageGenerationTaskDetail(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的任务ID",
		})
		return
	}

	task, err := model.GetImageTaskByID(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "任务不存在",
		})
		return
	}

	// 验证任务所有权
	if task.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问此任务",
		})
		return
	}

	sanitizeImageGenerationTaskParams(task)
	common.ApiSuccess(c, task)
}

// GetImageGenerationAssets 获取当前用户的图片资产仓库列表。
func GetImageGenerationAssets(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	pageInfo := common.GetPageQuery(c)

	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)
	queryParams := model.ImageAssetQueryParams{
		Keyword:     c.Query("keyword"),
		ModelId:     c.Query("model_id"),
		ModelSeries: c.Query("model_series"),
		StartTime:   startTime,
		EndTime:     endTime,
		SortBy:      c.Query("sort_by"),
		SortOrder:   c.Query("sort_order"),
	}

	assets, total, stats, err := model.GetImageAssetsByUserID(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, asset := range assets {
		sanitizeImageGenerationAssetParams(asset)
	}
	filterOptions, err := model.GetImageAssetFilterOptions(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
		"total":     int(total),
		"items":     assets,
		"stats":     stats,
		"filters":   filterOptions,
	})
}

// GetImageGenerationAssetDetail 获取当前用户的单个图片资产详情。
func GetImageGenerationAssetDetail(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的资产ID",
		})
		return
	}

	asset, err := model.GetImageAssetByID(userId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if asset == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "资产不存在",
		})
		return
	}

	sanitizeImageGenerationAssetParams(asset)
	common.ApiSuccess(c, asset)
}

// SubmitImageGenerationAssetToInspiration 将当前用户的图片资产提交到灵感审核。
func SubmitImageGenerationAssetToInspiration(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的资产ID",
		})
		return
	}

	submission, err := model.SubmitImageAssetToInspiration(userId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, submission)
}

// GetInspirationAssets 获取公开灵感作品列表。
func GetInspirationAssets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	assets, total, err := model.GetApprovedInspirationAssets(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, asset := range assets {
		sanitizeImageCreativeAssetParams(asset)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assets)
	common.ApiSuccess(c, pageInfo)
}

// GetInspirationAssetDetail 获取公开灵感作品详情。
func GetInspirationAssetDetail(c *gin.Context) {
	assetId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的作品ID",
		})
		return
	}

	asset, err := model.GetApprovedInspirationAssetByID(assetId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if asset == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "作品不存在",
		})
		return
	}

	sanitizeImageCreativeAssetParams(asset)
	common.ApiSuccess(c, asset)
}

// GetImageInspirationSubmissions 获取灵感审核列表。
func GetImageInspirationSubmissions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	submissions, total, err := model.GetImageInspirationSubmissions(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, submission := range submissions {
		sanitizeImageCreativeAdminSubmissionParams(submission)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(submissions)
	common.ApiSuccess(c, pageInfo)
}

// ReviewImageInspirationSubmission 审核灵感投稿。
func ReviewImageInspirationSubmission(c *gin.Context) {
	reviewerId := c.GetInt("id")
	if reviewerId == 0 {
		common.ApiError(c, errors.New("未授权"))
		return
	}

	submissionId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的投稿ID",
		})
		return
	}

	var req struct {
		Status       string `json:"status" binding:"required"`
		RejectReason string `json:"reject_reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	submission, err := model.ReviewImageInspirationSubmission(submissionId, reviewerId, req.Status, req.RejectReason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateInspirationLocalAssetAccessCache()

	sanitizeImageCreativeAdminSubmissionParams(submission)
	common.ApiSuccess(c, submission)
}

// DeleteImageInspirationSubmission 删除灵感投稿。
func DeleteImageInspirationSubmission(c *gin.Context) {
	adminId := c.GetInt("id")
	if adminId == 0 {
		common.ApiError(c, errors.New("未授权"))
		return
	}

	submissionId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的投稿ID",
		})
		return
	}

	if err := model.DeleteImageInspirationSubmission(submissionId); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateInspirationLocalAssetAccessCache()

	common.ApiSuccess(c, gin.H{"message": "删除成功"})
}

// GetImageGenerationFile 读取本地存储的图片生成结果文件。
func GetImageGenerationFile(c *gin.Context) {
	userId := c.GetInt("id")

	assetPath := c.Param("path")
	allowed := false
	if userId != 0 {
		userAllowed, err := service.CanAccessImageGenerationLocalAsset(userId, assetPath)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		allowed = userAllowed
	}
	if !allowed {
		publicAllowed, err := service.CanAccessApprovedInspirationLocalAsset(assetPath)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		allowed = publicAllowed
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "资源不存在",
		})
		return
	}

	file, contentType, err := service.OpenImageGenerationLocalAsset(assetPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "资源不存在",
		})
		return
	}
	defer file.Close()

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = io.Copy(c.Writer, file)
}

// RetryImageGenerationTask 重试失败任务
func RetryImageGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的任务ID",
		})
		return
	}

	task, err := model.GetImageTaskByID(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "任务不存在",
		})
		return
	}

	// 验证任务所有权
	if task.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问此任务",
		})
		return
	}

	// 只能重试失败的任务
	if task.Status != model.ImageTaskStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "只能重试失败的任务",
		})
		return
	}

	if err := service.RetryImageGenerationTask(taskId); err != nil {
		common.ApiError(c, err)
		return
	}

	reloadedTask, err := model.GetImageTaskByID(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	sanitizeImageGenerationTaskParams(reloadedTask)
	common.ApiSuccess(c, reloadedTask)
}

// DeleteImageGenerationTask 删除任务
func DeleteImageGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	taskId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的任务ID",
		})
		return
	}

	task, err := model.GetImageTaskByID(taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "任务不存在",
		})
		return
	}

	// 验证任务所有权
	if task.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问此任务",
		})
		return
	}

	// 删除任务
	if err := service.DeleteImageGenerationTask(task); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"message": "删除成功"})
}

// GetImageGenerationModels 获取可用的图片生成模型列表
func GetImageGenerationModels(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	// 获取所有启用的绘画模型
	mappings, _, err := model.GetActiveImageModelMappings(0, 1000)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var models []gin.H
	for _, mapping := range mappings {
		imageCapabilities, err := model.EffectiveImageCapabilities(mapping.ImageCapabilities)
		if err != nil {
			imageCapabilities = model.DefaultImageCapabilities()
		}
		models = append(models, gin.H{
			"request_model":      mapping.RequestModel,
			"display_name":       mapping.DisplayName,
			"model_series":       mapping.ModelSeries,
			"request_endpoint":   mapping.RequestEndpoint,
			"resolutions":        mapping.Resolutions,
			"aspect_ratios":      mapping.AspectRatios,
			"image_capabilities": imageCapabilities,
		})
	}

	common.ApiSuccess(c, models)
}

// ImageGenerationSSE SSE推送任务状态更新
func ImageGenerationSSE(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 创建一个通道用于通知客户端断开
	clientGone := c.Request.Context().Done()

	// 发送初始连接成功消息
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"message\":\"连接成功\"}\n\n")
	c.Writer.Flush()

	// 轮询任务状态变化
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 记录上次发送的任务状态，避免重复发送
	lastTaskStates := make(map[int]string)

	for {
		select {
		case <-clientGone:
			// 客户端断开连接
			return
		case <-ticker.C:
			// 查询用户的所有进行中的任务
			queryParams := model.ImageTaskQueryParams{}
			tasks, _, err := model.GetImageTasksByUserID(userId, 0, 100, queryParams)
			if err != nil {
				continue
			}

			// 检查任务状态变化
			for _, task := range tasks {
				lastState, exists := lastTaskStates[task.Id]
				currentState := task.Status

				// 如果状态发生变化，或者是新任务，发送更新
				if !exists || lastState != currentState {
					data := fmt.Sprintf(`{"id":%d,"status":"%s","image_url":"%s","thumbnail_url":"%s","error_message":"%s","completed_time":%d}`,
						task.Id, task.Status, task.ImageUrl, task.ThumbnailUrl, task.ErrorMessage, task.CompletedTime)
					fmt.Fprintf(c.Writer, "event: task_update\ndata: %s\n\n", data)
					c.Writer.Flush()

					lastTaskStates[task.Id] = currentState
				}
			}

			// 清理已完成或失败的任务状态记录（避免内存泄漏）
			for taskId, state := range lastTaskStates {
				if state == model.ImageTaskStatusSuccess || state == model.ImageTaskStatusFailed {
					// 检查任务是否还在列表中
					found := false
					for _, task := range tasks {
						if task.Id == taskId {
							found = true
							break
						}
					}
					if !found {
						delete(lastTaskStates, taskId)
					}
				}
			}
		}
	}
}
