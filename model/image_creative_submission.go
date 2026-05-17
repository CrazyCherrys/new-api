package model

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageCreativeSubmission records user-submitted image assets awaiting or passing review.
type ImageCreativeSubmission struct {
	Id            int    `json:"id" gorm:"primaryKey;index:idx_image_creative_public_order,priority:4"`
	TaskId        int    `json:"task_id" gorm:"uniqueIndex;not null"`
	UserId        int    `json:"user_id" gorm:"index;not null"`
	Status        string `json:"status" gorm:"size:20;not null;index;index:idx_image_creative_public_order,priority:1;default:'pending'"`
	SubmittedTime int64  `json:"submitted_time" gorm:"bigint;index;index:idx_image_creative_public_order,priority:3"`
	ReviewedTime  int64  `json:"reviewed_time" gorm:"bigint;index:idx_image_creative_public_order,priority:2"`
	ReviewerId    int    `json:"reviewer_id" gorm:"index"`
	RejectReason  string `json:"reject_reason" gorm:"type:text"`
}

const (
	CreativeSubmissionStatusPending  = "pending"
	CreativeSubmissionStatusApproved = "approved"
	CreativeSubmissionStatusRejected = "rejected"
)

type ImageCreativeAsset struct {
	Id              int     `json:"id"`
	ModelId         string  `json:"model_id"`
	DisplayName     string  `json:"display_name"`
	ModelSeries     string  `json:"model_series"`
	Prompt          string  `json:"prompt"`
	Params          string  `json:"params"`
	ImageUrl        string  `json:"image_url"`
	ThumbnailUrl    string  `json:"thumbnail_url"`
	ImageMetadata   string  `json:"image_metadata"`
	CardAspectRatio float64 `json:"card_aspect_ratio"`
	ReviewedTime    int64   `json:"-"`
	SubmittedTime   int64   `json:"-"`
}

type ImageCreativeListItem struct {
	Id              int     `json:"id"`
	ThumbnailUrl    string  `json:"thumbnail_url"`
	CardAspectRatio float64 `json:"card_aspect_ratio"`
	ReviewedTime    int64   `json:"-"`
	SubmittedTime   int64   `json:"-"`
}

type ImageCreativeAdminSubmission struct {
	Id              int    `json:"id"`
	SubmissionId    int    `json:"submission_id"`
	TaskId          int    `json:"task_id"`
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	UserName        string `json:"user_name"`
	UserDisplayName string `json:"user_display_name"`
	Status          string `json:"status"`
	RejectReason    string `json:"reject_reason"`
	ReviewerId      int    `json:"reviewer_id"`
	SubmittedTime   int64  `json:"submitted_time"`
	ReviewedTime    int64  `json:"reviewed_time"`
	ModelId         string `json:"model_id"`
	DisplayName     string `json:"display_name"`
	ModelSeries     string `json:"model_series"`
	Prompt          string `json:"prompt"`
	Params          string `json:"params"`
	ImageUrl        string `json:"image_url"`
	ImageMetadata   string `json:"image_metadata"`
	CreatedTime     int64  `json:"created_time"`
	CompletedTime   int64  `json:"completed_time"`
}

type InspirationAssetPage struct {
	Items      []*ImageCreativeListItem `json:"items"`
	Total      int64                    `json:"total"`
	NextCursor string                   `json:"next_cursor"`
	HasMore    bool                     `json:"has_more"`
}

const (
	inspirationAssetListCacheNamespace   = "new-api:inspiration_assets:v1"
	inspirationAssetDetailCacheNamespace = "new-api:inspiration_asset:v1"
)

var (
	inspirationAssetListCacheOnce   sync.Once
	inspirationAssetDetailCacheOnce sync.Once

	inspirationAssetListCache   *cachex.HybridCache[InspirationAssetPage]
	inspirationAssetDetailCache *cachex.HybridCache[ImageCreativeAsset]
)

