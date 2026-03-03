package controller

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// ImageTaskController 图像任务控制器
type ImageTaskController struct {
	taskService *service.ImageTaskService
}

// NewImageTaskController 创建图像任务控制器
func NewImageTaskController(taskService *service.ImageTaskService) *ImageTaskController {
	return &ImageTaskController{
		taskService: taskService,
	}
}

// CreateImageTask 创建图像生成任务
// POST /api/v1/image-tasks/generate
func (ctrl *ImageTaskController) CreateImageTask(c *gin.Context) {
	var req struct {
		ModelID        string `json:"model_id" binding:"required"`
		Prompt         string `json:"prompt" binding:"required"`
		Resolution     string `json:"resolution"`
		AspectRatio    string `json:"aspect_ratio"`
		ReferenceImage string `json:"reference_image"`
		Count          int    `json:"count"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	// 获取用户ID
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
		return
	}

	// 验证参数
	if req.Count <= 0 {
		req.Count = 1
	}

	// 创建任务
	task, err := ctrl.taskService.CreateTask(
		c.Request.Context(),
		userID,
		req.ModelID,
		req.Prompt,
		req.Resolution,
		req.AspectRatio,
		req.ReferenceImage,
		req.Count,
	)
	if err != nil {
		// 检查是否是限流错误
		if errors.Is(err, service.ErrRateLimited) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "rate limit exceeded for model " + req.ModelID,
			})
			return
		}
		if errors.Is(err, service.ErrInvalidModel) ||
			errors.Is(err, service.ErrInvalidResolution) ||
			errors.Is(err, service.ErrInvalidAspectRatio) ||
			errors.Is(err, service.ErrInvalidImageCount) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to create task: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "task created successfully",
		"data": gin.H{
			"task_id": task.ID,
			"status":  task.Status,
		},
	})
}

// GetImageTask 获取任务详情
// GET /api/v1/image-tasks/history/:id
func (ctrl *ImageTaskController) GetImageTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid task id",
		})
		return
	}

	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
		return
	}

	task, err := ctrl.taskService.GetTaskByID(taskID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "task not found",
		})
		return
	}

	// 解析图片URL
	imageURLs, _ := task.GetImageURLs()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":              task.ID,
			"model_id":        task.ModelID,
			"prompt":          task.Prompt,
			"resolution":      task.Resolution,
			"aspect_ratio":    task.AspectRatio,
			"reference_image": task.ReferenceImage,
			"count":           task.Count,
			"status":          task.Status,
			"error_message":   task.ErrorMessage,
			"image_urls":      imageURLs,
			"attempts":        task.Attempts,
			"created_at":      task.CreatedAt,
			"updated_at":      task.UpdatedAt,
			"completed_at":    task.CompletedAt,
		},
	})
}

// ListImageTasks 获取任务列表
// GET /api/v1/image-tasks/history
func (ctrl *ImageTaskController) ListImageTasks(c *gin.Context) {
	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
		return
	}

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	model := c.Query("model")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tasks, total, err := ctrl.taskService.ListTasksByUser(userID, page, pageSize, status, model, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to list tasks: " + err.Error(),
		})
		return
	}

	// 转换为响应格式
	taskList := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		imageURLs, _ := task.GetImageURLs()
		taskList = append(taskList, gin.H{
			"id":           task.ID,
			"model_id":     task.ModelID,
			"prompt":       task.Prompt,
			"status":       task.Status,
			"image_urls":   imageURLs,
			"created_at":   task.CreatedAt,
			"completed_at": task.CompletedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"tasks": taskList,
			"pagination": gin.H{
				"page":       page,
				"page_size":  pageSize,
				"total":      total,
				"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
			},
		},
	})
}

// DeleteImageTask 删除任务
// DELETE /api/v1/image-tasks/history/:id
func (ctrl *ImageTaskController) DeleteImageTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid task id",
		})
		return
	}

	userID := c.GetInt("id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "unauthorized",
		})
		return
	}

	err = ctrl.taskService.DeleteTask(taskID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to delete task: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "task deleted successfully",
	})
}

// 全局控制器实例（将在 main.go 中初始化）
var imageTaskController *ImageTaskController

// InitImageTaskController 初始化图像任务控制器
func InitImageTaskController(taskService *service.ImageTaskService) {
	imageTaskController = NewImageTaskController(taskService)
}

// 导出的 Handler 函数（用于路由注册）

func CreateImageTask(c *gin.Context) {
	if imageTaskController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "image task service not initialized",
		})
		return
	}
	imageTaskController.CreateImageTask(c)
}

func GetImageTask(c *gin.Context) {
	if imageTaskController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "image task service not initialized",
		})
		return
	}
	imageTaskController.GetImageTask(c)
}

func ListImageTasks(c *gin.Context) {
	if imageTaskController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "image task service not initialized",
		})
		return
	}
	imageTaskController.ListImageTasks(c)
}

func DeleteImageTask(c *gin.Context) {
	if imageTaskController == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "image task service not initialized",
		})
		return
	}
	imageTaskController.DeleteImageTask(c)
}

func GetImageGenerationConfig(c *gin.Context) {
	cfg := normalizeImageGenerationConfig(system_setting.GetImageGenerationSetting())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cfg,
	})
}

func UpdateImageGenerationConfig(c *gin.Context) {
	var req system_setting.ImageGenerationSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request: " + err.Error(),
		})
		return
	}
	req = normalizeImageGenerationConfig(&req)

	configMap, err := config.ConfigToMap(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to convert config: " + err.Error(),
		})
		return
	}

	// 添加前缀并准备批量更新
	updates := make(map[string]string)
	for key, value := range configMap {
		dbKey := "image_generation_setting." + key
		updates[dbKey] = value
	}

	// 使用事务批量更新
	if err := model.UpdateOptionsAtomic(updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to update config: " + err.Error(),
		})
		return
	}

	// 重新加载所有配置
	options, err := model.AllOption()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to reload config: " + err.Error(),
		})
		return
	}
	optionMap := make(map[string]string)
	for _, opt := range options {
		optionMap[opt.Key] = opt.Value
	}
	if err := config.GlobalConfig.LoadFromDB(optionMap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to apply config: " + err.Error(),
		})
		return
	}

	// 注意：ImageStorageService 每次创建新实例时会自动读取最新配置
	// 因此配置更新后，新的图像生成任务会使用新配置（包括 S3 切换）
	// ImageTaskService 的超时和重试参数也会在下次任务创建时生效

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "config updated successfully",
	})
}

func normalizeImageGenerationConfig(cfg *system_setting.ImageGenerationSetting) system_setting.ImageGenerationSetting {
	if cfg == nil {
		return system_setting.ImageGenerationSetting{}
	}

	normalized := *cfg
	if normalized.ModelSettings == nil {
		normalized.ModelSettings = make(map[string]system_setting.ImageGenerationModelSetting)
	}

	defaultResolution := normalized.DefaultResolution
	if defaultResolution == "" {
		defaultResolution = "1024x1024"
	}
	defaultAspectRatio := normalized.DefaultAspectRatio
	if defaultAspectRatio == "" {
		defaultAspectRatio = "1:1"
	}
	maxImageCount := normalized.MaxImageCount
	if maxImageCount <= 0 {
		maxImageCount = 10
	}

	trimmedModelSettings := make(map[string]system_setting.ImageGenerationModelSetting)
	for modelID, setting := range normalized.ModelSettings {
		id := strings.TrimSpace(modelID)
		if id == "" {
			continue
		}
		normalizedSetting := setting
		if normalizedSetting.DisplayName == "" {
			normalizedSetting.DisplayName = id
		}
		if normalizedSetting.RequestModelID == "" {
			normalizedSetting.RequestModelID = id
		}
		if normalizedSetting.RequestEndpoint == "" {
			normalizedSetting.RequestEndpoint = "openai"
		}
		if normalizedSetting.ModelType == "" {
			normalizedSetting.ModelType = "image"
		}
		if normalizedSetting.DefaultResolution == "" {
			normalizedSetting.DefaultResolution = defaultResolution
		}
		if normalizedSetting.DefaultAspectRatio == "" {
			normalizedSetting.DefaultAspectRatio = defaultAspectRatio
		}
		if len(normalizedSetting.Resolutions) == 0 && normalizedSetting.DefaultResolution != "" {
			normalizedSetting.Resolutions = []string{normalizedSetting.DefaultResolution}
		}
		if len(normalizedSetting.AspectRatios) == 0 && normalizedSetting.DefaultAspectRatio != "" {
			normalizedSetting.AspectRatios = []string{normalizedSetting.DefaultAspectRatio}
		}
		if normalizedSetting.MaxImageCount <= 0 {
			normalizedSetting.MaxImageCount = maxImageCount
		}
		if normalizedSetting.RpmLimit <= 0 {
			normalizedSetting.RpmLimit = normalized.RpmLimit
		}
		trimmedModelSettings[id] = normalizedSetting
	}
	normalized.ModelSettings = trimmedModelSettings

	enabledSet := make(map[string]struct{})
	enabledModels := make([]string, 0, len(normalized.EnabledModels))
	for _, modelID := range normalized.EnabledModels {
		id := strings.TrimSpace(modelID)
		if id == "" {
			continue
		}
		if _, ok := enabledSet[id]; ok {
			continue
		}
		enabledSet[id] = struct{}{}
		enabledModels = append(enabledModels, id)
	}
	if len(enabledModels) == 0 && len(normalized.ModelSettings) > 0 {
		for modelID := range normalized.ModelSettings {
			enabledModels = append(enabledModels, modelID)
		}
		sort.Strings(enabledModels)
	}
	normalized.EnabledModels = enabledModels

	for _, modelID := range normalized.EnabledModels {
		if _, ok := normalized.ModelSettings[modelID]; ok {
			continue
		}
		normalized.ModelSettings[modelID] = system_setting.ImageGenerationModelSetting{
			DisplayName:        modelID,
			RequestModelID:     modelID,
			RequestEndpoint:    "openai",
			ModelType:          "image",
			DefaultResolution:  defaultResolution,
			DefaultAspectRatio: defaultAspectRatio,
			Resolutions:        []string{defaultResolution},
			AspectRatios:       []string{defaultAspectRatio},
			Durations:          []string{},
			MaxImageCount:      maxImageCount,
			RpmLimit:           normalized.RpmLimit,
			RpmEnabled:         false,
		}
	}
	enabledSet = make(map[string]struct{}, len(normalized.EnabledModels))
	for _, modelID := range normalized.EnabledModels {
		enabledSet[modelID] = struct{}{}
	}

	if normalized.DefaultModel == "" {
		if len(normalized.EnabledModels) > 0 {
			normalized.DefaultModel = normalized.EnabledModels[0]
		}
	} else {
		if _, ok := enabledSet[normalized.DefaultModel]; !ok && len(normalized.EnabledModels) > 0 {
			normalized.DefaultModel = normalized.EnabledModels[0]
		}
	}

	return normalized
}

func UploadReferenceImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "no file uploaded",
		})
		return
	}

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to open uploaded file: " + err.Error(),
		})
		return
	}
	defer src.Close()

	// 读取文件内容
	fileData := make([]byte, file.Size)
	_, err = src.Read(fileData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to read file: " + err.Error(),
		})
		return
	}

	// 创建存储服务并保存文件
	storageService := service.NewImageStorageService()
	uploadedURL, err := storageService.StoreImageFromBytes(fileData, file.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to store image: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url": uploadedURL,
		},
	})
}
