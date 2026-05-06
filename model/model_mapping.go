package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ImageCapabilityGeneration = "image_generation"
	ImageCapabilityEditing    = "image_editing"
)

var defaultImageCapabilities = []string{
	ImageCapabilityGeneration,
	ImageCapabilityEditing,
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
	RequestEndpoint   string `json:"request_endpoint" gorm:"size:32;default:''"` // openai, gemini, openai_mod
	Resolutions       string `json:"resolutions" gorm:"type:text"`               // JSON array: ["1K","2K","4K"]
	AspectRatios      string `json:"aspect_ratios" gorm:"type:text"`             // JSON array: ["1:1","16:9",...]
	ImageCapabilities string `json:"image_capabilities" gorm:"type:text"`        // JSON array: ["image_generation","image_editing"]
	CreatedTime       int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime       int64  `json:"updated_time" gorm:"bigint"`
}

func DefaultImageCapabilities() []string {
	return append([]string(nil), defaultImageCapabilities...)
}

func normalizeImageCapability(capability string) string {
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

func (mm *ModelMapping) Insert() error {
	now := common.GetTimestamp()
	mm.CreatedTime = now
	mm.UpdatedTime = now
	return DB.Create(mm).Error
}

func (mm *ModelMapping) Update() error {
	mm.UpdatedTime = common.GetTimestamp()
	// Use Select to explicitly update all fields including zero values
	return DB.Model(mm).Select("*").Updates(mm).Error
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

	// 只返回启用状态的模型
	query = query.Where("status = ?", 1)

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

func DeleteModelMapping(id int) error {
	return DB.Delete(&ModelMapping{}, id).Error
}

func GetModelMappingByRequestModel(requestModel string) (*ModelMapping, error) {
	var mm ModelMapping
	err := DB.Where("request_model = ? AND status = 1", requestModel).First(&mm).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &mm, err
}