var (
	imageCreativeAssetDimensionPattern   = regexp.MustCompile(`^(\d{2,5})\s*[xX×*]\s*(\d{2,5})$`)
	imageCreativeAssetAspectRatioPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?::|x|X|/)\s*(\d+(?:\.\d+)?)$`)
)

func inspirationAssetListCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("INSPIRATION_ASSET_LIST_CACHE_TTL", 60)
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}
	return time.Duration(ttlSeconds) * time.Second
}

func inspirationAssetDetailCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("INSPIRATION_ASSET_DETAIL_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func inspirationAssetListCacheCapacity() int {
	capacity := common.GetEnvOrDefault("INSPIRATION_ASSET_LIST_CACHE_CAP", 200)
	if capacity <= 0 {
		capacity = 200
	}
	return capacity
}

func inspirationAssetDetailCacheCapacity() int {
	capacity := common.GetEnvOrDefault("INSPIRATION_ASSET_DETAIL_CACHE_CAP", 2000)
	if capacity <= 0 {
		capacity = 2000
	}
	return capacity
}

func getInspirationAssetListCache() *cachex.HybridCache[InspirationAssetPage] {
	inspirationAssetListCacheOnce.Do(func() {
		ttl := inspirationAssetListCacheTTL()
		inspirationAssetListCache = cachex.NewHybridCache[InspirationAssetPage](cachex.HybridCacheConfig[InspirationAssetPage]{
			Namespace: cachex.Namespace(inspirationAssetListCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[InspirationAssetPage]{},
			Memory: func() *hot.HotCache[string, InspirationAssetPage] {
				return hot.NewHotCache[string, InspirationAssetPage](hot.LRU, inspirationAssetListCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return inspirationAssetListCache
}

func getInspirationAssetDetailCache() *cachex.HybridCache[ImageCreativeAsset] {
	inspirationAssetDetailCacheOnce.Do(func() {
		ttl := inspirationAssetDetailCacheTTL()
		inspirationAssetDetailCache = cachex.NewHybridCache[ImageCreativeAsset](cachex.HybridCacheConfig[ImageCreativeAsset]{
			Namespace: cachex.Namespace(inspirationAssetDetailCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[ImageCreativeAsset]{},
			Memory: func() *hot.HotCache[string, ImageCreativeAsset] {
				return hot.NewHotCache[string, ImageCreativeAsset](hot.LRU, inspirationAssetDetailCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return inspirationAssetDetailCache
}

func inspirationAssetListCacheKey(cursor string, num int) string {
	if num <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(cursor), num)
}

func inspirationAssetDetailCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func cloneImageCreativeAsset(asset *ImageCreativeAsset) *ImageCreativeAsset {
	if asset == nil {
		return nil
	}
	next := *asset
	return &next
}

func cloneImageCreativeAssets(assets []*ImageCreativeAsset) []*ImageCreativeAsset {
	if len(assets) == 0 {
		return assets
	}
	next := make([]*ImageCreativeAsset, 0, len(assets))
	for _, asset := range assets {
		next = append(next, cloneImageCreativeAsset(asset))
	}
	return next
}

func cloneImageCreativeListItem(item *ImageCreativeListItem) *ImageCreativeListItem {
	if item == nil {
		return nil
	}
	next := *item
	return &next
}

func cloneImageCreativeListItems(items []*ImageCreativeListItem) []*ImageCreativeListItem {
	if len(items) == 0 {
		return items
	}
	next := make([]*ImageCreativeListItem, 0, len(items))
	for _, item := range items {
		next = append(next, cloneImageCreativeListItem(item))
	}
	return next
}

func parseImageCreativeAssetJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var data map[string]any
	if err := common.UnmarshalJsonStr(raw, &data); err != nil || len(data) == 0 {
		return nil
	}
	return data
}

func parseImageCreativeAssetNestedJSON(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return nil
		}
		return typed
	case string:
		return parseImageCreativeAssetJSON(typed)
	default:
		return nil
	}
}

func readImageCreativeAssetValue(sources []map[string]any, keys []string) any {
	for _, source := range sources {
		if len(source) == 0 {
			continue
		}
		for _, key := range keys {
			value, ok := source[key]
			if !ok || value == nil {
				continue
			}
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) == "" {
					continue
				}
			}
			return value
		}
	}
	return nil
}

func readImageCreativeAssetStringValue(sources []map[string]any, keys []string) string {
	value := readImageCreativeAssetValue(sources, keys)
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func readImageCreativeAssetPositiveFloat(sources []map[string]any, keys []string) float64 {
	value := readImageCreativeAssetValue(sources, keys)
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return typed
		}
	case float32:
		if typed > 0 {
			return float64(typed)
		}
	case int:
		if typed > 0 {
			return float64(typed)
		}
	case int64:
		if typed > 0 {
			return float64(typed)
		}
	case uint:
		if typed > 0 {
			return float64(typed)
		}
	case uint64:
		if typed > 0 {
			return float64(typed)
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	default:
		if value != nil {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
			if err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func parseImageCreativeAssetDimensionString(value string) (int, int) {
	matches := imageCreativeAssetDimensionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return 0, 0
	}

	width, err := strconv.Atoi(matches[1])
	if err != nil || width <= 0 {
		return 0, 0
	}
	height, err := strconv.Atoi(matches[2])
	if err != nil || height <= 0 {
		return 0, 0
	}
	return width, height
}

func parseImageCreativeAssetAspectRatio(value string) float64 {
	matches := imageCreativeAssetAspectRatioPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return 0
	}

	width, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || width <= 0 {
		return 0
	}
	height, err := strconv.ParseFloat(matches[2], 64)
	if err != nil || height <= 0 {
		return 0
	}
	return width / height
}

func resolveImageCreativeAssetCardAspectRatio(imageMetadata, params string) float64 {
	sources := make([]map[string]any, 0, 3)
	if metadata := parseImageCreativeAssetJSON(imageMetadata); len(metadata) > 0 {
		sources = append(sources, metadata)
		if nested := parseImageCreativeAssetNestedJSON(metadata["metadata"]); len(nested) > 0 {
			sources = append(sources, nested)
		}
	}
	if paramsMap := parseImageCreativeAssetJSON(params); len(paramsMap) > 0 {
		sources = append(sources, paramsMap)
	}

	width := readImageCreativeAssetPositiveFloat(sources, []string{
		"width",
		"output_width",
		"image_width",
		"outputWidth",
		"imageWidth",
	})
	height := readImageCreativeAssetPositiveFloat(sources, []string{
		"height",
		"output_height",
		"image_height",
		"outputHeight",
		"imageHeight",
	})
	if width > 0 && height > 0 {
		return width / height
	}

	dimensionText := readImageCreativeAssetStringValue(sources, []string{
		"size",
		"output_size",
		"dimensions",
		"outputSize",
	})
	if dimensionWidth, dimensionHeight := parseImageCreativeAssetDimensionString(dimensionText); dimensionWidth > 0 && dimensionHeight > 0 {
		return float64(dimensionWidth) / float64(dimensionHeight)
	}

	if ratio := parseImageCreativeAssetAspectRatio(readImageCreativeAssetStringValue(sources, []string{
		"aspect_ratio",
		"aspectRatio",
	})); ratio > 0 {
		return ratio
	}

	return 1
}

func populateImageCreativeAssetCardAspectRatio(asset *ImageCreativeAsset) {
	if asset == nil {
		return
	}
	asset.CardAspectRatio = resolveImageCreativeAssetCardAspectRatio(asset.ImageMetadata, asset.Params)
	if asset.CardAspectRatio <= 0 {
		asset.CardAspectRatio = 1
	}
}

func buildImageCreativeListItem(asset *ImageCreativeAsset) *ImageCreativeListItem {
	if asset == nil {
		return nil
	}
	return &ImageCreativeListItem{
		Id:              asset.Id,
		ThumbnailUrl:    asset.ThumbnailUrl,
		CardAspectRatio: asset.CardAspectRatio,
		ReviewedTime:    asset.ReviewedTime,
		SubmittedTime:   asset.SubmittedTime,
	}
}

func buildImageCreativeListItems(assets []*ImageCreativeAsset) []*ImageCreativeListItem {
	if len(assets) == 0 {
		return nil
	}
	items := make([]*ImageCreativeListItem, 0, len(assets))
	for _, asset := range assets {
		if item := buildImageCreativeListItem(asset); item != nil {
			items = append(items, item)
		}
	}
	return items
}

func InvalidateInspirationAssetCache() {
	_ = getInspirationAssetListCache().Purge()
	_ = getInspirationAssetDetailCache().Purge()
}

func isValidCreativeSubmissionStatus(status string) bool {
	switch status {
	case CreativeSubmissionStatusPending, CreativeSubmissionStatusApproved, CreativeSubmissionStatusRejected:
		return true
	default:
		return false
	}
}

func isValidCreativeReviewStatus(status string) bool {
	switch status {
	case CreativeSubmissionStatusApproved, CreativeSubmissionStatusRejected:
		return true
	default:
		return false
	}
}

func GetImageInspirationSubmissionByTaskID(taskId int) (*ImageCreativeSubmission, error) {
	var submission ImageCreativeSubmission
	err := DB.Where("task_id = ?", taskId).First(&submission).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &submission, err
}

func GetImageCreativeSubmissionByTaskID(taskId int) (*ImageCreativeSubmission, error) {
	return GetImageInspirationSubmissionByTaskID(taskId)
}

func SubmitImageAssetToInspiration(userId int, taskId int) (*ImageCreativeSubmission, error) {
	asset, err := GetImageAssetByID(userId, taskId)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, errors.New("资产不存在或不可提交")
	}

	submission := &ImageCreativeSubmission{
		TaskId:        taskId,
		UserId:        userId,
		Status:        CreativeSubmissionStatusPending,
		SubmittedTime: common.GetTimestamp(),
	}
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoNothing: true,
	}).Create(submission)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		existing, err := GetImageInspirationSubmissionByTaskID(taskId)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		return nil, errors.New("投稿已存在")
	}
	if submission.Id == 0 {
		existing, err := GetImageInspirationSubmissionByTaskID(taskId)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		return nil, errors.New("投稿创建失败")
	}
	return submission, nil
}

func SubmitImageAssetToCreativeSpace(userId int, taskId int) (*ImageCreativeSubmission, error) {
	return SubmitImageAssetToInspiration(userId, taskId)
}

func publicInspirationAssetsBaseQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		// Keep the feed payload small; full params belong to the detail endpoint.
		Select("s.id, s.reviewed_time, s.submitted_time, COALESCE(NULLIF(t.thumbnail_url, ''), t.image_url) AS thumbnail_url, t.image_metadata").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Where("s.status = ? AND t.status = ? AND t.image_url <> ?", CreativeSubmissionStatusApproved, ImageTaskStatusSuccess, "")
}

func publicInspirationAssetDetailQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		Select("s.id, s.reviewed_time, s.submitted_time, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url, t.thumbnail_url, t.image_metadata").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Where("s.status = ? AND t.status = ? AND t.image_url <> ?", CreativeSubmissionStatusApproved, ImageTaskStatusSuccess, "")
}

func encodeInspirationAssetCursor(reviewedTime int64, submittedTime int64, id int) string {
	if reviewedTime <= 0 || submittedTime < 0 || id <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d", reviewedTime, submittedTime, id)
}

func decodeInspirationAssetCursor(cursor string) (int64, int64, int, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, 0, 0, nil
	}
	parts := strings.Split(cursor, ":")
	if len(parts) != 3 {
		return 0, 0, 0, errors.New("invalid cursor")
	}
	reviewedTime, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, errors.New("invalid cursor")
	}
	submittedTime, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, errors.New("invalid cursor")
	}
	id64, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || id64 <= 0 {
		return 0, 0, 0, errors.New("invalid cursor")
	}
	return reviewedTime, submittedTime, int(id64), nil
}

func applyInspirationCursor(query *gorm.DB, cursor string) (*gorm.DB, error) {
	reviewedTime, submittedTime, id, err := decodeInspirationAssetCursor(cursor)
	if err != nil {
		return nil, err
	}
	if reviewedTime == 0 && submittedTime == 0 && id == 0 {
		return query, nil
	}
	return query.Where(
		"(s.reviewed_time < ?) OR (s.reviewed_time = ? AND s.submitted_time < ?) OR (s.reviewed_time = ? AND s.submitted_time = ? AND s.id < ?)",
		reviewedTime,
		reviewedTime, submittedTime,
		reviewedTime, submittedTime, id,
	), nil
}

func GetApprovedInspirationAssets(cursor string, num int) ([]*ImageCreativeListItem, int64, string, bool, error) {
	queryStart := time.Now()
	cacheKey := inspirationAssetListCacheKey(cursor, num)
	if cacheKey != "" {
		cached, ok, err := getInspirationAssetListCache().Get(cacheKey)
		if err == nil && ok {
			common.SysLog(fmt.Sprintf(
				"inspiration assets query: cache=hit cursor=%q page_size=%d items=%d has_more=%t elapsed_ms=%d",
				strings.TrimSpace(cursor),
				num,
				len(cached.Items),
				cached.HasMore,
				time.Since(queryStart).Milliseconds(),
			))
			return cloneImageCreativeListItems(cached.Items), cached.Total, cached.NextCursor, cached.HasMore, nil
		}
	}

	var assets []*ImageCreativeAsset
	var total int64
	limit := num
	if limit <= 0 {
		limit = 24
	}

	subQuery, err := applyInspirationCursor(
		DB.Table("image_creative_submissions AS s").
			Select("s.id, s.task_id, s.reviewed_time, s.submitted_time").
			Where("s.status = ?", CreativeSubmissionStatusApproved),
		cursor,
	)
	if err != nil {
		return nil, 0, "", false, err
	}

	subQuery = subQuery.
		Order("s.reviewed_time DESC, s.submitted_time DESC, s.id DESC").
		Limit(limit + 1)

	if err := DB.Table("(?) AS feed", subQuery).
		Select("feed.id, feed.reviewed_time, feed.submitted_time, COALESCE(NULLIF(t.thumbnail_url, ''), t.image_url) AS thumbnail_url, t.image_metadata").
		Joins("JOIN image_generation_tasks AS t ON t.id = feed.task_id").
		Where("t.status = ? AND t.image_url <> ?", ImageTaskStatusSuccess, "").
		Scan(&assets).Error; err != nil {
		return nil, 0, "", false, err
	}

	for _, asset := range assets {
		populateImageCreativeAssetCardAspectRatio(asset)
	}

	hasMore := len(assets) > limit
	if hasMore {
		assets = assets[:limit]
	}
	nextCursor := ""
	if hasMore && len(assets) > 0 {
		lastAsset := assets[len(assets)-1]
		nextCursor = encodeInspirationAssetCursor(lastAsset.ReviewedTime, lastAsset.SubmittedTime, lastAsset.Id)
	}

	if cacheKey != "" {
		items := buildImageCreativeListItems(assets)
		_ = getInspirationAssetListCache().SetWithTTL(cacheKey, InspirationAssetPage{
			Items:      cloneImageCreativeListItems(items),
			Total:      total,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		}, inspirationAssetListCacheTTL())
	}

	common.SysLog(fmt.Sprintf(
		"inspiration assets query: cache=miss cursor=%q page_size=%d items=%d has_more=%t elapsed_ms=%d",
		strings.TrimSpace(cursor),
		num,
		len(assets),
		hasMore,
		time.Since(queryStart).Milliseconds(),
	))

	return buildImageCreativeListItems(assets), total, nextCursor, hasMore, nil
}

func GetApprovedCreativeAssets(cursor string, num int) ([]*ImageCreativeListItem, int64, string, bool, error) {
	return GetApprovedInspirationAssets(cursor, num)
}

func GetApprovedInspirationAssetByID(id int) (*ImageCreativeAsset, error) {
	cacheKey := inspirationAssetDetailCacheKey(id)
	if cacheKey != "" {
		cached, ok, err := getInspirationAssetDetailCache().Get(cacheKey)
		if err == nil && ok {
			asset := cloneImageCreativeAsset(&cached)
			populateImageCreativeAssetCardAspectRatio(asset)
			return asset, nil
		}
	}

	var asset ImageCreativeAsset
	err := publicInspirationAssetDetailQuery().Where("s.id = ?", id).Scan(&asset).Error
	if err != nil {
		return nil, err
	}
	if asset.Id == 0 {
		return nil, nil
	}
	populateImageCreativeAssetCardAspectRatio(&asset)
	if cacheKey != "" {
		_ = getInspirationAssetDetailCache().SetWithTTL(cacheKey, asset, inspirationAssetDetailCacheTTL())
	}
	return cloneImageCreativeAsset(&asset), nil
}

func GetApprovedCreativeAssetByID(id int) (*ImageCreativeAsset, error) {
	return GetApprovedInspirationAssetByID(id)
}

func adminInspirationSubmissionsBaseQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		Select("s.id, s.id AS submission_id, s.task_id, s.user_id, COALESCE(u.username, '') AS username, COALESCE(u.username, '') AS user_name, COALESCE(u.display_name, '') AS user_display_name, s.status, s.reject_reason, s.reviewer_id, s.submitted_time, s.reviewed_time, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url, t.image_metadata, t.created_time, t.completed_time").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Joins("LEFT JOIN users AS u ON u.id = s.user_id")
}

func GetImageInspirationSubmissions(startIdx int, num int, status string) ([]*ImageCreativeAdminSubmission, int64, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = CreativeSubmissionStatusPending
	}
	if !isValidCreativeSubmissionStatus(status) {
		return nil, 0, errors.New("无效的审核状态")
	}

	query := adminInspirationSubmissionsBaseQuery().Where("s.status = ?", status)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var submissions []*ImageCreativeAdminSubmission
	if err := query.
		Order("s.submitted_time DESC, s.id DESC").
		Limit(num).
		Offset(startIdx).
		Scan(&submissions).Error; err != nil {
		return nil, 0, err
	}

	return submissions, total, nil
}

func GetImageCreativeSubmissions(startIdx int, num int, status string) ([]*ImageCreativeAdminSubmission, int64, error) {
	return GetImageInspirationSubmissions(startIdx, num, status)
}

func GetImageInspirationAdminSubmissionByID(id int) (*ImageCreativeAdminSubmission, error) {
	var submission ImageCreativeAdminSubmission
	err := adminInspirationSubmissionsBaseQuery().Where("s.id = ?", id).Scan(&submission).Error
	if err != nil {
		return nil, err
	}
	if submission.Id == 0 {
		return nil, nil
	}
	return &submission, nil
}

func GetImageCreativeAdminSubmissionByID(id int) (*ImageCreativeAdminSubmission, error) {
	return GetImageInspirationAdminSubmissionByID(id)
}

func ReviewImageInspirationSubmission(id int, reviewerId int, status string, rejectReason string) (*ImageCreativeAdminSubmission, error) {
	status = strings.TrimSpace(status)
	if !isValidCreativeReviewStatus(status) {
		return nil, errors.New("审核状态必须为 approved 或 rejected")
	}

	rejectReason = strings.TrimSpace(rejectReason)
	if status == CreativeSubmissionStatusApproved {
		rejectReason = ""
	}

	updates := map[string]interface{}{
		"status":        status,
		"reviewed_time": common.GetTimestamp(),
		"reviewer_id":   reviewerId,
		"reject_reason": rejectReason,
	}
	result := DB.Model(&ImageCreativeSubmission{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("投稿不存在")
	}

	InvalidateInspirationAssetCache()
	return GetImageInspirationAdminSubmissionByID(id)
}

func ReviewImageCreativeSubmission(id int, reviewerId int, status string, rejectReason string) (*ImageCreativeAdminSubmission, error) {
	return ReviewImageInspirationSubmission(id, reviewerId, status, rejectReason)
}

func DeleteImageInspirationSubmission(id int) error {
	result := DB.Delete(&ImageCreativeSubmission{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("投稿不存在")
	}
	InvalidateInspirationAssetCache()
	return nil
}

func DeleteImageCreativeSubmission(id int) error {
	return DeleteImageInspirationSubmission(id)
}
