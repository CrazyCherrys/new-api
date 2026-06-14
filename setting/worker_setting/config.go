package worker_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// WorkerSetting Worker 相关配置
type WorkerSetting struct {
	// MaxWorkers 最大并发 Worker 数量
	MaxWorkers int `json:"max_workers"`

	// UserCustomKeyEnabled 是否允许用户自定义 API 密钥
	UserCustomKeyEnabled bool `json:"user_custom_key_enabled"`
	// UserCustomBaseURLAllowed 是否允许用户自定义 API 地址
	UserCustomBaseURLAllowed bool `json:"user_custom_base_url_allowed"`
	// UserDefaultBaseURL 管理员默认 API 地址，仅在允许自定义密钥但不允许自定义地址时生效
	UserDefaultBaseURL string `json:"user_default_base_url"`

	// StorageType 存储类型: local / s3
	StorageType string `json:"storage_type"`
	// LocalStoragePath 本地存储路径（空使用系统临时目录）
	LocalStoragePath string `json:"local_storage_path"`

	// S3 对象存储配置
	S3Endpoint      string `json:"s3_endpoint"`
	S3Bucket        string `json:"s3_bucket"`
	S3Region        string `json:"s3_region"`
	S3AccessKey     string `json:"s3_access_key"`
	S3SecretKey     string `json:"s3_secret_key"`
	S3PathPrefix    string `json:"s3_path_prefix"`
	S3URLMode       string `json:"s3_url_mode"`
	S3PublicBaseURL string `json:"s3_public_base_url"`

	// ResultStorageType 结果图存储类型: local / s3
	ResultStorageType string `json:"result_storage_type"`
	// ResultLocalStoragePath 结果图本地存储路径（空使用系统临时目录）
	ResultLocalStoragePath string `json:"result_local_storage_path"`

	// ResultS3 对象存储配置
	ResultS3Endpoint      string `json:"result_s3_endpoint"`
	ResultS3Bucket        string `json:"result_s3_bucket"`
	ResultS3Region        string `json:"result_s3_region"`
	ResultS3AccessKey     string `json:"result_s3_access_key"`
	ResultS3SecretKey     string `json:"result_s3_secret_key"`
	ResultS3PathPrefix    string `json:"result_s3_path_prefix"`
	ResultS3URLMode       string `json:"result_s3_url_mode"`
	ResultS3PublicBaseURL string `json:"result_s3_public_base_url"`

	// ReferenceStorageType 参考图存储类型: local / s3
	ReferenceStorageType string `json:"reference_storage_type"`
	// ReferenceLocalStoragePath 参考图本地存储路径（空使用系统临时目录）
	ReferenceLocalStoragePath string `json:"reference_local_storage_path"`

	// ReferenceS3 对象存储配置
	ReferenceS3Endpoint      string `json:"reference_s3_endpoint"`
	ReferenceS3Bucket        string `json:"reference_s3_bucket"`
	ReferenceS3Region        string `json:"reference_s3_region"`
	ReferenceS3AccessKey     string `json:"reference_s3_access_key"`
	ReferenceS3SecretKey     string `json:"reference_s3_secret_key"`
	ReferenceS3PathPrefix    string `json:"reference_s3_path_prefix"`
	ReferenceS3URLMode       string `json:"reference_s3_url_mode"`
	ReferenceS3PublicBaseURL string `json:"reference_s3_public_base_url"`

	// ImageTimeout 图片任务超时时间（秒）
	ImageTimeout int `json:"image_timeout"`
	// VideoTimeout 视频任务超时时间（秒）
	VideoTimeout int `json:"video_timeout"`

	// RetryDelay 图片生成失败后重试间隔（秒）
	RetryDelay int `json:"retry_delay"`
	// MaxRetries 图片生成最大重试次数
	MaxRetries int `json:"max_retries"`

	// PollingInterval 轮询间隔（秒）
	PollingInterval int `json:"polling_interval"`
	// InspirationPageCacheTTL /inspiration 后续游标页缓存时长（秒）
	InspirationPageCacheTTL int `json:"inspiration_page_cache_ttl"`
	// InspirationFirstPageCacheTTL /inspiration 首页缓存时长（秒）
	InspirationFirstPageCacheTTL int `json:"inspiration_first_page_cache_ttl"`
	// AutoCleanupEnabled 自动清理开关
	AutoCleanupEnabled bool `json:"auto_cleanup_enabled"`
	// RetentionDays 保留天数
	RetentionDays int `json:"retention_days"`
	// ReferenceAutoCleanupEnabled 参考图自动清理开关
	ReferenceAutoCleanupEnabled bool `json:"reference_auto_cleanup_enabled"`
	// ReferenceRetentionDays 参考图保留天数
	ReferenceRetentionDays int `json:"reference_retention_days"`

	// MaxImageSize 单张参考图片最大大小（MB）
	MaxImageSize int `json:"max_image_size"`
}

