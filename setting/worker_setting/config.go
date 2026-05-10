package worker_setting

import (
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
	// AutoCleanupEnabled 自动清理开关
	AutoCleanupEnabled bool `json:"auto_cleanup_enabled"`
	// RetentionDays 保留天数
	RetentionDays int `json:"retention_days"`

	// MaxImageSize 单张参考图片最大大小（MB）
	MaxImageSize int `json:"max_image_size"`
}

// 默认配置
var workerSetting = WorkerSetting{
	MaxWorkers:               4,
	UserCustomKeyEnabled:     false,
	UserCustomBaseURLAllowed: false,
	StorageType:              "local",
	LocalStoragePath:         "",
	S3Endpoint:               "",
	S3Bucket:                 "",
	S3Region:                 "",
	S3AccessKey:              "",
	S3SecretKey:              "",
	S3PathPrefix:             "",
	S3URLMode:                "direct",
	S3PublicBaseURL:          "",
	ImageTimeout:             120,
	VideoTimeout:             600,
	RetryDelay:               5,
	MaxRetries:               3,
	PollingInterval:          5,
	AutoCleanupEnabled:       false,
	RetentionDays:            30,
	MaxImageSize:             10,
}

func init() {
	// 注册到全局配置管理器，key 前缀为 "worker_setting"
	config.GlobalConfig.Register("worker_setting", &workerSetting)
}

// GetWorkerSetting 获取 Worker 配置
func GetWorkerSetting() *WorkerSetting {
	return &workerSetting
}
