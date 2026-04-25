package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

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

	common.ApiSuccess(c, task)
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

	common.ApiSuccess(c, task)
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

	// 重置任务状态
	task.Status = model.ImageTaskStatusPending
	task.ErrorMessage = ""
	task.CompletedTime = 0

	if err := task.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, task)
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
	if err := model.DeleteImageTask(taskId); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"message": "删除成功"})
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
					data := fmt.Sprintf(`{"id":%d,"status":"%s","image_url":"%s","error_message":"%s","completed_time":%d}`,
						task.Id, task.Status, task.ImageUrl, task.ErrorMessage, task.CompletedTime)
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
