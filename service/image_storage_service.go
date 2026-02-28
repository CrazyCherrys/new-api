package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	MaxImageSize = 10 * 1024 * 1024 // 10MB
)

var (
	allowedMimeTypes = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}
)

// ImageStorageService 图片存储服务
type ImageStorageService struct {
	s3Enabled    bool
	s3Client     *minio.Client
	s3Bucket     string
	s3PublicURL  string
	localRoot    string
	clientMutex  sync.RWMutex
}

// NewImageStorageService 创建图片存储服务实例
func NewImageStorageService() (*ImageStorageService, error) {
	service := &ImageStorageService{}

	// 读取配置
	common.OptionMapRWMutex.RLock()
	s3EnabledStr := common.OptionMap["ImageStorageS3Enabled"]
	s3Endpoint := common.OptionMap["ImageStorageS3Endpoint"]
	s3Region := common.OptionMap["ImageStorageS3Region"]
	s3Bucket := common.OptionMap["ImageStorageS3Bucket"]
	s3AccessKey := common.OptionMap["ImageStorageS3AccessKey"]
	s3SecretKey := common.OptionMap["ImageStorageS3SecretKey"]
	s3PublicURL := common.OptionMap["ImageStorageS3PublicURL"]
	s3UseSSLStr := common.OptionMap["ImageStorageS3UseSSL"]
	s3PathStyleStr := common.OptionMap["ImageStorageS3PathStyle"]
	localRoot := common.OptionMap["ImageStorageLocalRoot"]
	common.OptionMapRWMutex.RUnlock()

	service.s3Enabled = s3EnabledStr == "true"
	service.s3Bucket = s3Bucket
	service.s3PublicURL = s3PublicURL
	service.localRoot = localRoot

	if service.localRoot == "" {
		service.localRoot = "./storage/images"
	}

	// 初始化 S3 客户端
	if service.s3Enabled && s3Endpoint != "" && s3Bucket != "" {
		useSSL := s3UseSSLStr != "false"
		pathStyle := s3PathStyleStr == "true"

		client, err := minio.New(s3Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(s3AccessKey, s3SecretKey, ""),
			Secure: useSSL,
			Region: s3Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 client: %w", err)
		}

		// 设置路径样式
		if pathStyle {
			client.SetAppInfo("new-api", "1.0")
		}

		service.s3Client = client
		common.SysLog("Image storage service initialized with S3")
	} else {
		service.s3Enabled = false
		common.SysLog("Image storage service initialized with local storage")
	}

	return service, nil
}

// StoreGeneratedImages 存储生成的图片
func (s *ImageStorageService) StoreGeneratedImages(images []string) ([]string, error) {
	if len(images) == 0 {
		return []string{}, nil
	}

	results := make([]string, 0, len(images))
	for _, img := range images {
		url, err := s.storeImage(img)
		if err != nil {
			common.SysLog(fmt.Sprintf("Failed to store image: %v", err))
			// 存储失败时保留原始 URL
			results = append(results, img)
			continue
		}
		results = append(results, url)
	}

	return results, nil
}

// storeImage 存储单个图片
func (s *ImageStorageService) storeImage(imageData string) (string, error) {
	// 解析图片数据
	data, mimeType, err := s.parseImageData(imageData)
	if err != nil {
		return "", err
	}

	// 验证大小
	if len(data) > MaxImageSize {
		return "", fmt.Errorf("image size exceeds limit: %d bytes", len(data))
	}

	// 验证 MIME 类型
	ext, ok := allowedMimeTypes[mimeType]
	if !ok {
		return "", fmt.Errorf("unsupported image type: %s", mimeType)
	}

	// 根据配置选择存储方式
	if s.s3Enabled {
		return s.storeS3(data, ext)
	}
	return s.storeLocal(data, ext)
}

