package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ImageCapabilityGeneration   = "image_generation"
	ImageCapabilityEditing      = "image_editing"
	VideoCapabilityImageToVideo = "image_to_video"
	VideoCapabilityTextToVideo  = "text_to_video"
)

var defaultImageCapabilities = []string{
	ImageCapabilityGeneration,
	ImageCapabilityEditing,
}

var defaultVideoCapabilities = []string{
	VideoCapabilityImageToVideo,
}

// ModelMapping 模型映射配置
type ModelMapping struct {
	Id                int    `json:"id"`
	RequestModel      string `json:"request_model" gorm:"size:128;not null;uniqueIndex:uk_request_model"`
	ActualModel       string `json:"actual_model" gorm:"size:128;not null"`
	DisplayName       string `json:"display_name" gorm:"size:255;not null"`
	ModelSeries       string `json:"model_series" gorm:"size:64;default:'';index"`
	ModelType         int    `json:"model_type" gorm:"default:1;index"` // 1=对话 2=绘画 3=视频 4=音频
	Description       string `json:"description" gorm:"type:text"`
	Status            int    `json:"status" gorm:"default:1;index"`
	Priority          int    `json:"priority" gorm:"default:0"`
	RequestEndpoint   string `json:"request_endpoint" gorm:"size:32;default:''"` // openai, openai-response, gemini
	Resolutions       string `json:"resolutions" gorm:"type:text"`               // JSON array: ["1K","2K","4K"]
	AspectRatios      string `json:"aspect_ratios" gorm:"type:text"`             // JSON array: ["1:1","16:9",...]
	ImageCapabilities string `json:"image_capabilities" gorm:"type:text"`        // JSON array: ["image_generation","image_editing"]
	VideoCapabilities string `json:"video_capabilities" gorm:"type:text"`        // JSON array: ["image_to_video","text_to_video"]
	DurationOptions   string `json:"duration_options" gorm:"type:text"`          // JSON array: [5,10]
	CreatedTime       int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64  `json:"updated_time" gorm:"bigint"`
}

func DefaultImageCapabilities() []string {
	return append([]string(nil), defaultImageCapabilities...)
}

func DefaultVideoCapabilities() []string {
	return append([]string(nil), defaultVideoCapabilities...)
}

func normalizeImageCapability(capability string) string {
	return strings.ToLower(strings.TrimSpace(capability))
}

func normalizeVideoCapability(capability string) string {
	return strings.ToLower(strings.TrimSpace(capability))
}

