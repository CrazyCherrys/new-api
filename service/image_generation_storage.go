package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/worker_setting"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

type imageGenerationAsset struct {
	data        []byte
	contentType string
	extension   string
}

func storeImageGenerationResult(ctx context.Context, taskId int, imageUrl string) string {
	cfg := worker_setting.GetWorkerSetting()
	if cfg == nil || cfg.StorageType != "s3" || strings.TrimSpace(imageUrl) == "" {
		return imageUrl
	}

	storedURL, err := uploadImageGenerationResultToS3(ctx, taskId, imageUrl, cfg)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to upload image generation task %d result to S3, fallback to original result: %v", taskId, err))
		return imageUrl
	}
	return storedURL
}

func uploadImageGenerationResultToS3(ctx context.Context, taskId int, imageUrl string, cfg *worker_setting.WorkerSetting) (string, error) {
	if err := validateImageS3Config(cfg); err != nil {
		return "", err
	}

	asset, err := loadImageGenerationAsset(ctx, imageUrl)
	if err != nil {
		return "", err
	}

	objectKey := buildImageGenerationObjectKey(taskId, cfg.S3PathPrefix, asset.extension)
	client := newImageS3Client(cfg)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(strings.TrimSpace(cfg.S3Bucket)),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(asset.data),
		ContentType: aws.String(asset.contentType),
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	return buildImageGenerationObjectURL(cfg, objectKey), nil
}

func validateImageS3Config(cfg *worker_setting.WorkerSetting) error {
	if strings.TrimSpace(cfg.S3Endpoint) == "" {
		return fmt.Errorf("s3 endpoint is empty")
	}
	if strings.TrimSpace(cfg.S3Bucket) == "" {
		return fmt.Errorf("s3 bucket is empty")
	}
	if strings.TrimSpace(cfg.S3AccessKey) == "" || strings.TrimSpace(cfg.S3SecretKey) == "" {
		return fmt.Errorf("s3 credentials are empty")
	}
	return nil
}

func loadImageGenerationAsset(ctx context.Context, imageUrl string) (*imageGenerationAsset, error) {
	if strings.HasPrefix(imageUrl, "data:") || !strings.HasPrefix(imageUrl, "http://") && !strings.HasPrefix(imageUrl, "https://") {
		return decodeImageGenerationBase64Asset(imageUrl)
	}
	return downloadImageGenerationAsset(ctx, imageUrl)
}

func decodeImageGenerationBase64Asset(raw string) (*imageGenerationAsset, error) {
	contentType := ""
	payload := strings.TrimSpace(raw)
	if strings.HasPrefix(payload, "data:") {
		commaIndex := strings.Index(payload, ",")
		if commaIndex < 0 {
			return nil, fmt.Errorf("invalid data URL")
		}
		header := payload[:commaIndex]
		payload = payload[commaIndex+1:]
		if mediaType, _, err := mime.ParseMediaType(strings.TrimPrefix(header, "data:")); err == nil && mediaType != "" {
			contentType = mediaType
		}
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		if decoded, rawErr := base64.RawStdEncoding.DecodeString(payload); rawErr == nil {
			data = decoded
		} else {
			return nil, fmt.Errorf("decode base64 image: %w", err)
		}
	}
	return normalizeImageGenerationAsset(data, contentType)
}

func downloadImageGenerationAsset(ctx context.Context, imageURL string) (*imageGenerationAsset, error) {
	resp, err := DoDownloadRequest(imageURL, "image_generation_result_storage")
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download image status: %d", resp.StatusCode)
	}

	maxBytes := int64(constant.MaxFileDownloadMB) * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 64 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("image size exceeds maximum allowed size: %dMB", constant.MaxFileDownloadMB)
	}

	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	return normalizeImageGenerationAsset(data, contentType)
}

func normalizeImageGenerationAsset(data []byte, contentType string) (*imageGenerationAsset, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("image data is empty")
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if _, webpErr := webp.DecodeConfig(bytes.NewReader(data)); webpErr == nil {
			format = "webp"
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid image data: %w", err)
	}

	contentType = imageContentTypeFromFormat(format)

	return &imageGenerationAsset{
		data:        data,
		contentType: contentType,
		extension:   imageExtensionFromContentType(contentType, format),
	}, nil
}

func imageContentTypeFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func imageExtensionFromContentType(contentType string, fallbackFormat string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}

	switch strings.ToLower(fallbackFormat) {
	case "jpeg":
		return ".jpg"
	case "png", "gif", "webp":
		return "." + strings.ToLower(fallbackFormat)
	default:
		return ".png"
	}
}

func buildImageGenerationObjectKey(taskId int, prefix string, extension string) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	day := time.Now().Format("20060102")
	filename := fmt.Sprintf("%d-%s%s", taskId, uuid.NewString(), extension)
	if cleanPrefix == "" {
		return path.Join("image-generation", day, filename)
	}
	return path.Join(cleanPrefix, "image-generation", day, filename)
}

func newImageS3Client(cfg *worker_setting.WorkerSetting) *s3.Client {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.S3Endpoint), "/")
	region := strings.TrimSpace(cfg.S3Region)
	if region == "" {
		region = "auto"
	}

	options := s3.Options{
		Region:       region,
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(strings.TrimSpace(cfg.S3AccessKey), strings.TrimSpace(cfg.S3SecretKey), "")),
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
	}
	if httpClient := GetHttpClient(); httpClient != nil {
		options.HTTPClient = httpClient
	}
	return s3.New(options)
}

func buildImageGenerationObjectURL(cfg *worker_setting.WorkerSetting, objectKey string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.S3Endpoint), "/")
	bucket := strings.Trim(strings.TrimSpace(cfg.S3Bucket), "/")
	escapedKey := pathEscapeObjectKey(objectKey)
	return endpoint + "/" + bucket + "/" + escapedKey
}

func pathEscapeObjectKey(objectKey string) string {
	parts := strings.Split(objectKey, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
