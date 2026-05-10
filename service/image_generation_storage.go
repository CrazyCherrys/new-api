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
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/worker_setting"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

const imageGenerationAssetURLPrefix = "/api/image-generation/files/"

const imageGenerationReferenceSubdir = "ref"

type imageGenerationAsset struct {
	data        []byte
	contentType string
	extension   string
}

type imageGenerationAssetLoader func(context.Context, string) (*imageGenerationAsset, error)

func storeImageGenerationResult(ctx context.Context, taskId int, imageUrl string) string {
	storedURL, err := storeImageGenerationAssetWithLoader(ctx, taskId, imageUrl, "", loadImageGenerationAsset)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to store image generation task %d result, fallback to original result: %v", taskId, err))
		return imageUrl
	}
	return storedURL
}

func storeImageGenerationReferenceImage(ctx context.Context, taskId int, imageUrl string) (string, error) {
	return storeImageGenerationAssetWithLoader(ctx, taskId, imageUrl, imageGenerationReferenceSubdir, loadImageGenerationReferenceAsset)
}

func storeImageGenerationReferenceImages(ctx context.Context, taskId int, refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if isImageGenerationStoredReferenceURL(ref) {
			out = append(out, ref)
			continue
		}
		stored, err := storeImageGenerationReferenceImage(ctx, taskId, ref)
		if err != nil {
			cleanupStoredImageGenerationAssets(out)
			return nil, err
		}
		out = append(out, stored)
	}
	return out, nil
}

func cleanupStoredImageGenerationAssets(refs []string) {
	cfg := worker_setting.GetWorkerSetting()
	if cfg == nil {
		return
	}
	for _, ref := range refs {
		if err := deleteImageFile(ref, cfg); err != nil {
			common.SysLog(fmt.Sprintf("Failed to cleanup stored image generation asset %q: %v", ref, err))
		}
	}
}

func storeImageGenerationAsset(ctx context.Context, taskId int, imageUrl string, subdir string) (string, error) {
	return storeImageGenerationAssetWithLoader(ctx, taskId, imageUrl, subdir, loadImageGenerationAsset)
}

func storeImageGenerationAssetWithLoader(ctx context.Context, taskId int, imageUrl string, subdir string, loader imageGenerationAssetLoader) (string, error) {
	cfg := worker_setting.GetWorkerSetting()
	if cfg == nil || strings.TrimSpace(imageUrl) == "" {
		return imageUrl, nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.StorageType)) {
	case "s3":
		return uploadImageGenerationAssetToS3(ctx, taskId, imageUrl, cfg, subdir, loader)
	case "local":
		return saveImageGenerationAssetLocally(ctx, taskId, imageUrl, cfg, subdir, loader)
	default:
		return imageUrl, nil
	}
}

func uploadImageGenerationResultToS3(ctx context.Context, taskId int, imageUrl string, cfg *worker_setting.WorkerSetting) (string, error) {
	return uploadImageGenerationAssetToS3(ctx, taskId, imageUrl, cfg, "", loadImageGenerationAsset)
}

func uploadImageGenerationAssetToS3(ctx context.Context, taskId int, imageUrl string, cfg *worker_setting.WorkerSetting, subdir string, loader imageGenerationAssetLoader) (string, error) {
	if err := validateImageS3Config(cfg); err != nil {
		return "", err
	}

	if loader == nil {
		loader = loadImageGenerationAsset
	}

	asset, err := loader(ctx, imageUrl)
	if err != nil {
		return "", err
	}

	objectKey := buildImageGenerationObjectKeyWithSubdir(taskId, cfg.S3PathPrefix, subdir, asset.extension)
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
	switch strings.ToLower(strings.TrimSpace(cfg.S3URLMode)) {
	case "", "direct":
		return nil
	case "cdn":
		publicBase := strings.TrimSpace(cfg.S3PublicBaseURL)
		if publicBase == "" {
			return fmt.Errorf("s3 public base url is empty")
		}
		parsed, err := url.Parse(publicBase)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("s3 public base url is invalid")
		}
		return nil
	default:
		return fmt.Errorf("invalid s3 url mode")
	}
}

