package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// ImageStorageService 图片存储服务
type ImageStorageService struct {
	config   *system_setting.ImageGenerationSetting
	s3Client *s3.Client
}

// NewImageStorageService 创建图片存储服务实例
func NewImageStorageService() *ImageStorageService {
	config := system_setting.GetImageGenerationSetting()
	service := &ImageStorageService{
		config: config,
	}

	// 如果配置为 S3 存储，初始化 S3 客户端
	if config.StorageType == "s3" {
		service.initS3Client()
	}

	return service
}

// initS3Client 初始化 S3 客户端
func (s *ImageStorageService) initS3Client() {
	if s.config.StorageS3Endpoint == "" || s.config.StorageS3Bucket == "" {
		common.SysError("S3 storage configured but endpoint or bucket is empty")
		return
	}

	// 创建 AWS 配置
	cfg := aws.Config{
		Region: s.config.StorageS3Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			s.config.StorageS3AccessKey,
			s.config.StorageS3SecretKey,
			"",
		),
	}

	// 创建 S3 客户端
	s.s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s.config.StorageS3Endpoint)
		o.UsePathStyle = true // 使用路径样式访问，兼容 MinIO 等 S3 兼容服务
	})

	common.SysLog(fmt.Sprintf("S3 client initialized: endpoint=%s, bucket=%s, region=%s",
		s.config.StorageS3Endpoint, s.config.StorageS3Bucket, s.config.StorageS3Region))
}

// StoreImageFromURL 从 URL 下载图片并存储
func (s *ImageStorageService) StoreImageFromURL(imageURL string) (string, error) {
	// 下载图片
	data, filename, err := s.downloadImage(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}

	// 存储图片
	return s.StoreImageFromBytes(data, filename)
}

// StoreImageFromBytes 从字节数据存储图片
func (s *ImageStorageService) StoreImageFromBytes(data []byte, filename string) (string, error) {
	// 生成唯一文件名
	uniqueFilename := s.generateUniqueFilename(filename)

	// 根据配置选择存储方式
	switch s.config.StorageType {
	case "s3":
		return s.storeToS3(data, uniqueFilename)
	case "local":
		return s.storeToLocal(data, uniqueFilename)
	default:
		return "", fmt.Errorf("unsupported storage type: %s", s.config.StorageType)
	}
}

// downloadImage 下载图片
func (s *ImageStorageService) downloadImage(imageURL string) ([]byte, string, error) {
	// 使用重试机制下载
	var lastErr error
	maxRetries := s.config.ImageMaxRetryAttempts
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryInterval := time.Duration(s.config.ImageRetryIntervalSeconds) * time.Second
	if retryInterval <= 0 {
		retryInterval = 10 * time.Second
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			common.SysLog(fmt.Sprintf("Retrying image download (attempt %d/%d): %s", attempt+1, maxRetries, imageURL))
			time.Sleep(retryInterval)
		}

		resp, err := DoDownloadRequest(imageURL, "image_storage")
		if err != nil {
			lastErr = fmt.Errorf("download request failed: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("download failed with status code: %d", resp.StatusCode)
			continue
		}

		// 读取图片数据
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// 从 URL 或 Content-Type 推断文件扩展名
		filename := s.extractFilenameFromURL(imageURL, resp.Header.Get("Content-Type"))

		common.SysLog(fmt.Sprintf("Successfully downloaded image: %s (size: %d bytes)", imageURL, len(data)))
		return data, filename, nil
	}

	return nil, "", fmt.Errorf("failed to download image after %d attempts: %w", maxRetries, lastErr)
}

// extractFilenameFromURL 从 URL 或 Content-Type 提取文件名
func (s *ImageStorageService) extractFilenameFromURL(url string, contentType string) string {
	// 尝试从 URL 路径提取文件名
	if idx := strings.LastIndex(url, "/"); idx != -1 {
		path := url[idx+1:]
		if qIdx := strings.Index(path, "?"); qIdx != -1 {
			path = path[:qIdx]
		}
		if path != "" && strings.Contains(path, ".") {
			return path
		}
	}

	// 根据 Content-Type 生成文件名
	ext := ".jpg" // 默认扩展名
	if contentType != "" {
		switch {
		case strings.Contains(contentType, "png"):
			ext = ".png"
		case strings.Contains(contentType, "gif"):
			ext = ".gif"
		case strings.Contains(contentType, "webp"):
			ext = ".webp"
		case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
			ext = ".jpg"
		}
	}

	return "image" + ext
}