// 默认配置
var workerSetting = WorkerSetting{
	MaxWorkers:                   4,
	UserCustomKeyEnabled:         false,
	UserCustomBaseURLAllowed:     false,
	UserDefaultBaseURL:           "",
	StorageType:                  "local",
	LocalStoragePath:             "",
	S3Endpoint:                   "",
	S3Bucket:                     "",
	S3Region:                     "",
	S3AccessKey:                  "",
	S3SecretKey:                  "",
	S3PathPrefix:                 "",
	S3URLMode:                    "direct",
	S3PublicBaseURL:              "",
	ResultStorageType:            "",
	ResultLocalStoragePath:       "",
	ResultS3Endpoint:             "",
	ResultS3Bucket:               "",
	ResultS3Region:               "",
	ResultS3AccessKey:            "",
	ResultS3SecretKey:            "",
	ResultS3PathPrefix:           "",
	ResultS3URLMode:              "",
	ResultS3PublicBaseURL:        "",
	ReferenceStorageType:         "",
	ReferenceLocalStoragePath:    "",
	ReferenceS3Endpoint:          "",
	ReferenceS3Bucket:            "",
	ReferenceS3Region:            "",
	ReferenceS3AccessKey:         "",
	ReferenceS3SecretKey:         "",
	ReferenceS3PathPrefix:        "",
	ReferenceS3URLMode:           "",
	ReferenceS3PublicBaseURL:     "",
	ImageTimeout:                 120,
	VideoTimeout:                 600,
	RetryDelay:                   5,
	MaxRetries:                   3,
	PollingInterval:              5,
	InspirationPageCacheTTL:      common.GetEnvOrDefault("INSPIRATION_ASSET_LIST_CACHE_TTL", 300),
	InspirationFirstPageCacheTTL: common.GetEnvOrDefault("INSPIRATION_ASSET_FIRST_PAGE_CACHE_TTL", 900),
	AutoCleanupEnabled:           false,
	RetentionDays:                30,
	ReferenceAutoCleanupEnabled:  false,
	ReferenceRetentionDays:       7,
	MaxImageSize:                 10,
}

func init() {
	// 注册到全局配置管理器，key 前缀为 "worker_setting"
	config.GlobalConfig.Register("worker_setting", &workerSetting)
}

// GetWorkerSetting 获取 Worker 配置
func GetWorkerSetting() *WorkerSetting {
	return &workerSetting
}

func (ws *WorkerSetting) effectiveStorageType(storageType string) string {
	trimmed := strings.ToLower(strings.TrimSpace(storageType))
	if trimmed != "" {
		return trimmed
	}
	return "local"
}

func (ws *WorkerSetting) fallbackString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (ws *WorkerSetting) EffectiveResultStorageType() string {
	if ws == nil {
		return "local"
	}
	return ws.effectiveStorageType(ws.fallbackString(ws.ResultStorageType, ws.StorageType))
}

func (ws *WorkerSetting) EffectiveUserDefaultBaseURL() string {
	if ws == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(ws.UserDefaultBaseURL), "/")
}

func (ws *WorkerSetting) EffectiveReferenceStorageType() string {
	if ws == nil {
		return "local"
	}
	return ws.effectiveStorageType(ws.fallbackString(ws.ReferenceStorageType, ws.ResultStorageType, ws.StorageType))
}

func (ws *WorkerSetting) EffectiveResultLocalStoragePath() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultLocalStoragePath, ws.LocalStoragePath)
}

func (ws *WorkerSetting) EffectiveResultS3Endpoint() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3Endpoint, ws.S3Endpoint)
}

func (ws *WorkerSetting) EffectiveResultS3Bucket() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3Bucket, ws.S3Bucket)
}

func (ws *WorkerSetting) EffectiveResultS3Region() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3Region, ws.S3Region)
}

func (ws *WorkerSetting) EffectiveResultS3AccessKey() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3AccessKey, ws.S3AccessKey)
}

func (ws *WorkerSetting) EffectiveResultS3SecretKey() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3SecretKey, ws.S3SecretKey)
}

func (ws *WorkerSetting) EffectiveResultS3PathPrefix() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3PathPrefix, ws.S3PathPrefix)
}

func (ws *WorkerSetting) EffectiveResultS3URLMode() string {
	if ws == nil {
		return "direct"
	}
	return ws.fallbackString(ws.ResultS3URLMode, ws.S3URLMode)
}

func (ws *WorkerSetting) EffectiveResultS3PublicBaseURL() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ResultS3PublicBaseURL, ws.S3PublicBaseURL)
}

func (ws *WorkerSetting) EffectiveReferenceLocalStoragePath() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceLocalStoragePath, ws.ResultLocalStoragePath, ws.LocalStoragePath)
}

func (ws *WorkerSetting) EffectiveReferenceS3Endpoint() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3Endpoint, ws.ResultS3Endpoint, ws.S3Endpoint)
}

func (ws *WorkerSetting) EffectiveReferenceS3Bucket() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3Bucket, ws.ResultS3Bucket, ws.S3Bucket)
}

func (ws *WorkerSetting) EffectiveReferenceS3Region() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3Region, ws.ResultS3Region, ws.S3Region)
}

func (ws *WorkerSetting) EffectiveReferenceS3AccessKey() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3AccessKey, ws.ResultS3AccessKey, ws.S3AccessKey)
}

func (ws *WorkerSetting) EffectiveReferenceS3SecretKey() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3SecretKey, ws.ResultS3SecretKey, ws.S3SecretKey)
}

func (ws *WorkerSetting) EffectiveReferenceS3PathPrefix() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3PathPrefix, ws.ResultS3PathPrefix, ws.S3PathPrefix)
}

func (ws *WorkerSetting) EffectiveReferenceS3URLMode() string {
	if ws == nil {
		return "direct"
	}
	return ws.fallbackString(ws.ReferenceS3URLMode, ws.ResultS3URLMode, ws.S3URLMode)
}

func (ws *WorkerSetting) EffectiveReferenceS3PublicBaseURL() string {
	if ws == nil {
		return ""
	}
	return ws.fallbackString(ws.ReferenceS3PublicBaseURL, ws.ResultS3PublicBaseURL, ws.S3PublicBaseURL)
}