func saveImageGenerationResultLocally(ctx context.Context, taskId int, imageUrl string, cfg *worker_setting.WorkerSetting) (string, error) {
	return saveImageGenerationAssetLocally(ctx, taskId, imageUrl, cfg, "", loadImageGenerationAsset)
}

func saveImageGenerationAssetLocally(ctx context.Context, taskId int, imageUrl string, cfg *worker_setting.WorkerSetting, subdir string, loader imageGenerationAssetLoader) (string, error) {
	if loader == nil {
		loader = loadImageGenerationAsset
	}

	asset, err := loader(ctx, imageUrl)
	if err != nil {
		return "", err
	}

	objectKey := buildImageGenerationObjectKeyWithSubdir(taskId, "", subdir, asset.extension)
	fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", fmt.Errorf("create local image directory: %w", err)
	}
	if err := os.WriteFile(fullPath, asset.data, 0o644); err != nil {
		return "", fmt.Errorf("write local image file: %w", err)
	}

	return buildImageGenerationLocalObjectURL(objectKey), nil
}

func loadImageGenerationAsset(ctx context.Context, imageUrl string) (*imageGenerationAsset, error) {
	if strings.HasPrefix(imageUrl, "data:") || !strings.HasPrefix(imageUrl, "http://") && !strings.HasPrefix(imageUrl, "https://") {
		return decodeImageGenerationBase64Asset(imageUrl)
	}
	return downloadImageGenerationAsset(ctx, imageUrl)
}

func loadImageGenerationReferenceAsset(ctx context.Context, ref string) (*imageGenerationAsset, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("reference image is empty")
	}

	if objectKey, ok := imageGenerationLocalAssetKeyFromURL(ref); ok {
		cfg := worker_setting.GetWorkerSetting()
		fullPath, err := imageGenerationLocalAssetPath(cfg, objectKey)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read local image file: %w", err)
		}
		return normalizeImageGenerationAsset(data, mime.TypeByExtension(filepath.Ext(fullPath)))
	}

	cfg := worker_setting.GetWorkerSetting()
	if cfg != nil {
		if objectKey, ok := imageGenerationS3ObjectKeyFromURL(ref, cfg); ok {
			client := newImageS3Client(cfg)
			resp, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(strings.TrimSpace(cfg.S3Bucket)),
				Key:    aws.String(objectKey),
			})
			if err != nil {
				return nil, fmt.Errorf("get s3 image object: %w", err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read s3 image object: %w", err)
			}
			contentType := ""
			if resp.ContentType != nil {
				contentType = strings.TrimSpace(*resp.ContentType)
			}
			return normalizeImageGenerationAsset(data, contentType)
		}
	}

	return loadImageGenerationAsset(ctx, ref)
}