// generateUniqueFilename 生成唯一文件名
func (s *ImageStorageService) generateUniqueFilename(originalFilename string) string {
	// 提取文件扩展名
	ext := filepath.Ext(originalFilename)
	if ext == "" {
		ext = ".jpg"
	}

	// 使用 UUID 生成唯一文件名
	uniqueID := uuid.New().String()
	timestamp := time.Now().Format("20060102")

	return fmt.Sprintf("%s_%s%s", timestamp, uniqueID, ext)
}

// storeToLocal 存储到本地文件系统
func (s *ImageStorageService) storeToLocal(data []byte, filename string) (string, error) {
	// 确保存储目录存在
	storagePath := s.config.StorageLocalPath
	if storagePath == "" {
		storagePath = "./data/images"
	}

	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	// 构建完整文件路径
	filePath := filepath.Join(storagePath, filename)

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	common.SysLog(fmt.Sprintf("Image stored locally: %s (size: %d bytes)", filePath, len(data)))

	// 返回可访问的 URL 路径
	// 假设通过 /images/ 路径可以访问存储的图片
	return fmt.Sprintf("/images/%s", filename), nil
}

// storeToS3 存储到 S3
func (s *ImageStorageService) storeToS3(data []byte, filename string) (string, error) {
	if s.s3Client == nil {
		return "", fmt.Errorf("S3 client not initialized")
	}

	// 构建 S3 对象键
	objectKey := filename
	if s.config.StorageS3PathPrefix != "" {
		objectKey = filepath.Join(s.config.StorageS3PathPrefix, filename)
	}

	// 检测 Content-Type
	contentType := http.DetectContentType(data)
	if contentType == "application/octet-stream" {
		// 根据文件扩展名设置更准确的 Content-Type
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		}
	}

	// 上传到 S3
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.StorageS3Bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	common.SysLog(fmt.Sprintf("Image stored to S3: bucket=%s, key=%s (size: %d bytes)",
		s.config.StorageS3Bucket, objectKey, len(data)))

	// 构建 S3 URL
	s3URL := s.buildS3URL(objectKey)
	return s3URL, nil
}

// buildS3URL 构建 S3 访问 URL
func (s *ImageStorageService) buildS3URL(objectKey string) string {
	endpoint := s.config.StorageS3Endpoint
	bucket := s.config.StorageS3Bucket

	// 移除 endpoint 末尾的斜杠
	endpoint = strings.TrimSuffix(endpoint, "/")

	// 构建 URL（路径样式）
	return fmt.Sprintf("%s/%s/%s", endpoint, bucket, objectKey)
}

// DeleteImage 删除图片（可选功能）
func (s *ImageStorageService) DeleteImage(imageURL string) error {
	switch s.config.StorageType {
	case "local":
		return s.deleteFromLocal(imageURL)
	case "s3":
		return s.deleteFromS3(imageURL)
	default:
		return fmt.Errorf("unsupported storage type: %s", s.config.StorageType)
	}
}

// deleteFromLocal 从本地删除
func (s *ImageStorageService) deleteFromLocal(imageURL string) error {
	// 从 URL 提取文件名
	filename := filepath.Base(imageURL)
	filePath := filepath.Join(s.config.StorageLocalPath, filename)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，视为成功
		}
		return fmt.Errorf("failed to delete local file: %w", err)
	}

	common.SysLog(fmt.Sprintf("Image deleted from local storage: %s", filePath))
	return nil
}

// deleteFromS3 从 S3 删除
func (s *ImageStorageService) deleteFromS3(imageURL string) error {
	if s.s3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	// 从 URL 提取对象键
	objectKey := s.extractS3KeyFromURL(imageURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.StorageS3Bucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	common.SysLog(fmt.Sprintf("Image deleted from S3: bucket=%s, key=%s",
		s.config.StorageS3Bucket, objectKey))
	return nil
}

// extractS3KeyFromURL 从 S3 URL 提取对象键
func (s *ImageStorageService) extractS3KeyFromURL(url string) string {
	// 移除 endpoint 和 bucket 部分
	endpoint := strings.TrimSuffix(s.config.StorageS3Endpoint, "/")
	bucket := s.config.StorageS3Bucket

	prefix := fmt.Sprintf("%s/%s/", endpoint, bucket)
	if strings.HasPrefix(url, prefix) {
		return url[len(prefix):]
	}

	// 如果无法解析，返回 URL 的最后部分
	return filepath.Base(url)
}
