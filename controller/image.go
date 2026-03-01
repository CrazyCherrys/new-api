package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateImageTask 创建图像生成任务
func CreateImageTask(c *gin.Context) {
	// 1. 用户认证检查
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权，请先登录",
		})
		return
	}

	// 2. 请求参数验证
	var req dto.CreateImageTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 设置默认值
	if req.Resolution == "" {
		req.Resolution = "1K"
	}
	if req.AspectRatio == "" {
		req.AspectRatio = "1:1"
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 4 {
		req.Count = 4
	}

	// 3. 模型 RPM 限流检查
	rpmService := service.NewModelRPMService()
	acquired, retryAfter, err := rpmService.Acquire(userID, req.Model)
	if err != nil {
		common.ApiError(c, fmt.Errorf("限流检查失败: %w", err))
		return
	}
	if !acquired {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": fmt.Sprintf("请求过于频繁，请在 %v 后重试", retryAfter),
			"retry_after": retryAfter.Seconds(),
		})
		return
	}

	// 4. 创建任务记录
	taskID := uuid.New().String()
	now := time.Now().Unix()
	task := &model.ImageGenerationTask{
		ID:             taskID,
		UserID:         userID,
		ModelID:        req.Model,
		Prompt:         req.Prompt,
		Resolution:     req.Resolution,
		AspectRatio:    req.AspectRatio,
		ReferenceImage: req.ReferenceImage,
		Count:          req.Count,
		Status:         "pending",
		Attempts:       0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := task.Insert(); err != nil {
		common.ApiError(c, fmt.Errorf("创建任务失败: %w", err))
		return
	}

	// 5. 返回任务 ID 和状态
	common.ApiSuccess(c, task.ToDTO())
}

// GetImageTask 获取单个图像任务详情
func GetImageTask(c *gin.Context) {
	// 1. 用户认证检查
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权，请先登录",
		})
		return
	}

	// 2. 任务 ID 验证
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "任务 ID 不能为空",
		})
		return
	}

	// 3. 查询任务
	task, err := model.GetImageTaskByID(taskID)
	if err != nil {
		common.ApiError(c, fmt.Errorf("任务不存在: %w", err))
		return
	}

	// 4. 权限检查（仅查询自己的任务）
	if task.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问该任务",
		})
		return
	}

	// 5. 返回任务详情
	common.ApiSuccess(c, task.ToDTO())
}

// ListImageTasks 获取用户的图像任务列表
func ListImageTasks(c *gin.Context) {
	// 1. 用户认证检查
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权，请先登录",
		})
		return
	}

	// 2. 分页参数解析
	var req dto.ListImageTasksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("请求参数错误: %v", err),
		})
		return
	}

	// 设置默认分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 3. 过滤条件解析
	filters := make(map[string]interface{})
	if req.Status != "" {
		filters["status"] = req.Status
	}
	if req.Model != "" {
		filters["model"] = req.Model
	}
	if req.StartTime > 0 {
		filters["start_time"] = req.StartTime
	}
	if req.EndTime > 0 {
		filters["end_time"] = req.EndTime
	}
	if req.Search != "" {
		filters["search"] = req.Search
	}

	// 4. 查询任务列表
	tasks, total, err := model.ListByUser(userID, req.Page, req.PageSize, filters)
	if err != nil {
		common.ApiError(c, fmt.Errorf("查询任务列表失败: %w", err))
		return
	}

	// 5. 返回分页结果
	common.ApiSuccess(c, gin.H{
		"data":      model.TaskListToDTO(tasks),
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	})
}

// DeleteImageTask 删除图像任务
func DeleteImageTask(c *gin.Context) {
	// 1. 用户认证检查
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权，请先登录",
		})
		return
	}

	// 2. 任务 ID 验证
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "任务 ID 不能为空",
		})
		return
	}

	// 3. 删除任务（防越权）
	err := model.DeleteImageTaskByID(taskID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "任务不存在或无权删除",
			})
			return
		}
		common.ApiError(c, fmt.Errorf("删除任务失败: %w", err))
		return
	}

	// 4. 返回成功
	common.ApiSuccess(c, gin.H{
		"message": "任务删除成功",
	})
}

func GetImage(c *gin.Context) {
	// 保留原有的空函数，避免破坏现有引用
}