func referenceImageAsDataURL(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	if strings.HasPrefix(ref, "data:") {
		return ref, nil
	}
	if !isImageGenerationStoredReferenceURL(ref) {
		return ref, nil
	}

	asset, err := loadImageGenerationReferenceAsset(ctx, ref)
	if err != nil {
		return "", err
	}

	contentType := strings.TrimSpace(asset.contentType)
	if contentType == "" {
		contentType = "image/png"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(asset.data), nil
}

func isImageGenerationStoredReferenceURL(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	if _, ok := imageGenerationLocalAssetKeyFromURL(ref); ok {
		return true
	}
	cfg := worker_setting.GetWorkerSetting()
	if cfg == nil {
		return false
	}
	_, ok := imageGenerationS3ObjectKeyFromURL(ref, cfg)
	return ok
}

func imageGenerationS3ObjectKeyFromURL(imageUrl string, cfg *worker_setting.WorkerSetting) (string, bool) {
	if cfg == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(imageUrl)
	if trimmed == "" {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	for _, baseURL := range imageGenerationS3ObjectBaseURLs(cfg) {
		objectKey, ok := imageGenerationObjectKeyFromBaseURL(parsed, baseURL)
		if ok {
			return objectKey, true
		}
	}
	return "", false
}

func unescapeImageGenerationObjectKey(objectKey string) (string, error) {
	parts := strings.Split(strings.TrimSpace(objectKey), "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("empty object key")
	}
	for i, part := range parts {
		unescaped, err := url.PathUnescape(part)
		if err != nil {
			return "", err
		}
		parts[i] = unescaped
	}
	return strings.Join(parts, "/"), nil
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
	return buildImageGenerationObjectKeyWithSubdir(taskId, prefix, "", extension)
}

func buildImageGenerationObjectKeyWithSubdir(taskId int, prefix string, subdir string, extension string) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	day := time.Now().Format("20060102")
	filename := fmt.Sprintf("%d-%s%s", taskId, uuid.NewString(), extension)
	parts := []string{"image-generation"}
	if strings.TrimSpace(subdir) != "" {
		parts = append(parts, strings.Trim(strings.TrimSpace(subdir), "/"))
	}
	parts = append(parts, day, filename)
	if cleanPrefix == "" {
		return path.Join(parts...)
	}
	return path.Join(append([]string{cleanPrefix}, parts...)...)
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

func imageGenerationS3ObjectBaseURLs(cfg *worker_setting.WorkerSetting) []string {
	if cfg == nil {
		return nil
	}

	bases := make([]string, 0, 3)
	for _, directBase := range buildImageGenerationDirectObjectBaseURLs(cfg) {
		isDuplicate := false
		for _, base := range bases {
			if base == directBase {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			bases = append(bases, directBase)
		}
	}
	if publicBase := strings.TrimRight(strings.TrimSpace(cfg.S3PublicBaseURL), "/"); publicBase != "" {
		isDuplicate := false
		for _, base := range bases {
			if base == publicBase {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			bases = append(bases, publicBase)
		}
	}
	return bases
}

func imageGenerationObjectKeyFromBaseURL(assetURL *url.URL, baseURL string) (string, bool) {
	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(assetURL.Scheme, parsedBaseURL.Scheme) || assetURL.Host != parsedBaseURL.Host {
		return "", false
	}

	basePath := strings.TrimRight(parsedBaseURL.Path, "/")
	assetPath := assetURL.Path
	var candidate string
	switch {
	case basePath == "":
		candidate = strings.Trim(assetPath, "/")
	case assetPath == basePath:
		return "", false
	case strings.HasPrefix(assetPath, basePath+"/"):
		candidate = strings.TrimPrefix(assetPath, basePath+"/")
	default:
		return "", false
	}
	if strings.TrimSpace(candidate) == "" {
		return "", false
	}

	objectKey, err := unescapeImageGenerationObjectKey(candidate)
	if err != nil {
		return "", false
	}
	return objectKey, true
}

func buildImageGenerationDirectObjectBaseURLs(cfg *worker_setting.WorkerSetting) []string {
	if cfg == nil {
		return nil
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.S3Endpoint), "/")
	bucket := strings.Trim(strings.TrimSpace(cfg.S3Bucket), "/")
	if endpoint == "" || bucket == "" {
		return nil
	}

	bases := []string{endpoint + "/" + bucket}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return bases
	}

	legacyHosts := make([]string, 0, 2)
	if strings.Contains(parsedEndpoint.Host, "-internal.") {
		legacyHosts = append(legacyHosts, strings.Replace(parsedEndpoint.Host, "-internal.", ".", 1))
	}
	if strings.Contains(parsedEndpoint.Host, ".internal.") {
		legacyHosts = append(legacyHosts, strings.Replace(parsedEndpoint.Host, ".internal.", ".", 1))
	}

	for _, host := range legacyHosts {
		clone := *parsedEndpoint
		clone.Host = host
		legacyEndpoint := strings.TrimRight(clone.String(), "/")
		baseURL := legacyEndpoint + "/" + bucket
		isDuplicate := false
		for _, base := range bases {
			if base == baseURL {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			bases = append(bases, baseURL)
		}
	}
	return bases
}

func buildImageGenerationObjectURL(cfg *worker_setting.WorkerSetting, objectKey string) string {
	escapedKey := pathEscapeObjectKey(objectKey)
	mode := strings.ToLower(strings.TrimSpace(cfg.S3URLMode))
	if mode == "cdn" {
		if publicBase := strings.TrimRight(strings.TrimSpace(cfg.S3PublicBaseURL), "/"); publicBase != "" {
			return publicBase + "/" + escapedKey
		}
	}
	directBases := buildImageGenerationDirectObjectBaseURLs(cfg)
	if len(directBases) == 0 {
		return escapedKey
	}
	return directBases[0] + "/" + escapedKey
}

func buildImageGenerationLocalObjectURL(objectKey string) string {
	return imageGenerationAssetURLPrefix + pathEscapeObjectKey(objectKey)
}

func imageGenerationLocalStorageBasePath(cfg *worker_setting.WorkerSetting) string {
	if cfg != nil && strings.TrimSpace(cfg.LocalStoragePath) != "" {
		return strings.TrimSpace(cfg.LocalStoragePath)
	}
	return filepath.Join(os.TempDir(), "new-api-image-generation")
}

func sanitizeImageGenerationLocalAssetPath(raw string) (string, error) {
	clean := path.Clean("/" + strings.TrimPrefix(raw, "/"))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("invalid asset path")
	}
	if !strings.HasPrefix(clean, "image-generation/") {
		return "", fmt.Errorf("invalid asset path")
	}
	return clean, nil
}

func imageGenerationLocalAssetPath(cfg *worker_setting.WorkerSetting, assetPath string) (string, error) {
	clean, err := sanitizeImageGenerationLocalAssetPath(assetPath)
	if err != nil {
		return "", err
	}
	basePath := imageGenerationLocalStorageBasePath(cfg)
	fullPath := filepath.Join(basePath, filepath.FromSlash(clean))
	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, fullAbs)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("invalid asset path")
	}
	return fullAbs, nil
}

