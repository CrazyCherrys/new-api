package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type createVideoGenerationTaskRequest struct {
	ModelId         string `json:"model_id" binding:"required"`
	Prompt          string `json:"prompt" binding:"required"`
	RequestEndpoint string `json:"request_endpoint" binding:"required"`
	Params          string `json:"params"`
}

func GetVideoGenerationModels(c *gin.Context) {
	items, err := service.ListVideoGenerationModels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func CreateVideoGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	var req createVideoGenerationTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	task, err := service.CreateVideoGenerationTask(userId, req.ModelId, req.Prompt, req.RequestEndpoint, req.Params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, task)
}

func GetVideoGenerationTasks(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	pageInfo := common.GetPageQuery(c)
	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)
	page, err := service.ListVideoGenerationTasks(
		userId,
		pageInfo.GetPage(),
		pageInfo.GetPageSize(),
		c.Query("cursor"),
		c.Query("status"),
		c.Query("model_id"),
		startTime,
		endTime,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload := gin.H{
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
		"items":     page.Items,
		"has_more":  page.HasMore,
	}
	if page.NextCursor != "" {
		payload["next_cursor"] = page.NextCursor
	}
	if page.HasTotal {
		payload["total"] = page.Total
	}
	common.ApiSuccess(c, payload)
}

func GetVideoGenerationTaskUpdates(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
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

	items, err := service.ListVideoGenerationTaskUpdates(userId, completedSince, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": items,
	})
}

func GetVideoGenerationTaskDetail(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	identifier := strings.TrimSpace(c.Param("id"))
	task, err := service.GetVideoGenerationTaskDetail(userId, identifier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, task)
}

func RetryVideoGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	task, err := service.RetryVideoGenerationTask(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, task)
}

func DeleteVideoGenerationTask(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.DeleteVideoGenerationTask(userId, id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
