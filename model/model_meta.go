package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

type BoundChannel struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type CatalogDisplayMetadata struct {
	Description string `json:"description,omitempty"`
	VendorIcon  string `json:"vendor_icon,omitempty"`
}

type Model struct {
	Id           int            `json:"id"`
	ModelName    string         `json:"model_name" gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description  string         `json:"description,omitempty" gorm:"type:text"`
	Icon         string         `json:"icon,omitempty" gorm:"type:varchar(128)"`
	Tags         string         `json:"tags,omitempty" gorm:"type:varchar(255)"`
	VendorID     int            `json:"vendor_id,omitempty" gorm:"index"`
	Endpoints    string         `json:"endpoints,omitempty" gorm:"type:text"`
	Status       int            `json:"status" gorm:"default:1"`
	SyncOfficial int            `json:"sync_official" gorm:"default:1"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`

	BoundChannels []BoundChannel `json:"bound_channels,omitempty" gorm:"-"`
	EnableGroups  []string       `json:"enable_groups,omitempty" gorm:"-"`
	QuotaTypes    []int          `json:"quota_types,omitempty" gorm:"-"`
	NameRule      int            `json:"name_rule" gorm:"default:0"`

	MatchedModels []string `json:"matched_models,omitempty" gorm:"-"`
	MatchedCount  int      `json:"matched_count,omitempty" gorm:"-"`
}

func (mi *Model) Insert() error {
	now := common.GetTimestamp()
	mi.CreatedTime = now
	mi.UpdatedTime = now

	// 保存原始值（因为 Create 后可能被 GORM 的 default 标签覆盖为 1）
	originalStatus := mi.Status
	originalSyncOfficial := mi.SyncOfficial

	// 先创建记录（GORM 会对零值字段应用默认值）
	if err := DB.Create(mi).Error; err != nil {
		return err
	}

	// 使用保存的原始值进行更新，确保零值能正确保存
	return DB.Model(&Model{}).Where("id = ?", mi.Id).Updates(map[string]interface{}{
		"status":        originalStatus,
		"sync_official": originalSyncOfficial,
	}).Error
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

func (mi *Model) Update() error {
	mi.UpdatedTime = common.GetTimestamp()
	// 使用 Select 强制更新所有字段，包括零值
	return DB.Model(&Model{}).Where("id = ?", mi.Id).
		Select("model_name", "description", "icon", "tags", "vendor_id", "endpoints", "status", "sync_official", "name_rule", "updated_time").
		Updates(mi).Error
}

func (mi *Model) Delete() error {
	return DB.Delete(mi).Error
}

func GetVendorModelCounts() (map[int64]int64, error) {
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := DB.Model(&Model{}).
		Select("vendor_id as vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(stats))
	for _, s := range stats {
		m[s.VendorID] = s.Count
	}
	return m, nil
}

func GetAllModels(offset int, limit int) ([]*Model, error) {
	var models []*Model
	err := DB.Order("id DESC").Offset(offset).Limit(limit).Find(&models).Error
	return models, err
}

func modelNameRuleMatches(rule int, configuredModelName string, targetModelName string) bool {
	configuredModelName = strings.TrimSpace(configuredModelName)
	targetModelName = strings.TrimSpace(targetModelName)
	if configuredModelName == "" || targetModelName == "" {
		return false
	}
	switch rule {
	case NameRuleExact:
		return targetModelName == configuredModelName
	case NameRulePrefix:
		return strings.HasPrefix(targetModelName, configuredModelName)
	case NameRuleContains:
		return strings.Contains(targetModelName, configuredModelName)
	case NameRuleSuffix:
		return strings.HasSuffix(targetModelName, configuredModelName)
	default:
		return false
	}
}

func buildModelMetaMatchMap(models []Model, modelNames []string) map[string]*Model {
	normalizedNames := make([]string, 0, len(modelNames))
	seenNames := make(map[string]struct{}, len(modelNames))
	for _, modelName := range modelNames {
		trimmed := strings.TrimSpace(modelName)
		if trimmed == "" {
			continue
		}
		if _, ok := seenNames[trimmed]; ok {
			continue
		}
		seenNames[trimmed] = struct{}{}
		normalizedNames = append(normalizedNames, trimmed)
	}

	matched := make(map[string]*Model, len(normalizedNames))
	ruleOrder := []int{
		NameRuleExact,
		NameRulePrefix,
		NameRuleSuffix,
		NameRuleContains,
	}
	for _, rule := range ruleOrder {
		for i := range models {
			meta := &models[i]
			if meta.NameRule != rule {
				continue
			}
			for _, modelName := range normalizedNames {
				if _, ok := matched[modelName]; ok {
					continue
				}
				if modelNameRuleMatches(meta.NameRule, meta.ModelName, modelName) {
					matched[modelName] = meta
				}
			}
		}
	}
	return matched
}

func GetCatalogDisplayMetadataByModelNames(modelNames []string) (map[string]CatalogDisplayMetadata, error) {
	metadataByModel := make(map[string]CatalogDisplayMetadata)

	normalizedNames := make([]string, 0, len(modelNames))
	seenNames := make(map[string]struct{}, len(modelNames))
	for _, modelName := range modelNames {
		trimmed := strings.TrimSpace(modelName)
		if trimmed == "" {
			continue
		}
		if _, ok := seenNames[trimmed]; ok {
			continue
		}
		seenNames[trimmed] = struct{}{}
		normalizedNames = append(normalizedNames, trimmed)
	}
	if len(normalizedNames) == 0 {
		return metadataByModel, nil
	}

	var models []Model
	if err := DB.
		Where("status = ?", 1).
		Order("id ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	matchedModelMeta := buildModelMetaMatchMap(models, normalizedNames)
	vendorIDs := make([]int, 0, len(matchedModelMeta))
	seenVendorIDs := make(map[int]struct{}, len(matchedModelMeta))
	for _, meta := range matchedModelMeta {
		if meta == nil || meta.VendorID <= 0 {
			continue
		}
		if _, ok := seenVendorIDs[meta.VendorID]; ok {
			continue
		}
		seenVendorIDs[meta.VendorID] = struct{}{}
		vendorIDs = append(vendorIDs, meta.VendorID)
	}

	vendorsByID := make(map[int]*Vendor, len(vendorIDs))
	if len(vendorIDs) > 0 {
		var vendors []Vendor
		if err := DB.
			Where("status = ?", 1).
			Where("id IN ?", vendorIDs).
			Find(&vendors).Error; err != nil {
			return nil, err
		}
		for i := range vendors {
			vendor := &vendors[i]
			vendorsByID[vendor.Id] = vendor
		}
	}

	for modelName, meta := range matchedModelMeta {
		if meta == nil {
			continue
		}
		item := CatalogDisplayMetadata{
			Description: strings.TrimSpace(meta.Description),
		}
		if vendor, ok := vendorsByID[meta.VendorID]; ok {
			item.VendorIcon = strings.TrimSpace(vendor.Icon)
		}
		metadataByModel[modelName] = item
	}
	return metadataByModel, nil
}

func GetBoundChannelsByModelsMap(modelNames []string) (map[string][]BoundChannel, error) {
	result := make(map[string][]BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Name  string
		Type  int
	}
	var rows []row
	err := DB.Table("channels").
		Select("abilities.model as model, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = append(result[r.Model], BoundChannel{Name: r.Name, Type: r.Type})
	}
	return result, nil
}

func SearchModels(keyword string, vendor string, offset int, limit int) ([]*Model, int64, error) {
	var models []*Model
	db := DB.Model(&Model{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("model_name LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like)
	}
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			db = db.Where("models.vendor_id = ?", vid)
		} else {
			db = db.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}