func imageGenerationLocalAssetKeyFromURL(imageUrl string) (string, bool) {
	trimmed := strings.TrimSpace(imageUrl)
	if strings.HasPrefix(trimmed, imageGenerationAssetURLPrefix) {
		return strings.TrimPrefix(trimmed[len(imageGenerationAssetURLPrefix):], "/"), true
	}
	return "", false
}

func CanAccessImageGenerationLocalAsset(userId int, assetPath string) (bool, error) {
	clean, err := sanitizeImageGenerationLocalAssetPath(assetPath)
	if err != nil {
		return false, nil
	}

	assetURL := buildImageGenerationLocalObjectURL(clean)
	var count int64
	if err := model.DB.Model(&model.ImageGenerationTask{}).
		Where("user_id = ? AND image_url = ?", userId, assetURL).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func CanAccessApprovedInspirationLocalAsset(assetPath string) (bool, error) {
	clean, err := sanitizeImageGenerationLocalAssetPath(assetPath)
	if err != nil {
		return false, nil
	}

	assetURL := buildImageGenerationLocalObjectURL(clean)
	var count int64
	if err := model.DB.Table("image_generation_tasks AS t").
		Joins("JOIN image_creative_submissions AS s ON s.task_id = t.id").
		Where("s.status = ? AND t.status = ? AND t.image_url = ?", model.CreativeSubmissionStatusApproved, model.ImageTaskStatusSuccess, assetURL).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func CanAccessApprovedCreativeSpaceLocalAsset(assetPath string) (bool, error) {
	return CanAccessApprovedInspirationLocalAsset(assetPath)
}

func OpenImageGenerationLocalAsset(assetPath string) (*os.File, string, error) {
	cfg := worker_setting.GetWorkerSetting()
	fullPath, err := imageGenerationLocalAssetPath(cfg, assetPath)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, "", err
	}
	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return file, contentType, nil
}

func pathEscapeObjectKey(objectKey string) string {
	parts := strings.Split(objectKey, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
