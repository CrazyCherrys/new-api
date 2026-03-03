package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type ImageGenerationModelSetting struct {
	DisplayName        string   `json:"display_name"`         // 模型展示名称
	RequestModelID     string   `json:"request_model_id"`     // 实际请求上游的模型名
	RequestEndpoint    string   `json:"request_endpoint"`     // 请求端点类型（openai/gemini/openai_mod）
	ModelType          string   `json:"model_type"`           // 模型类型（image/video/text）
	DefaultResolution  string   `json:"default_resolution"`   // 模型默认分辨率
	DefaultAspectRatio string   `json:"default_aspect_ratio"` // 模型默认宽高比
	Resolutions        []string `json:"resolutions"`          // 可用分辨率
	AspectRatios       []string `json:"aspect_ratios"`        // 可用宽高比
	Durations          []string `json:"durations"`            // 可用时长（视频模型）
	MaxImageCount      int      `json:"max_image_count"`      // 模型单次最大生成数量
	RpmLimit           int      `json:"rpm_limit"`            // 模型 RPM 限制
	RpmEnabled         bool     `json:"rpm_enabled"`          // 是否启用模型 RPM 限制
}

// ImageGenerationSetting 图像生成配置
type ImageGenerationSetting struct {
	// 存储配置
	StorageType         string `json:"storage_type"`           // local 或 s3
	StorageLocalPath    string `json:"storage_local_path"`     // 本地存储路径
	StorageS3Endpoint   string `json:"storage_s3_endpoint"`    // S3 端点
	StorageS3Bucket     string `json:"storage_s3_bucket"`      // S3 桶名
	StorageS3AccessKey  string `json:"storage_s3_access_key"`  // S3 访问密钥
	StorageS3SecretKey  string `json:"storage_s3_secret_key"`  // S3 密钥
	StorageS3Region     string `json:"storage_s3_region"`      // S3 区域
	StorageS3PathPrefix string `json:"storage_s3_path_prefix"` // S3 路径前缀

	// 生成参数配置
	ImageTimeoutSeconds       int `json:"image_timeout_seconds"`        // 图像生成超时时间（秒）
	ImageMaxRetryAttempts     int `json:"image_max_retry_attempts"`     // 最大重试次数
	ImageRetryIntervalSeconds int `json:"image_retry_interval_seconds"` // 重试间隔（秒）
	ImageWorkerCount          int `json:"image_worker_count"`           // Worker 数量
	ImageStaleAfterMinutes    int `json:"image_stale_after_minutes"`    // 僵尸任务判定时间（分钟）
	RpmLimit                  int `json:"rpm_limit"`                    // 单模型请求频率限制（每分钟）

	// 模型配置
	EnabledModels      []string                               `json:"enabled_models"`       // 启用的模型列表
	DefaultModel       string                                 `json:"default_model"`        // 默认模型
	DefaultResolution  string                                 `json:"default_resolution"`   // 默认分辨率
	DefaultAspectRatio string                                 `json:"default_aspect_ratio"` // 默认宽高比
	MaxImageCount      int                                    `json:"max_image_count"`      // 单次最大生成数量
	ModelSettings      map[string]ImageGenerationModelSetting `json:"model_settings"`       // 每模型独立配置
}

var defaultImageGenerationSetting = ImageGenerationSetting{
	// 存储配置默认值
	StorageType:         "local",
	StorageLocalPath:    "./data/images",
	StorageS3Endpoint:   "",
	StorageS3Bucket:     "",
	StorageS3AccessKey:  "",
	StorageS3SecretKey:  "",
	StorageS3Region:     "us-east-1",
	StorageS3PathPrefix: "generated-images",

	// 生成参数默认值
	ImageTimeoutSeconds:       300, // 5分钟
	ImageMaxRetryAttempts:     3,   // 最多重试3次
	ImageRetryIntervalSeconds: 10,  // 重试间隔10秒
	ImageWorkerCount:          2,   // 2个Worker
	ImageStaleAfterMinutes:    10,  // 10分钟后判定为僵尸任务
	RpmLimit:                  60,  // 每分钟60次请求

	// 模型配置默认值
	EnabledModels: []string{
		"dall-e-3",
		"dall-e-2",
		"stable-diffusion-xl",
	},
	DefaultModel:       "dall-e-3",
	DefaultResolution:  "1024x1024",
	DefaultAspectRatio: "1:1",
	MaxImageCount:      10,
	ModelSettings: map[string]ImageGenerationModelSetting{
		"dall-e-3": {
			DisplayName:        "DALL-E 3",
			RequestModelID:     "dall-e-3",
			RequestEndpoint:    "openai",
			ModelType:          "image",
			DefaultResolution:  "1024x1024",
			DefaultAspectRatio: "1:1",
			Resolutions: []string{
				"1024x1024",
				"1152x896",
				"896x1152",
				"1216x832",
				"832x1216",
				"1344x768",
				"768x1344",
				"1536x640",
				"640x1536",
			},
			AspectRatios: []string{
				"1:1",
				"16:9",
				"9:16",
				"4:3",
				"3:4",
				"21:9",
				"9:21",
			},
			MaxImageCount: 10,
			RpmLimit:      60,
			RpmEnabled:    false,
		},
		"dall-e-2": {
			DisplayName:        "DALL-E 2",
			RequestModelID:     "dall-e-2",
			RequestEndpoint:    "openai",
			ModelType:          "image",
			DefaultResolution:  "1024x1024",
			DefaultAspectRatio: "1:1",
			Resolutions: []string{
				"1024x1024",
			},
			AspectRatios: []string{
				"1:1",
			},
			MaxImageCount: 10,
			RpmLimit:      60,
			RpmEnabled:    false,
		},
		"stable-diffusion-xl": {
			DisplayName:        "Stable Diffusion XL",
			RequestModelID:     "stable-diffusion-xl",
			RequestEndpoint:    "openai",
			ModelType:          "image",
			DefaultResolution:  "1024x1024",
			DefaultAspectRatio: "1:1",
			Resolutions: []string{
				"1024x1024",
				"1152x896",
				"896x1152",
				"1216x832",
				"832x1216",
				"1344x768",
				"768x1344",
				"1536x640",
				"640x1536",
			},
			AspectRatios: []string{
				"1:1",
				"16:9",
				"9:16",
				"4:3",
				"3:4",
				"21:9",
				"9:21",
			},
			MaxImageCount: 10,
			RpmLimit:      60,
			RpmEnabled:    false,
		},
	},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("image_generation_setting", &defaultImageGenerationSetting)
}

// GetImageGenerationSetting 获取图像生成配置
func GetImageGenerationSetting() *ImageGenerationSetting {
	return &defaultImageGenerationSetting
}