// parseImageData 解析图片数据（支持 base64、data URL、HTTP URL）
func (s *ImageStorageService) parseImageData(imageData string) ([]byte, string, error) {
	// 处理 data URL
	if strings.HasPrefix(imageData, "data:") {
		parts := strings.SplitN(imageData, ",", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid data URL format")
		}

		// 提取 MIME 类型
		mimeType := "image/png" // 默认
		if strings.Contains(parts[0], ";") {
			mimePart := strings.Split(parts[0], ";")[0]
			if strings.HasPrefix(mimePart, "data:") {
				mimeType = strings.TrimPrefix(mimePart, "data:")
			}
		}

		// 解码 base64
		data, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode base64: %w", err)
		}

		return data, mimeType, nil
	}

	// 处理 HTTP/HTTPS URL
	if strings.HasPrefix(imageData, "http://") || strings.HasPrefix(imageData, "https://") {
		resp, err := DoDownloadRequest(imageData, "store generated image")
		if err != nil {
			return nil, "", fmt.Errorf("failed to download image: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("failed to download image: status %d", resp.StatusCode)
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageSize+1))
		if err != nil {
			return nil, "", fmt.Errorf("failed to read image data: %w", err)
		}

		mimeType := resp.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "image/png"
		}

		return data, mimeType, nil
	}

	// 处理纯 base64
	data, err := base64.StdEncoding.DecodeString(imageData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode base64: %w", err)
	}

	return data, "image/png", nil
}

// storeLocal 本地存储
func (s *ImageStorageService) storeLocal(data []byte, ext string) (string, error) {
	// 生成路径：YYYY/MM/DD/UUID.ext
	now := time.Now()
	datePath := now.Format("2006/01/02")
	filename := uuid.New().String() + ext

	fullPath := filepath.Join(s.localRoot, datePath, filename)

	// 创建目录
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// 返回访问 URL
	relativePath := filepath.Join(datePath, filename)
	// 转换为正斜杠（URL 格式）
	relativePath = strings.ReplaceAll(relativePath, "\\", "/")
	url := fmt.Sprintf("/api/v1/storage/images/%s", relativePath)

	common.SysLog(fmt.Sprintf("Image stored locally: %s", url))
	return url, nil
}

// storeS3 S3 存储
func (s *ImageStorageService) storeS3(data []byte, ext string) (string, error) {
	s.clientMutex.RLock()
	client := s.s3Client
	bucket := s.s3Bucket
	s.clientMutex.RUnlock()

	if client == nil {
		return "", fmt.Errorf("S3 client not initialized")
	}

	// 生成对象键：YYYY/MM/DD/UUID.ext
	now := time.Now()
	datePath := now.Format("2006/01/02")
	filename := uuid.New().String() + ext
	objectKey := fmt.Sprintf("%s/%s", datePath, filename)

	// 上传到 S3
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader := bytes.NewReader(data)
	_, err := client.PutObject(ctx, bucket, objectKey, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: s.getContentType(ext),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// 返回访问 URL
	var url string
	if s.s3PublicURL != "" {
		// 使用自定义公开 URL
		url = fmt.Sprintf("%s/%s", strings.TrimRight(s.s3PublicURL, "/"), objectKey)
	} else {
		// 使用 S3 默认 URL
		url = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, objectKey)
	}

	common.SysLog(fmt.Sprintf("Image stored to S3: %s", url))
	return url, nil
}

// getContentType 根据扩展名获取 Content-Type
func (s *ImageStorageService) getContentType(ext string) string {
	for mimeType, e := range allowedMimeTypes {
		if e == ext {
			return mimeType
		}
	}
	return "application/octet-stream"
}

// CheckHealth 健康检查
func (s *ImageStorageService) CheckHealth() error {
	if s.s3Enabled {
		s.clientMutex.RLock()
		client := s.s3Client
		bucket := s.s3Bucket
		s.clientMutex.RUnlock()

		if client == nil {
			return fmt.Errorf("S3 client not initialized")
		}

		// 检查 bucket 是否存在
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("failed to check S3 bucket: %w", err)
		}
		if !exists {
			return fmt.Errorf("S3 bucket does not exist: %s", bucket)
		}

		return nil
	}

	// 检查本地存储目录
	if _, err := os.Stat(s.localRoot); os.IsNotExist(err) {
		// 尝试创建目录
		if err := os.MkdirAll(s.localRoot, 0755); err != nil {
			return fmt.Errorf("failed to create local storage directory: %w", err)
		}
	}

	return nil
}