func parseImageCapabilities(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var capabilities []string
	if err := common.UnmarshalJsonStr(raw, &capabilities); err != nil {
		return nil, fmt.Errorf("failed to parse image capabilities: %w", err)
	}

	normalized := make([]string, 0, len(capabilities))
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		value := normalizeImageCapability(capability)
		if value == "" {
			continue
		}
		switch value {
		case ImageCapabilityGeneration, ImageCapabilityEditing:
		default:
			return nil, fmt.Errorf("unsupported image capability: %s", capability)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized, nil
}

func NormalizeImageCapabilities(raw string) (string, error) {
	capabilities, err := parseImageCapabilities(raw)
	if err != nil {
		return "", err
	}
	if len(capabilities) == 0 {
		return "", nil
	}

	data, err := common.Marshal(capabilities)
	if err != nil {
		return "", fmt.Errorf("failed to marshal image capabilities: %w", err)
	}
	return string(data), nil
}

func EffectiveImageCapabilities(raw string) ([]string, error) {
	capabilities, err := parseImageCapabilities(raw)
	if err != nil {
		return nil, err
	}
	if len(capabilities) == 0 {
		return DefaultImageCapabilities(), nil
	}
	return capabilities, nil
}

func HasImageCapability(raw string, target string) (bool, error) {
	capabilities, err := EffectiveImageCapabilities(raw)
	if err != nil {
		return false, err
	}
	normalizedTarget := normalizeImageCapability(target)
	for _, capability := range capabilities {
		if capability == normalizedTarget {
			return true, nil
		}
	}
	return false, nil
}

func parseVideoCapabilities(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var capabilities []string
	if err := common.UnmarshalJsonStr(raw, &capabilities); err != nil {
		return nil, fmt.Errorf("failed to parse video capabilities: %w", err)
	}

	normalized := make([]string, 0, len(capabilities))
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		value := normalizeVideoCapability(capability)
		if value == "" {
			continue
		}
		switch value {
		case VideoCapabilityImageToVideo, VideoCapabilityTextToVideo:
		default:
			return nil, fmt.Errorf("unsupported video capability: %s", capability)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized, nil
}

func NormalizeVideoCapabilities(raw string) (string, error) {
	capabilities, err := parseVideoCapabilities(raw)
	if err != nil {
		return "", err
	}
	if len(capabilities) == 0 {
		return "", nil
	}

	data, err := common.Marshal(capabilities)
	if err != nil {
		return "", fmt.Errorf("failed to marshal video capabilities: %w", err)
	}
	return string(data), nil
}

func EffectiveVideoCapabilities(raw string) ([]string, error) {
	capabilities, err := parseVideoCapabilities(raw)
	if err != nil {
		return nil, err
	}
	if len(capabilities) == 0 {
		return DefaultVideoCapabilities(), nil
	}
	return capabilities, nil
}

func HasVideoCapability(raw string, target string) (bool, error) {
	capabilities, err := EffectiveVideoCapabilities(raw)
	if err != nil {
		return false, err
	}
	normalizedTarget := normalizeVideoCapability(target)
	for _, capability := range capabilities {
		if capability == normalizedTarget {
			return true, nil
		}
	}
	return false, nil
}

func parseDurationOptions(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var durations []int
	if err := common.UnmarshalJsonStr(raw, &durations); err != nil {
		return nil, fmt.Errorf("failed to parse duration options: %w", err)
	}

	normalized := make([]int, 0, len(durations))
	seen := make(map[int]struct{}, len(durations))
	for _, duration := range durations {
		if duration <= 0 {
			return nil, fmt.Errorf("invalid duration option: %d", duration)
		}
		if _, ok := seen[duration]; ok {
			continue
		}
		seen[duration] = struct{}{}
		normalized = append(normalized, duration)
	}

	return normalized, nil
}

func NormalizeDurationOptions(raw string) (string, error) {
	durations, err := parseDurationOptions(raw)
	if err != nil {
		return "", err
	}
	if len(durations) == 0 {
		return "", nil
	}

	data, err := common.Marshal(durations)
	if err != nil {
		return "", fmt.Errorf("failed to marshal duration options: %w", err)
	}
	return string(data), nil
}

func EffectiveDurationOptions(raw string) ([]int, error) {
	return parseDurationOptions(raw)
}

func (mm *ModelMapping) Insert() error {
	now := common.GetTimestamp()
	mm.CreatedTime = now
	mm.UpdatedTime = now
	record := map[string]any{
		"request_model":      mm.RequestModel,
		"actual_model":       mm.ActualModel,
		"display_name":       mm.DisplayName,
		"model_series":       mm.ModelSeries,
		"model_type":         mm.ModelType,
		"description":        mm.Description,
		"status":             mm.Status,
		"priority":           mm.Priority,
		"request_endpoint":   mm.RequestEndpoint,
		"resolutions":        mm.Resolutions,
		"aspect_ratios":      mm.AspectRatios,
		"image_capabilities": mm.ImageCapabilities,
		"video_capabilities": mm.VideoCapabilities,
		"duration_options":   mm.DurationOptions,
		"created_time":       mm.CreatedTime,
		"updated_time":       mm.UpdatedTime,
	}
	if err := DB.Model(&ModelMapping{}).Create(record).Error; err != nil {
		return err
	}
	return DB.Where("request_model = ?", mm.RequestModel).First(mm).Error
}

func (mm *ModelMapping) Update() error {
	mm.UpdatedTime = common.GetTimestamp()
	updates := map[string]any{
		"request_model":      mm.RequestModel,
		"actual_model":       mm.ActualModel,
		"display_name":       mm.DisplayName,
		"model_series":       mm.ModelSeries,
		"model_type":         mm.ModelType,
		"description":        mm.Description,
		"status":             mm.Status,
		"priority":           mm.Priority,
		"request_endpoint":   mm.RequestEndpoint,
		"resolutions":        mm.Resolutions,
		"aspect_ratios":      mm.AspectRatios,
		"image_capabilities": mm.ImageCapabilities,
		"video_capabilities": mm.VideoCapabilities,
		"duration_options":   mm.DurationOptions,
		"updated_time":       mm.UpdatedTime,
	}
	return DB.Model(&ModelMapping{}).Where("id = ?", mm.Id).Updates(updates).Error
}

func GetModelMapping(id int) (*ModelMapping, error) {
	var mm ModelMapping
	err := DB.First(&mm, id).Error
	return &mm, err
}

func GetAllModelMappings(startIdx int, num int) ([]*ModelMapping, error) {
	var mappings []*ModelMapping
	err := DB.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, err
}

func SearchModelMappings(keyword string, modelType int, startIdx int, num int) ([]*ModelMapping, int64, error) {
	var mappings []*ModelMapping
	var total int64

	query := DB.Model(&ModelMapping{})

	if keyword != "" {
		query = query.Where("request_model LIKE ? OR display_name LIKE ? OR model_series LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if modelType > 0 {
		query = query.Where("model_type = ?", modelType)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, total, err
}

func GetActiveImageModelMappings(startIdx int, num int) ([]*ModelMapping, int64, error) {
	var mappings []*ModelMapping

	query := DB.Model(&ModelMapping{}).
		Where("model_type = ? AND status = ? AND request_endpoint IN ?", 2, 1, []string{"openai", "openai-response", "gemini"})

	err := query.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, int64(len(mappings)), err
}

func GetActiveVideoModelMappings(startIdx int, num int) ([]*ModelMapping, int64, error) {
	var mappings []*ModelMapping

	query := DB.Model(&ModelMapping{}).
		Where("model_type = ? AND status = ? AND request_endpoint <> ''", 3, 1)

	err := query.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, int64(len(mappings)), err
}

func DeleteModelMapping(id int) error {
	return DB.Delete(&ModelMapping{}, id).Error
}

func GetModelMappingByRequestModel(requestModel string) (*ModelMapping, error) {
	var mm ModelMapping
	err := DB.Where("request_model = ?", requestModel).First(&mm).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &mm, err
}

func GetActiveModelMappingByRequestModel(requestModel string) (*ModelMapping, error) {
	var mm ModelMapping
	err := DB.Where("request_model = ? AND status = 1", requestModel).
		Where("(model_type <> 2 OR request_endpoint IN ?)", []string{"openai", "openai-response", "gemini"}).
		First(&mm).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &mm, err
}
