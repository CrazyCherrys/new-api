package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// ImageGeneratorFunc 图像生成函数类型
// 通过依赖注入避免 service -> relay 循环依赖
type ImageGeneratorFunc func(ctx context.Context, userID int, req *dto.ImageRequest) (*dto.ImageResponse, error)

// GenerateImageFunc 图像生成函数（在 main.go 中注入）
var GenerateImageFunc ImageGeneratorFunc

// ImageGenerationService 图像生成服务
// 负责执行实际的图像生成逻辑
type ImageGenerationService struct {
	storageService *ImageStorageService
}

// NewImageGenerationService 创建图像生成服务
func NewImageGenerationService() *ImageGenerationService {
	return &ImageGenerationService{
		storageService: NewImageStorageService(),
	}
}

// Generate 执行图像生成
func (s *ImageGenerationService) Generate(ctx context.Context, task *model.ImageTask) ([]string, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	if GenerateImageFunc == nil {
		return nil, fmt.Errorf("image generation function not initialized")
	}

	log.Printf("[ImageGen] Task #%d: model=%s, prompt=%s",
		task.ID, task.ModelID, truncateString(task.Prompt, 50))

	requestModelID := task.ModelID
	cfg := system_setting.GetImageGenerationSetting()
	if modelCfg, ok := resolveImageModelSetting(cfg, task.ModelID); ok {
		if strings.TrimSpace(modelCfg.RequestModelID) != "" {
			requestModelID = modelCfg.RequestModelID
		}
	}

	// 构建图像生成请求
	imageReq := &dto.ImageRequest{
		Model:  requestModelID,
		Prompt: task.Prompt,
	}

	// 设置可选参数
	if task.Count > 0 {
		n := uint(task.Count)
		imageReq.N = &n
	}
	if task.Resolution != "" {
		imageReq.Size = task.Resolution
	}
	if task.AspectRatio != "" {
		imageReq.AspectRatio = task.AspectRatio
	}
	if task.ReferenceImage != "" {
		imageReq.ReferenceImage = task.ReferenceImage
	}

	// 调用注入的图像生成函数
	resp, err := GenerateImageFunc(ctx, task.UserID, imageReq)
	if err != nil {
		return nil, fmt.Errorf("image generation failed: %w", err)
	}

	if resp == nil || len(resp.Data) == 0 {
		return nil, fmt.Errorf("no images generated")
	}

	// 提取图片 URL 并存储
	var imageURLs []string
	for i, img := range resp.Data {
		var storedURL string
		var storeErr error

		if img.Url != "" {
			// 从 URL 下载并存储
			storedURL, storeErr = s.storageService.StoreImageFromURL(img.Url)
		} else if img.B64Json != "" {
			// TODO: 处理 base64 编码的图片
			log.Printf("[ImageGen] Task #%d: skipping base64 image #%d (not yet implemented)", task.ID, i+1)
			continue
		} else {
			log.Printf("[ImageGen] Task #%d: image #%d has no URL or base64 data", task.ID, i+1)
			continue
		}

		if storeErr != nil {
			log.Printf("[ImageGen] Task #%d: failed to store image #%d: %v", task.ID, i+1, storeErr)
			continue
		}

		imageURLs = append(imageURLs, storedURL)
		log.Printf("[ImageGen] Task #%d: stored image #%d: %s", task.ID, i+1, storedURL)
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("failed to store any images")
	}

	return imageURLs, nil
}

// truncateString 截断字符串用于日志
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
