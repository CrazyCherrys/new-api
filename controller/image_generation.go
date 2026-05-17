package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/worker_setting"

	"github.com/gin-gonic/gin"
)

var imageGenerationDimensionPattern = regexp.MustCompile(`^(\d{2,5})\s*[xX×*]\s*(\d{2,5})$`)
var imageGenerationAspectRatioPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?::|x|X|/)\s*(\d+(?:\.\d+)?)$`)

func sanitizeImageGenerationTaskParams(task *model.ImageGenerationTask) {
	if task == nil {
		return
	}
	service.FillImageGenerationTaskSummary(task)
	task.Params = service.SanitizeImageGenerationParamsForResponse(task.Params)
}

func resolveImageGenerationTaskDisplayName(task *model.ImageGenerationTask) string {
	if task == nil || strings.TrimSpace(task.ModelId) == "" {
		return ""
	}
	mapping, err := model.GetModelMappingByRequestModel(task.ModelId)
	if err != nil || mapping == nil {
		return ""
	}
	return strings.TrimSpace(mapping.DisplayName)
}

func parseImageGenerationTaskJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var data map[string]any
	if err := common.UnmarshalJsonStr(raw, &data); err != nil || len(data) == 0 {
		return nil
	}
	return data
}

func parseImageGenerationNestedJSON(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return nil
		}
		return typed
	case string:
		return parseImageGenerationTaskJSON(typed)
	default:
		return nil
	}
}

func readImageGenerationValue(sources []map[string]any, keys []string) any {
	for _, source := range sources {
		if len(source) == 0 {
			continue
		}
		for _, key := range keys {
			value, ok := source[key]
			if !ok || value == nil {
				continue
			}
			if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
				continue
			}
			return value
		}
	}
	return nil
}

func readImageGenerationStringValue(sources []map[string]any, keys []string) string {
	value := readImageGenerationValue(sources, keys)
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func readImageGenerationPositiveInt(sources []map[string]any, keys []string) int {
	value := readImageGenerationValue(sources, keys)
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case float32:
		if typed > 0 {
			return int(typed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func parseImageGenerationDimension(value string) (int, int) {
	matches := imageGenerationDimensionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return 0, 0
	}
	width, err := strconv.Atoi(matches[1])
	if err != nil || width <= 0 {
		return 0, 0
	}
	height, err := strconv.Atoi(matches[2])
	if err != nil || height <= 0 {
		return 0, 0
	}
	return width, height
}

func parseImageGenerationAspectRatio(value string) float64 {
	matches := imageGenerationAspectRatioPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return 0
	}
	width, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || width <= 0 {
		return 0
	}
	height, err := strconv.ParseFloat(matches[2], 64)
	if err != nil || height <= 0 {
		return 0
	}
	return width / height
}

func joinImageGenerationTextParts(parts []string) string {
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		normalized := strings.TrimSpace(part)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return strings.Join(result, " · ")
}

func buildImageGenerationTaskDetail(task *model.ImageGenerationTask) *model.ImageGenerationTaskDetail {
	if task == nil {
		return nil
	}

	detailQuantity := 0
	paramsMap := parseImageGenerationTaskJSON(task.Params)
	metadataMap := parseImageGenerationTaskJSON(task.ImageMetadata)
	metadataDetails := parseImageGenerationNestedJSON(metadataMap["metadata"])
	sources := []map[string]any{paramsMap, metadataMap, metadataDetails}

	outputWidth := readImageGenerationPositiveInt([]map[string]any{metadataMap, metadataDetails}, []string{
		"width",
		"output_width",
		"image_width",
		"outputWidth",
		"imageWidth",
	})
	outputHeight := readImageGenerationPositiveInt([]map[string]any{metadataMap, metadataDetails}, []string{
		"height",
		"output_height",
		"image_height",
		"outputHeight",
		"imageHeight",
	})
	if outputWidth <= 0 || outputHeight <= 0 {
		if width, height := parseImageGenerationDimension(readImageGenerationStringValue([]map[string]any{metadataMap, metadataDetails}, []string{
			"size",
			"output_size",
			"dimensions",
			"outputSize",
		})); width > 0 && height > 0 {
			outputWidth = width
			outputHeight = height
		}
	}

	outputSizeText := "-"
	if outputWidth > 0 && outputHeight > 0 {
		outputSizeText = fmt.Sprintf("%dx%d", outputWidth, outputHeight)
	} else if metadataSize := readImageGenerationStringValue([]map[string]any{metadataMap, metadataDetails}, []string{
		"size",
		"output_size",
		"dimensions",
		"outputSize",
	}); metadataSize != "" {
		outputSizeText = metadataSize
	}

	sizeParts := make([]string, 0, 2)
	if aspectRatio := readImageGenerationStringValue(sources, []string{"aspect_ratio", "aspectRatio"}); aspectRatio != "" {
		sizeParts = append(sizeParts, aspectRatio)
	}
	if resolution := readImageGenerationStringValue([]map[string]any{paramsMap}, []string{"resolution", "image_size", "imageSize"}); resolution != "" {
		sizeParts = append(sizeParts, resolution)
	}
	sizeText := joinImageGenerationTextParts(sizeParts)
	if sizeText == "" {
		if requestSize := readImageGenerationStringValue([]map[string]any{paramsMap}, []string{"size"}); requestSize != "" {
			sizeText = requestSize
		} else if outputSizeText != "-" {
			sizeText = outputSizeText
		}
	}

	qualityParts := []string{
		readImageGenerationStringValue(sources, []string{"quality", "quality_level"}),
		readImageGenerationStringValue([]map[string]any{paramsMap}, []string{"resolution", "image_size", "imageSize"}),
		readImageGenerationStringValue(sources, []string{"style"}),
	}
	if quantity := readImageGenerationPositiveInt([]map[string]any{paramsMap}, []string{"n", "quantity"}); quantity > 0 {
		detailQuantity = quantity
	}
	qualityText := joinImageGenerationTextParts(qualityParts)
	if qualityText == "" {
		qualityText = "-"
	}

	displayName := resolveImageGenerationTaskDisplayName(task)
	detail := model.BuildImageGenerationTaskDetail(task, displayName)
	if detail == nil {
		return nil
	}
	detail.OutputWidth = outputWidth
	detail.OutputHeight = outputHeight
	detail.OutputSizeText = outputSizeText
	detail.SizeText = sizeText
	detail.QualityText = qualityText
	detail.Quantity = detailQuantity
	return detail
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

func sanitizeImageCreativeListItem(item *model.ImageCreativeListItem) {
	if item == nil {
		return
	}
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
	detail := buildImageGenerationTaskDetail(task)
	common.ApiSuccess(c, detail)
}

// GetImageGenerationSettings 获取用户侧图片生成设置
func GetImageGenerationSettings(c *gin.Context) {
	cfg := worker_setting.GetWorkerSetting()
	maxImageSize := cfg.MaxImageSize
	if maxImageSize <= 0 {
		maxImageSize = 10
	}
	pollingInterval := cfg.PollingInterval
	if pollingInterval <= 0 {
		pollingInterval = 5
	}
	common.ApiSuccess(c, gin.H{
		"max_image_size":               maxImageSize,
		"polling_interval":             pollingInterval,
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

	cursor := c.Query("cursor")
	if cursor != "" && model.ImageTaskCursorPaginationSupported(queryParams) {
		cursorPage, err := model.GetImageTasksByUserCursor(userId, cursor, pageInfo.GetPageSize(), queryParams)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		summaries := make([]*model.ImageGenerationTaskSummary, 0, len(cursorPage.Items))
		for _, task := range cursorPage.Items {
			summary := model.BuildImageGenerationTaskSummary(task)
			if summary != nil {
				summaries = append(summaries, summary)
			}
		}

		common.ApiSuccess(c, gin.H{
			"page":        pageInfo.GetPage(),
			"page_size":   pageInfo.GetPageSize(),
			"total":       int(cursorPage.Total),
			"items":       summaries,
			"next_cursor": cursorPage.NextCursor,
			"has_more":    cursorPage.HasMore,
		})
		return
	}

	includeTotal := pageInfo.GetPage() <= 1
	tasks, total, hasMore, err := model.GetImageTasksByUserID(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams, includeTotal)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	for _, task := range tasks {
		sanitizeImageGenerationTaskParams(task)
	}
	summaries := make([]*model.ImageGenerationTaskSummary, 0, len(tasks))
	for _, task := range tasks {
		summary := model.BuildImageGenerationTaskSummary(task)
		if summary != nil {
			summaries = append(summaries, summary)
		}
	}
	if includeTotal {
		pageInfo.SetTotal(int(total))
	}
	pageInfo.SetItems(summaries)
	common.ApiSuccess(c, gin.H{
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
		"total":     pageInfo.Total,
		"items":     pageInfo.Items,
		"has_more":  hasMore,
	})
}

// GetImageGenerationTaskUpdates 获取任务列表的轻量增量更新。
func GetImageGenerationTaskUpdates(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	completedSince, _ := strconv.ParseInt(c.Query("completed_since"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	tasks, err := model.GetImageTaskUpdatesByUserID(userId, completedSince, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	summaries := make([]*model.ImageGenerationTaskSummary, 0, len(tasks))
	for _, task := range tasks {
		summary := model.BuildImageGenerationTaskSummary(task)
		if summary != nil {
			summaries = append(summaries, summary)
		}
	}

	common.ApiSuccess(c, gin.H{
		"items": summaries,
	})
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
	detail := buildImageGenerationTaskDetail(task)
	common.ApiSuccess(c, detail)
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
	localAssetPaths := make([]string, 0, len(assets)*2)
	for _, asset := range assets {
		if asset == nil {
			continue
		}
		if clean, ok := service.ImageGenerationLocalAssetPathFromURLForCache(asset.ImageUrl); ok {
			localAssetPaths = append(localAssetPaths, clean)
		}
		if clean, ok := service.ImageGenerationLocalAssetPathFromURLForCache(asset.ThumbnailUrl); ok {
			localAssetPaths = append(localAssetPaths, clean)
		}
	}
	service.WarmImageGenerationLocalAssetAccessCacheForUser(userId, localAssetPaths)
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
	startedAt := time.Now()
	pageInfo := common.GetPageQuery(c)
	cursor := c.Query("cursor")
	assets, total, nextCursor, hasMore, err := model.GetApprovedInspirationAssets(cursor, pageInfo.GetPageSize())
	if err != nil {
		common.SysLog(fmt.Sprintf(
			"inspiration assets handler failed: cursor=%q page_size=%d elapsed_ms=%d err=%v",
			strings.TrimSpace(cursor),
			pageInfo.GetPageSize(),
			time.Since(startedAt).Milliseconds(),
			err,
		))
		common.ApiError(c, err)
		return
	}

	for _, asset := range assets {
		sanitizeImageCreativeListItem(asset)
	}
	localAssetPaths := make([]string, 0, len(assets))
	for _, asset := range assets {
		if asset == nil {
			continue
		}
		if clean, ok := service.ImageGenerationLocalAssetPathFromURLForCache(asset.ThumbnailUrl); ok {
			localAssetPaths = append(localAssetPaths, clean)
		}
	}
	service.WarmApprovedInspirationLocalAssetAccessCache(localAssetPaths)
	pageInfo.SetItems(assets)
	common.SysLog(fmt.Sprintf(
		"inspiration assets handler: cursor=%q page_size=%d items=%d total=%d has_more=%t elapsed_ms=%d",
		strings.TrimSpace(cursor),
		pageInfo.GetPageSize(),
		len(assets),
		total,
		hasMore,
		time.Since(startedAt).Milliseconds(),
	))
	common.ApiSuccess(c, gin.H{
		"page":        pageInfo.GetPage(),
		"page_size":   pageInfo.GetPageSize(),
		"total":       int(total),
		"items":       assets,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
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

	if task.Status == model.ImageTaskStatusPending || task.Status == model.ImageTaskStatusGenerating {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "运行中的任务暂不支持删除，请等待完成后再删除",
		})
		return
	}

	// 删除任务
	if err := service.DeleteImageGenerationTask(task); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateImageGenerationLocalAssetAccessCache()

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

func writeImageGenerationSSETaskUpdate(c *gin.Context, task *model.ImageGenerationTask) bool {
	data, err := common.Marshal(gin.H{
		"id":             task.Id,
		"model_id":       task.ModelId,
		"prompt":         task.Prompt,
		"status":         task.Status,
		"image_url":      task.ImageUrl,
		"thumbnail_url":  task.ThumbnailUrl,
		"error_message":  task.ErrorMessage,
		"created_time":   task.CreatedTime,
		"started_time":   task.EffectiveStartedTime(),
		"completed_time": task.CompletedTime,
	})
	if err != nil {
		return false
	}

	fmt.Fprintf(c.Writer, "event: task_update\ndata: %s\n\n", string(data))
	c.Writer.Flush()
	return true
}

func writeImageGenerationSSETaskUpdatePayload(c *gin.Context, update service.ImageGenerationTaskUpdate) bool {
	data, err := common.Marshal(gin.H{
		"id":             update.Id,
		"model_id":       update.ModelId,
		"prompt":         update.Prompt,
		"status":         update.Status,
		"image_url":      update.ImageUrl,
		"thumbnail_url":  update.ThumbnailUrl,
		"error_message":  update.ErrorMessage,
		"created_time":   update.CreatedTime,
		"started_time":   update.StartedTime,
		"completed_time": update.CompletedTime,
	})
	if err != nil {
		return false
	}

	fmt.Fprintf(c.Writer, "event: task_update\ndata: %s\n\n", string(data))
	c.Writer.Flush()
	return true
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

	updateCh := service.SubscribeImageGenerationTaskUpdates(userId)
	defer service.UnsubscribeImageGenerationTaskUpdates(userId, updateCh)

	initialTasks, err := model.GetImageTaskUpdatesByUserID(userId, common.GetTimestamp()-60, 100)
	if err == nil {
		for _, task := range initialTasks {
			if task != nil {
				_ = writeImageGenerationSSETaskUpdate(c, task)
			}
		}
	}

	heartbeatTicker := time.NewTicker(service.ImageGenerationTaskHeartbeatInterval())
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-clientGone:
			return
		case update, ok := <-updateCh:
			if !ok {
				return
			}
			if !writeImageGenerationSSETaskUpdatePayload(c, update) {
				return
			}
		case <-heartbeatTicker.C:
			fmt.Fprintf(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		}
	}
}
