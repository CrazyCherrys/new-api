package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// SetImageRouter 设置图像生成任务路由
func SetImageRouter(router *gin.Engine) {
	imageGroup := router.Group("/api/v1/image-tasks")
	imageGroup.Use(middleware.UserAuth()) // 需要用户认证
	{
		// 创建图像生成任务
		imageGroup.POST("/generate", controller.CreateImageTask)

		// 参考图上传
		imageGroup.POST("/upload-reference", controller.UploadReferenceImage)

		// 任务历史记录
		imageGroup.GET("/history", controller.ListImageTasks)
		imageGroup.GET("/history/:id", controller.GetImageTask)
		imageGroup.DELETE("/history/:id", controller.DeleteImageTask)

		// 配置管理（仅管理员）
		imageGroup.GET("/config", middleware.AdminAuth(), controller.GetImageGenerationConfig)
		imageGroup.PUT("/config", middleware.AdminAuth(), controller.UpdateImageGenerationConfig)
	}
}
