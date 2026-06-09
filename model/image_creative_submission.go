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
	"github.com/QuantumNous/new-api/setting/worker_setting"
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
	SelectedGroup   string  `json:"selected_group"`
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
	CachedAt   int64                    `json:"cached_at"`
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

	inspirationAssetRefreshMu       sync.Mutex
	inspirationAssetRefreshInFlight = make(map[string]struct{})
)

var (
	imageCreativeAssetDimensionPattern   = regexp.MustCompile(`^(\d{2,5})\s*[xX×*]\s*(\d{2,5})$`)
	imageCreativeAssetAspectRatioPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?::|x|X|/)\s*(\d+(?:\.\d+)?)$`)
)

func inspirationAssetListCacheTTL() time.Duration {
	ttlSeconds := worker_setting.GetWorkerSetting().InspirationPageCacheTTL
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}
	return time.Duration(ttlSeconds) * time.Second
}

func inspirationAssetFirstPageCacheTTL() time.Duration {
	ttlSeconds := worker_setting.GetWorkerSetting().InspirationFirstPageCacheTTL
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}
	return time.Duration(ttlSeconds) * time.Second
}

func inspirationAssetFirstPageStorageTTL() time.Duration {
	freshTTL := inspirationAssetFirstPageCacheTTL()
	if freshTTL <= 0 {
		return 0
	}
	return freshTTL * 2
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

func inspirationAssetListCacheKey(cursor string, num int, includeTotal bool) string {
	if num <= 0 {
		return ""
	}
	if includeTotal {
		return fmt.Sprintf("%s:%d:with_total", strings.TrimSpace(cursor), num)
	}
	return fmt.Sprintf("%s:%d:no_total", strings.TrimSpace(cursor), num)
}

func inspirationAssetListCacheTTLForCursor(cursor string) time.Duration {
	if strings.TrimSpace(cursor) == "" {
		return inspirationAssetFirstPageCacheTTL()
	}
	return inspirationAssetListCacheTTL()
}

func inspirationAssetListStorageTTLForCursor(cursor string) time.Duration {
	if strings.TrimSpace(cursor) == "" {
		return inspirationAssetFirstPageStorageTTL()
	}
	return inspirationAssetListCacheTTL()
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

func cloneInspirationAssetPage(page InspirationAssetPage) InspirationAssetPage {
	return InspirationAssetPage{
		Items:      cloneImageCreativeListItems(page.Items),
		Total:      page.Total,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
		CachedAt:   page.CachedAt,
	}
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
		Select("s.id, s.reviewed_time, s.submitted_time, t.model_id, t.selected_group, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url, t.thumbnail_url, t.image_metadata").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Where("s.status = ? AND t.status = ? AND t.image_url <> ?", CreativeSubmissionStatusApproved, ImageTaskStatusSuccess, "")
}

func encodeInspirationAssetCursor(reviewedTime int64, submittedTime int64, id int) string {
	if reviewedTime < 0 || submittedTime < 0 || id <= 0 {
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

type inspirationSubmissionFeedRow struct {
	Id            int   `gorm:"column:id"`
	TaskId        int   `gorm:"column:task_id"`
	ReviewedTime  int64 `gorm:"column:reviewed_time"`
	SubmittedTime int64 `gorm:"column:submitted_time"`
}

func countApprovedInspirationAssetsFromStore() (int64, error) {
	var taskIDs []int
	if err := DB.Model(&ImageCreativeSubmission{}).
		Where("status = ?", CreativeSubmissionStatusApproved).
		Pluck("task_id", &taskIDs).Error; err != nil {
		return 0, err
	}
	if len(taskIDs) == 0 {
		return 0, nil
	}
	store, err := imageTaskStoreForCanvas()
	if err != nil {
		return 0, err
	}
	var total int64
	err = forEachChunk(taskIDs, func(chunk []int) error {
		var count int64
		if err := store.model().
			Where("id IN ? AND status = ? AND image_url <> ?", chunk, ImageTaskStatusSuccess, "").
			Count(&count).Error; err != nil {
			return err
		}
		total += count
		return nil
	})
	return total, err
}

func approvedInspirationTasksByIDs(taskIDs []int) (map[int]*ImageGenerationTask, error) {
	result := make(map[int]*ImageGenerationTask)
	tasks, err := listImageTasksByIDs(taskIDs)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task == nil || task.Status != ImageTaskStatusSuccess || strings.TrimSpace(task.ImageUrl) == "" {
			continue
		}
		result[task.Id] = task
	}
	return result, nil
}

func imageCreativeUsersByIDs(userIDs []int) (map[int]*User, error) {
	result := make(map[int]*User)
	if len(userIDs) == 0 {
		return result, nil
	}
	err := forEachChunk(userIDs, func(chunk []int) error {
		var users []*User
		if err := DB.Select("id", "username", "display_name").Where("id IN ?", chunk).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			if user == nil {
				continue
			}
			result[user.Id] = user
		}
		return nil
	})
	return result, err
}

func getApprovedInspirationAssetsFromStore(cursor string, num int, includeTotal bool) (InspirationAssetPage, error) {
	cursor = strings.TrimSpace(cursor)
	var total int64
	limit := num
	if limit <= 0 {
		limit = 24
	}
	if includeTotal && cursor == "" {
		if count, err := countApprovedInspirationAssetsFromStore(); err != nil {
			return InspirationAssetPage{}, err
		} else {
			total = count
		}
	}

	assets := make([]*ImageCreativeAsset, 0, limit+1)
	scanCursor := cursor
	for len(assets) <= limit {
		rows, err := listApprovedInspirationSubmissionRows(scanCursor, limit+1)
		if err != nil {
			return InspirationAssetPage{}, err
		}
		if len(rows) == 0 {
			break
		}

		taskIDs := make([]int, 0, len(rows))
		for _, row := range rows {
			if row == nil || row.TaskId <= 0 {
				continue
			}
			taskIDs = append(taskIDs, row.TaskId)
		}
		tasksByID, err := approvedInspirationTasksByIDs(taskIDs)
		if err != nil {
			return InspirationAssetPage{}, err
		}
		for _, row := range rows {
			if row == nil {
				continue
			}
			task := tasksByID[row.TaskId]
			if task == nil {
				continue
			}
			asset := &ImageCreativeAsset{
				Id:            row.Id,
				ThumbnailUrl:  strings.TrimSpace(task.ThumbnailUrl),
				ImageMetadata: task.ImageMetadata,
				ReviewedTime:  row.ReviewedTime,
				SubmittedTime: row.SubmittedTime,
			}
			if asset.ThumbnailUrl == "" {
				asset.ThumbnailUrl = task.ImageUrl
			}
			populateImageCreativeAssetCardAspectRatio(asset)
			assets = append(assets, asset)
			if len(assets) > limit {
				break
			}
		}
		if len(assets) > limit || len(rows) < limit+1 {
			break
		}
		lastRow := rows[len(rows)-1]
		nextScanCursor := encodeInspirationAssetCursor(lastRow.ReviewedTime, lastRow.SubmittedTime, lastRow.Id)
		if nextScanCursor == "" || nextScanCursor == scanCursor {
			break
		}
		scanCursor = nextScanCursor
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

	return InspirationAssetPage{
		Items:      buildImageCreativeListItems(assets),
		Total:      total,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func listApprovedInspirationSubmissionRows(cursor string, limit int) ([]*inspirationSubmissionFeedRow, error) {
	if limit <= 0 {
		return []*inspirationSubmissionFeedRow{}, nil
	}
	subQuery, err := applyInspirationCursor(
		DB.Table("image_creative_submissions AS s").
			Select("s.id, s.task_id, s.reviewed_time, s.submitted_time").
			Where("s.status = ?", CreativeSubmissionStatusApproved),
		cursor,
	)
	if err != nil {
		return nil, err
	}
	subQuery = subQuery.
		Order("s.reviewed_time DESC, s.submitted_time DESC, s.id DESC").
		Limit(limit)

	var rows []*inspirationSubmissionFeedRow
	if err := DB.Table("(?) AS feed", subQuery).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func refreshApprovedInspirationAssetsCache(cacheKey, cursor string, num int, includeTotal bool, cacheTTL time.Duration) error {
	if strings.TrimSpace(cacheKey) == "" || cacheTTL <= 0 {
		return nil
	}
	page, err := getApprovedInspirationAssetsFromStore(cursor, num, includeTotal)
	if err != nil {
		return err
	}
	page.CachedAt = common.GetTimestamp()
	return getInspirationAssetListCache().SetWithTTL(cacheKey, cloneInspirationAssetPage(page), cacheTTL)
}

func triggerApprovedInspirationAssetsRefresh(cacheKey, cursor string, num int, includeTotal bool, cacheTTL time.Duration) {
	if strings.TrimSpace(cacheKey) == "" || cacheTTL <= 0 {
		return
	}

	inspirationAssetRefreshMu.Lock()
	if _, exists := inspirationAssetRefreshInFlight[cacheKey]; exists {
		inspirationAssetRefreshMu.Unlock()
		return
	}
	inspirationAssetRefreshInFlight[cacheKey] = struct{}{}
	inspirationAssetRefreshMu.Unlock()

	go func() {
		defer func() {
			inspirationAssetRefreshMu.Lock()
			delete(inspirationAssetRefreshInFlight, cacheKey)
			inspirationAssetRefreshMu.Unlock()
		}()

		if err := refreshApprovedInspirationAssetsCache(cacheKey, cursor, num, includeTotal, cacheTTL); err != nil {
			common.SysLog(fmt.Sprintf(
				"inspiration assets async refresh failed: cursor=%q page_size=%d include_total=%t err=%v",
				strings.TrimSpace(cursor),
				num,
				includeTotal,
				err,
			))
		}
	}()
}

func GetApprovedInspirationAssets(cursor string, num int, includeTotal bool) ([]*ImageCreativeListItem, int64, string, bool, error) {
	queryStart := time.Now()
	cursor = strings.TrimSpace(cursor)
	cacheTTL := inspirationAssetListStorageTTLForCursor(cursor)
	cacheKey := inspirationAssetListCacheKey(cursor, num, includeTotal)
	cacheEnabled := cacheKey != "" && cacheTTL > 0
	isFirstPage := cursor == ""
	freshTTL := inspirationAssetListCacheTTLForCursor(cursor)

	if cacheEnabled {
		cached, ok, err := getInspirationAssetListCache().Get(cacheKey)
		if err == nil && ok {
			isStaleFirstPage := false
			if isFirstPage && freshTTL > 0 {
				cachedAt := time.Unix(cached.CachedAt, 0)
				isStaleFirstPage = cached.CachedAt <= 0 || time.Since(cachedAt) > freshTTL
			}
			if isStaleFirstPage {
				triggerApprovedInspirationAssetsRefresh(cacheKey, cursor, num, includeTotal, cacheTTL)
				common.SysLog(fmt.Sprintf(
					"inspiration assets query: cache=stale cursor=%q page_size=%d items=%d has_more=%t elapsed_ms=%d",
					strings.TrimSpace(cursor),
					num,
					len(cached.Items),
					cached.HasMore,
					time.Since(queryStart).Milliseconds(),
				))
				return cloneImageCreativeListItems(cached.Items), cached.Total, cached.NextCursor, cached.HasMore, nil
			}

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

	page, err := getApprovedInspirationAssetsFromStore(cursor, num, includeTotal)
	if err != nil {
		return nil, 0, "", false, err
	}

	if cacheEnabled {
		page.CachedAt = common.GetTimestamp()
		_ = getInspirationAssetListCache().SetWithTTL(cacheKey, cloneInspirationAssetPage(page), cacheTTL)
	}

	common.SysLog(fmt.Sprintf(
		"inspiration assets query: cache=miss cursor=%q page_size=%d items=%d has_more=%t elapsed_ms=%d",
		strings.TrimSpace(cursor),
		num,
		len(page.Items),
		page.HasMore,
		time.Since(queryStart).Milliseconds(),
	))

	return cloneImageCreativeListItems(page.Items), page.Total, page.NextCursor, page.HasMore, nil
}

func GetApprovedCreativeAssets(cursor string, num int, includeTotal bool) ([]*ImageCreativeListItem, int64, string, bool, error) {
	return GetApprovedInspirationAssets(cursor, num, includeTotal)
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

	var submission ImageCreativeSubmission
	err := DB.Where("id = ? AND status = ?", id, CreativeSubmissionStatusApproved).First(&submission).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task, err := GetImageTaskByID(submission.TaskId)
	if err != nil {
		return nil, err
	}
	if task == nil || task.Status != ImageTaskStatusSuccess || strings.TrimSpace(task.ImageUrl) == "" {
		return nil, nil
	}
	displayByModelID, err := imageTaskModelDisplayByModelIDs([]string{task.ModelId})
	if err != nil {
		return nil, err
	}
	display := displayByModelID[strings.TrimSpace(task.ModelId)]
	asset := ImageCreativeAsset{
		Id:            submission.Id,
		ModelId:       task.ModelId,
		SelectedGroup: task.SelectedGroup,
		DisplayName:   display.DisplayName,
		ModelSeries:   display.ModelSeries,
		Prompt:        task.Prompt,
		Params:        task.Params,
		ImageUrl:      task.ImageUrl,
		ThumbnailUrl:  task.ThumbnailUrl,
		ImageMetadata: task.ImageMetadata,
		ReviewedTime:  submission.ReviewedTime,
		SubmittedTime: submission.SubmittedTime,
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

	query := DB.Model(&ImageCreativeSubmission{}).Where("status = ?", status)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []*ImageCreativeSubmission
	if err := query.
		Order("submitted_time DESC, id DESC").
		Limit(num).
		Offset(startIdx).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	taskIDs := make([]int, 0, len(rows))
	userIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		taskIDs = append(taskIDs, row.TaskId)
		userIDs = append(userIDs, row.UserId)
	}
	taskList, err := listImageTasksByIDs(taskIDs)
	if err != nil {
		return nil, 0, err
	}
	tasksByID := make(map[int]*ImageGenerationTask, len(taskList))
	modelIDs := make([]string, 0, len(taskList))
	for _, task := range taskList {
		if task == nil {
			continue
		}
		tasksByID[task.Id] = task
		modelIDs = append(modelIDs, task.ModelId)
	}
	displayByModelID, err := imageTaskModelDisplayByModelIDs(modelIDs)
	if err != nil {
		return nil, 0, err
	}
	usersByID, err := imageCreativeUsersByIDs(userIDs)
	if err != nil {
		return nil, 0, err
	}
	submissions := make([]*ImageCreativeAdminSubmission, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		task := tasksByID[row.TaskId]
		user := usersByID[row.UserId]
		display := imageTaskModelDisplay{}
		if task != nil {
			display = displayByModelID[strings.TrimSpace(task.ModelId)]
		}
		item := &ImageCreativeAdminSubmission{
			Id:            row.Id,
			SubmissionId:  row.Id,
			TaskId:        row.TaskId,
			UserId:        row.UserId,
			Status:        row.Status,
			RejectReason:  row.RejectReason,
			ReviewerId:    row.ReviewerId,
			SubmittedTime: row.SubmittedTime,
			ReviewedTime:  row.ReviewedTime,
		}
		if user != nil {
			item.Username = user.Username
			item.UserName = user.Username
			item.UserDisplayName = user.DisplayName
		}
		if task != nil {
			item.ModelId = task.ModelId
			item.DisplayName = display.DisplayName
			item.ModelSeries = display.ModelSeries
			item.Prompt = task.Prompt
			item.Params = task.Params
			item.ImageUrl = task.ImageUrl
			item.ImageMetadata = task.ImageMetadata
			item.CreatedTime = task.CreatedTime
			item.CompletedTime = task.CompletedTime
		}
		submissions = append(submissions, item)
	}
	return submissions, total, nil
}

func GetImageCreativeSubmissions(startIdx int, num int, status string) ([]*ImageCreativeAdminSubmission, int64, error) {
	return GetImageInspirationSubmissions(startIdx, num, status)
}

func GetImageInspirationAdminSubmissionByID(id int) (*ImageCreativeAdminSubmission, error) {
	var row ImageCreativeSubmission
	err := DB.Where("id = ?", id).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task, err := GetImageTaskByID(row.TaskId)
	if err != nil {
		return nil, err
	}
	displayByModelID := map[string]imageTaskModelDisplay{}
	if task != nil {
		displayByModelID, err = imageTaskModelDisplayByModelIDs([]string{task.ModelId})
		if err != nil {
			return nil, err
		}
	}
	usersByID, err := imageCreativeUsersByIDs([]int{row.UserId})
	if err != nil {
		return nil, err
	}
	submission := &ImageCreativeAdminSubmission{
		Id:            row.Id,
		SubmissionId:  row.Id,
		TaskId:        row.TaskId,
		UserId:        row.UserId,
		Status:        row.Status,
		RejectReason:  row.RejectReason,
		ReviewerId:    row.ReviewerId,
		SubmittedTime: row.SubmittedTime,
		ReviewedTime:  row.ReviewedTime,
	}
	if user := usersByID[row.UserId]; user != nil {
		submission.Username = user.Username
		submission.UserName = user.Username
		submission.UserDisplayName = user.DisplayName
	}
	if task != nil {
		display := displayByModelID[strings.TrimSpace(task.ModelId)]
		submission.ModelId = task.ModelId
		submission.DisplayName = display.DisplayName
		submission.ModelSeries = display.ModelSeries
		submission.Prompt = task.Prompt
		submission.Params = task.Params
		submission.ImageUrl = task.ImageUrl
		submission.ImageMetadata = task.ImageMetadata
		submission.CreatedTime = task.CreatedTime
		submission.CompletedTime = task.CompletedTime
	}
	return submission, nil
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
