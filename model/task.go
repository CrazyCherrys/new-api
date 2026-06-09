package model

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	commonRelay "github.com/QuantumNous/new-api/relay/common"
	"gorm.io/gorm"
)

type TaskStatus string

func (t TaskStatus) ToVideoStatus() string {
	var status string
	switch t {
	case TaskStatusNotStart, TaskStatusQueued, TaskStatusSubmitted:
		status = dto.VideoStatusQueued
	case TaskStatusInProgress:
		status = dto.VideoStatusInProgress
	case TaskStatusSuccess:
		status = dto.VideoStatusCompleted
	case TaskStatusFailure:
		status = dto.VideoStatusFailed
	default:
		status = dto.VideoStatusUnknown // Default fallback
	}
	return status
}

const (
	TaskStatusNotStart   TaskStatus = "NOT_START"
	TaskStatusSubmitted             = "SUBMITTED"
	TaskStatusQueued                = "QUEUED"
	TaskStatusInProgress            = "IN_PROGRESS"
	TaskStatusFailure               = "FAILURE"
	TaskStatusSuccess               = "SUCCESS"
	TaskStatusUnknown               = "UNKNOWN"
)

type Task struct {
	ID                int64                 `json:"id" gorm:"primary_key;AUTO_INCREMENT;index:idx_tasks_user_action_submit_id,priority:4;index:idx_tasks_user_action_status_submit_id,priority:5;index:idx_tasks_user_action_status_finish_id,priority:5;index:idx_tasks_user_action_origin_model_id,priority:5;index:idx_tasks_user_action_upstream_model_id,priority:5"`
	CreatedAt         int64                 `json:"created_at" gorm:"index"`
	UpdatedAt         int64                 `json:"updated_at"`
	TaskID            string                `json:"task_id" gorm:"type:varchar(191);index"` // 第三方id，不一定有/ song id\ Task id
	Platform          constant.TaskPlatform `json:"platform" gorm:"type:varchar(30);index"` // 平台
	UserId            int                   `json:"user_id" gorm:"index;index:idx_tasks_user_action_submit_id,priority:1;index:idx_tasks_user_action_status_submit_id,priority:1;index:idx_tasks_user_action_status_finish_id,priority:1;index:idx_tasks_user_action_origin_model_id,priority:1;index:idx_tasks_user_action_upstream_model_id,priority:1"`
	Group             string                `json:"group" gorm:"type:varchar(50)"` // 修正计费用
	ChannelId         int                   `json:"channel_id" gorm:"index"`
	Quota             int                   `json:"quota"`
	Action            string                `json:"action" gorm:"type:varchar(40);index;index:idx_tasks_user_action_submit_id,priority:2;index:idx_tasks_user_action_status_submit_id,priority:2;index:idx_tasks_user_action_status_finish_id,priority:2;index:idx_tasks_user_action_origin_model_id,priority:2;index:idx_tasks_user_action_upstream_model_id,priority:2"` // 任务类型, song, lyrics, description-mode
	Status            TaskStatus            `json:"status" gorm:"type:varchar(20);index;index:idx_tasks_user_action_status_submit_id,priority:3;index:idx_tasks_user_action_status_finish_id,priority:3"`                                                                                                                                                                  // 任务状态
	FailReason        string                `json:"fail_reason"`
	SubmitTime        int64                 `json:"submit_time" gorm:"index;index:idx_tasks_user_action_submit_id,priority:3;index:idx_tasks_user_action_status_submit_id,priority:4;index:idx_tasks_user_action_origin_model_id,priority:4;index:idx_tasks_user_action_upstream_model_id,priority:4"`
	StartTime         int64                 `json:"start_time" gorm:"index"`
	FinishTime        int64                 `json:"finish_time" gorm:"index;index:idx_tasks_user_action_status_finish_id,priority:4"`
	Progress          string                `json:"progress" gorm:"type:varchar(20);index"`
	OriginModelName   string                `json:"origin_model_name,omitempty" gorm:"type:varchar(191);index:idx_tasks_user_action_origin_model_id,priority:3"`
	UpstreamModelName string                `json:"upstream_model_name,omitempty" gorm:"type:varchar(191);index:idx_tasks_user_action_upstream_model_id,priority:3"`
	Properties        Properties            `json:"properties" gorm:"type:json"`
	Username          string                `json:"username,omitempty" gorm:"-"`
	StorageScope      string                `json:"-" gorm:"-"`
	// 禁止返回给用户，内部可能包含key等隐私信息
	PrivateData TaskPrivateData `json:"-" gorm:"column:private_data;type:json"`
	Data        json.RawMessage `json:"data" gorm:"type:json"`
}

const (
	taskStorageScopeMain  = "main"
	taskStorageScopeVideo = "canvas_video"
)

func videoTaskDB() (*gorm.DB, error) {
	if DB == nil {
		return nil, fmt.Errorf("video task database is not initialized")
	}
	return DB, nil
}

func canvasVideoTaskDB() (*gorm.DB, error) {
	return canvasModeDataDB(CanvasModeVideo)
}

func taskDBForScope(scope string) (*gorm.DB, error) {
	switch strings.TrimSpace(scope) {
	case taskStorageScopeVideo:
		return canvasVideoTaskDB()
	default:
		if DB == nil {
			return nil, fmt.Errorf("task database is not initialized")
		}
		return DB, nil
	}
}

func taskScopeForTask(task *Task) string {
	if task == nil {
		return taskStorageScopeMain
	}
	if strings.TrimSpace(task.StorageScope) == taskStorageScopeVideo {
		return taskStorageScopeVideo
	}
	return taskStorageScopeMain
}

func taskDBForTask(task *Task) (*gorm.DB, error) {
	return taskDBForScope(taskScopeForTask(task))
}

func MarkTaskCanvasVideoScope(task *Task) {
	if task == nil {
		return
	}
	task.StorageScope = taskStorageScopeVideo
}

func (Task *Task) InsertCanvasVideo() error {
	MarkTaskCanvasVideoScope(Task)
	return Task.Insert()
}

func markTasksStorageScope(tasks []*Task, scope string) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		task.StorageScope = scope
	}
}

func (t *Task) SetData(data any) {
	b, _ := common.Marshal(data)
	t.Data = json.RawMessage(b)
}

func (t *Task) GetData(v any) error {
	return common.Unmarshal(t.Data, &v)
}

type Properties struct {
	Input             string `json:"input"`
	UpstreamModelName string `json:"upstream_model_name,omitempty"`
	OriginModelName   string `json:"origin_model_name,omitempty"`
	RequestParams     string `json:"request_params,omitempty"`
}

func (m *Properties) Scan(val interface{}) error {
	bytesValue, _ := val.([]byte)
	if len(bytesValue) == 0 {
		*m = Properties{}
		return nil
	}
	return common.Unmarshal(bytesValue, m)
}

func (m Properties) Value() (driver.Value, error) {
	if m == (Properties{}) {
		return nil, nil
	}
	return common.Marshal(m)
}

type TaskPrivateData struct {
	Key            string `json:"key,omitempty"`
	UpstreamTaskID string `json:"upstream_task_id,omitempty"` // 上游真实 task ID
	ResultURL      string `json:"result_url,omitempty"`       // 任务成功后的结果 URL（视频地址等）
	// 计费上下文：用于异步退款/差额结算（轮询阶段读取）
	BillingSource  string              `json:"billing_source,omitempty"`  // "wallet" 或 "subscription"
	SubscriptionId int                 `json:"subscription_id,omitempty"` // 订阅 ID，用于订阅退款
	TokenId        int                 `json:"token_id,omitempty"`        // 令牌 ID，用于令牌额度退款
	BillingContext *TaskBillingContext `json:"billing_context,omitempty"` // 计费参数快照（用于轮询阶段重新计算）
}

// TaskBillingContext 记录任务提交时的计费参数，以便轮询阶段可以重新计算额度。
type TaskBillingContext struct {
	ModelPrice      float64            `json:"model_price,omitempty"`       // 模型单价
	GroupRatio      float64            `json:"group_ratio,omitempty"`       // 分组倍率
	ModelRatio      float64            `json:"model_ratio,omitempty"`       // 模型倍率
	OtherRatios     map[string]float64 `json:"other_ratios,omitempty"`      // 附加倍率（时长、分辨率等）
	OriginModelName string             `json:"origin_model_name,omitempty"` // 模型名称，必须为OriginModelName
	PerCallBilling  bool               `json:"per_call_billing,omitempty"`  // 按次计费：跳过轮询阶段的差额结算
}

// GetUpstreamTaskID 获取上游真实 task ID（用于与 provider 通信）
// 旧数据没有 UpstreamTaskID 时，TaskID 本身就是上游 ID
func (t *Task) GetUpstreamTaskID() string {
	if t.PrivateData.UpstreamTaskID != "" {
		return t.PrivateData.UpstreamTaskID
	}
	return t.TaskID
}

// GetResultURL 获取任务结果 URL（视频地址等）
// 新数据存在 PrivateData.ResultURL 中；旧数据回退到 FailReason（历史兼容）
func (t *Task) GetResultURL() string {
	if t.PrivateData.ResultURL != "" {
		return t.PrivateData.ResultURL
	}
	return t.FailReason
}

// GenerateTaskID 生成对外暴露的 task_xxxx 格式 ID
func GenerateTaskID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "task_" + key
}

func (p *TaskPrivateData) Scan(val interface{}) error {
	bytesValue, _ := val.([]byte)
	if len(bytesValue) == 0 {
		return nil
	}
	return common.Unmarshal(bytesValue, p)
}

func (p TaskPrivateData) Value() (driver.Value, error) {
	if (p == TaskPrivateData{}) {
		return nil, nil
	}
	return common.Marshal(p)
}

// SyncTaskQueryParams 用于包含所有搜索条件的结构体，可以根据需求添加更多字段
type SyncTaskQueryParams struct {
	Platform       constant.TaskPlatform
	ChannelID      string
	TaskID         string
	UserID         string
	Action         string
	Status         string
	StartTimestamp int64
	EndTimestamp   int64
	UserIDs        []int
}

func InitTask(platform constant.TaskPlatform, relayInfo *commonRelay.RelayInfo) *Task {
	properties := Properties{}
	privateData := TaskPrivateData{}
	if relayInfo != nil && relayInfo.ChannelMeta != nil {
		if relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeGemini ||
			relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeVertexAi {
			privateData.Key = relayInfo.ChannelMeta.ApiKey
		}
		if relayInfo.UpstreamModelName != "" {
			properties.UpstreamModelName = relayInfo.UpstreamModelName
		}
		if relayInfo.OriginModelName != "" {
			properties.OriginModelName = relayInfo.OriginModelName
		}
	}

	// 使用预生成的公开 ID（如果有），否则新生成
	taskID := ""
	if relayInfo.TaskRelayInfo != nil && relayInfo.TaskRelayInfo.PublicTaskID != "" {
		taskID = relayInfo.TaskRelayInfo.PublicTaskID
	} else {
		taskID = GenerateTaskID()
	}

	t := &Task{
		TaskID:            taskID,
		UserId:            relayInfo.UserId,
		Group:             relayInfo.UsingGroup,
		SubmitTime:        time.Now().Unix(),
		Status:            TaskStatusNotStart,
		Progress:          "0%",
		ChannelId:         relayInfo.ChannelId,
		Platform:          platform,
		OriginModelName:   properties.OriginModelName,
		UpstreamModelName: properties.UpstreamModelName,
		Properties:        properties,
		PrivateData:       privateData,
	}
	return t
}

func taskScopesForQuery(queryParams SyncTaskQueryParams) []string {
	return []string{taskStorageScopeMain}
}

func videoTasksUseDedicatedStore() bool {
	return CanvasModeUsesDedicatedMessageDB(CanvasModeVideo)
}

func taskLookupScopes() []string {
	return []string{taskStorageScopeMain}
}

func taskPollingScopes() []string {
	if videoTasksUseDedicatedStore() {
		return []string{taskStorageScopeMain, taskStorageScopeVideo}
	}
	return []string{taskStorageScopeMain}
}

func applyTaskStorageScopeFilters(query *gorm.DB, scope string) *gorm.DB {
	return query
}

func applySyncTaskFilters(query *gorm.DB, queryParams SyncTaskQueryParams) *gorm.DB {
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.UserID != "" {
		query = query.Where("user_id = ?", queryParams.UserID)
	}
	if len(queryParams.UserIDs) != 0 {
		query = query.Where("user_id in (?)", queryParams.UserIDs)
	}
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	return query
}

func listTasksForScope(scope string, queryParams SyncTaskQueryParams, extra func(*gorm.DB) *gorm.DB) ([]*Task, error) {
	db, err := taskDBForScope(scope)
	if err != nil {
		if scope == taskStorageScopeVideo && IsCanvasModeUnavailableError(err) {
			return []*Task{}, nil
		}
		return nil, err
	}
	query := db.Model(&Task{})
	query = applyTaskStorageScopeFilters(query, scope)
	query = applySyncTaskFilters(query, queryParams)
	if extra != nil {
		query = extra(query)
	}
	var tasks []*Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	markTasksStorageScope(tasks, scope)
	return tasks, nil
}

func sortTasksNewestFirst(tasks []*Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].SubmitTime == tasks[j].SubmitTime {
			if tasks[i].ID == tasks[j].ID {
				return taskScopeForTask(tasks[i]) < taskScopeForTask(tasks[j])
			}
			return tasks[i].ID > tasks[j].ID
		}
		return tasks[i].SubmitTime > tasks[j].SubmitTime
	})
}

func sortTasksOldestFirst(tasks []*Task) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].SubmitTime == tasks[j].SubmitTime {
			if tasks[i].ID == tasks[j].ID {
				return taskScopeForTask(tasks[i]) < taskScopeForTask(tasks[j])
			}
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].SubmitTime < tasks[j].SubmitTime
	})
}

func TaskGetAllUserTask(userId int, startIdx int, num int, queryParams SyncTaskQueryParams) []*Task {
	if startIdx < 0 {
		startIdx = 0
	}
	db, err := taskDBForScope(taskStorageScopeMain)
	if err != nil {
		return nil
	}
	query := db.Model(&Task{}).Omit("channel_id")
	query = applyTaskStorageScopeFilters(query, taskStorageScopeMain)
	query = applySyncTaskFilters(query.Where("user_id = ?", userId), queryParams)
	query = query.Order("id desc").Offset(startIdx)
	if num > 0 {
		query = query.Limit(num)
	}
	var tasks []*Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil
	}
	markTasksStorageScope(tasks, taskStorageScopeMain)
	return tasks
}

func TaskGetAllTasks(startIdx int, num int, queryParams SyncTaskQueryParams) []*Task {
	if startIdx < 0 {
		startIdx = 0
	}
	db, err := taskDBForScope(taskStorageScopeMain)
	if err != nil {
		return nil
	}
	query := db.Model(&Task{})
	query = applyTaskStorageScopeFilters(query, taskStorageScopeMain)
	query = applySyncTaskFilters(query, queryParams)
	query = query.Order("id desc").Offset(startIdx)
	if num > 0 {
		query = query.Limit(num)
	}
	var tasks []*Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil
	}
	markTasksStorageScope(tasks, taskStorageScopeMain)
	return tasks
}

func GetTimedOutUnfinishedTasks(cutoffUnix int64, limit int) []*Task {
	items := make([]*Task, 0)
	queryParams := SyncTaskQueryParams{
		EndTimestamp: cutoffUnix - 1,
	}
	for _, scope := range taskPollingScopes() {
		tasks, err := listTasksForScope(scope, queryParams, func(query *gorm.DB) *gorm.DB {
			return query.Where("progress != ?", "100%").
				Where("status NOT IN ?", []string{TaskStatusFailure, TaskStatusSuccess})
		})
		if err != nil {
			return nil
		}
		items = append(items, tasks...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SubmitTime == items[j].SubmitTime {
			return items[i].ID < items[j].ID
		}
		return items[i].SubmitTime < items[j].SubmitTime
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func GetAllUnFinishSyncTasks(limit int) []*Task {
	items := make([]*Task, 0)
	for _, scope := range taskPollingScopes() {
		tasks, err := listTasksForScope(scope, SyncTaskQueryParams{}, func(query *gorm.DB) *gorm.DB {
			return query.Where("progress != ?", "100%").
				Where("status != ?", TaskStatusFailure).
				Where("status != ?", TaskStatusSuccess)
		})
		if err != nil {
			return nil
		}
		items = append(items, tasks...)
	}
	sortTasksOldestFirst(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func GetByOnlyTaskId(taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	for _, scope := range taskLookupScopes() {
		db, err := taskDBForScope(scope)
		if err != nil {
			if scope == taskStorageScopeVideo && IsCanvasModeUnavailableError(err) {
				continue
			}
			return nil, false, err
		}
		var task Task
		query := applyTaskStorageScopeFilters(db.Model(&Task{}), scope)
		err = query.Where("task_id = ?", taskId).First(&task).Error
		exist, recordErr := RecordExist(err)
		if recordErr != nil {
			return nil, false, recordErr
		}
		if exist {
			task.StorageScope = scope
			return &task, true, nil
		}
	}
	return nil, false, nil
}

func GetByTaskId(userId int, taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	for _, scope := range taskLookupScopes() {
		db, err := taskDBForScope(scope)
		if err != nil {
			if scope == taskStorageScopeVideo && IsCanvasModeUnavailableError(err) {
				continue
			}
			return nil, false, err
		}
		var task Task
		query := applyTaskStorageScopeFilters(db.Model(&Task{}), scope)
		err = query.Where("user_id = ? and task_id = ?", userId, taskId).First(&task).Error
		exist, recordErr := RecordExist(err)
		if recordErr != nil {
			return nil, false, recordErr
		}
		if exist {
			task.StorageScope = scope
			return &task, true, nil
		}
	}
	return nil, false, nil
}

func GetByTaskIds(userId int, taskIds []any) ([]*Task, error) {
	if len(taskIds) == 0 {
		return nil, nil
	}
	result := make([]*Task, 0)
	seenTaskIDs := make(map[string]struct{}, len(taskIds))
	for _, scope := range taskLookupScopes() {
		db, err := taskDBForScope(scope)
		if err != nil {
			if scope == taskStorageScopeVideo && IsCanvasModeUnavailableError(err) {
				continue
			}
			return nil, err
		}
		var tasks []*Task
		query := applyTaskStorageScopeFilters(db.Model(&Task{}), scope)
		if err := query.Where("user_id = ? and task_id in (?)", userId, taskIds).Find(&tasks).Error; err != nil {
			return nil, err
		}
		markTasksStorageScope(tasks, scope)
		for _, task := range tasks {
			if task == nil {
				continue
			}
			taskKey := strings.TrimSpace(task.TaskID)
			if taskKey == "" {
				taskKey = fmt.Sprintf("%s:%d", scope, task.ID)
			}
			if _, ok := seenTaskIDs[taskKey]; ok {
				continue
			}
			seenTaskIDs[taskKey] = struct{}{}
			result = append(result, task)
		}
	}
	return result, nil
}

func (Task *Task) Insert() error {
	db, err := taskDBForTask(Task)
	if err != nil {
		return err
	}
	Task.StorageScope = taskScopeForTask(Task)
	return db.Create(Task).Error
}

type taskSnapshot struct {
	Status     TaskStatus
	Progress   string
	StartTime  int64
	FinishTime int64
	FailReason string
	ResultURL  string
	Data       json.RawMessage
}

func (s taskSnapshot) Equal(other taskSnapshot) bool {
	return s.Status == other.Status &&
		s.Progress == other.Progress &&
		s.StartTime == other.StartTime &&
		s.FinishTime == other.FinishTime &&
		s.FailReason == other.FailReason &&
		s.ResultURL == other.ResultURL &&
		bytes.Equal(s.Data, other.Data)
}

func (t *Task) Snapshot() taskSnapshot {
	return taskSnapshot{
		Status:     t.Status,
		Progress:   t.Progress,
		StartTime:  t.StartTime,
		FinishTime: t.FinishTime,
		FailReason: t.FailReason,
		ResultURL:  t.PrivateData.ResultURL,
		Data:       t.Data,
	}
}

func (Task *Task) Update() error {
	db, err := taskDBForTask(Task)
	if err != nil {
		return err
	}
	return db.Save(Task).Error
}

func (t *Task) EffectiveOriginModelName() string {
	if t == nil {
		return ""
	}
	value := strings.TrimSpace(t.OriginModelName)
	if value != "" {
		return value
	}
	return strings.TrimSpace(t.Properties.OriginModelName)
}

func (t *Task) EffectiveUpstreamModelName() string {
	if t == nil {
		return ""
	}
	value := strings.TrimSpace(t.UpstreamModelName)
	if value != "" {
		return value
	}
	return strings.TrimSpace(t.Properties.UpstreamModelName)
}

func (t *Task) syncModelFieldsFromProperties() {
	if t == nil {
		return
	}
	t.OriginModelName = strings.TrimSpace(t.Properties.OriginModelName)
	t.UpstreamModelName = strings.TrimSpace(t.Properties.UpstreamModelName)
}

func (t *Task) BeforeSave(tx *gorm.DB) error {
	t.syncModelFieldsFromProperties()
	return nil
}

// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Returns (true, nil) if this caller won the update, (false, nil) if
// another process already moved the task out of fromStatus.
//
// Uses Model().Select("*").Updates() instead of Save() because GORM's Save
// falls back to INSERT ON CONFLICT when the WHERE-guarded UPDATE matches
// zero rows, which silently bypasses the CAS guard.
func (t *Task) UpdateWithStatus(fromStatus TaskStatus) (bool, error) {
	db, err := taskDBForTask(t)
	if err != nil {
		return false, err
	}
	result := db.Model(t).Where("status = ?", fromStatus).Select("*").Updates(t)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// TaskBulkUpdateByID performs an unconditional bulk UPDATE by primary key IDs.
// WARNING: This function has NO CAS (Compare-And-Swap) guard — it will overwrite
// any concurrent status changes. DO NOT use in billing/quota lifecycle flows
// (e.g., timeout, success, failure transitions that trigger refunds or settlements).
// For status transitions that involve billing, use Task.UpdateWithStatus() instead.
func TaskBulkUpdateByID(ids []int64, params map[string]any) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Model(&Task{}).
		Where("id in (?)", ids).
		Updates(params).Error
}

func TaskBulkUpdateTasks(tasks []*Task, params map[string]any) error {
	groupedIDs := map[string][]int64{}
	for _, task := range tasks {
		if task == nil || task.ID <= 0 {
			continue
		}
		scope := taskScopeForTask(task)
		groupedIDs[scope] = append(groupedIDs[scope], task.ID)
	}
	for scope, ids := range groupedIDs {
		db, err := taskDBForScope(scope)
		if err != nil {
			return err
		}
		if err := db.Model(&Task{}).Where("id in (?)", ids).Updates(params).Error; err != nil {
			return err
		}
	}
	return nil
}

type TaskQuotaUsage struct {
	Mode  string  `json:"mode"`
	Count float64 `json:"count"`
}

// TaskCountAllTasks returns total tasks that match the given query params (admin usage)
func TaskCountAllTasks(queryParams SyncTaskQueryParams) int64 {
	var total int64
	for _, scope := range taskScopesForQuery(queryParams) {
		db, err := taskDBForScope(scope)
		if err != nil {
			continue
		}
		var count int64
		query := applyTaskStorageScopeFilters(db.Model(&Task{}), scope)
		query = applySyncTaskFilters(query, queryParams)
		_ = query.Count(&count).Error
		total += count
	}
	return total
}

// TaskCountAllUserTask returns total tasks for given user
func TaskCountAllUserTask(userId int, queryParams SyncTaskQueryParams) int64 {
	var total int64
	for _, scope := range taskScopesForQuery(queryParams) {
		db, err := taskDBForScope(scope)
		if err != nil {
			continue
		}
		var count int64
		query := applyTaskStorageScopeFilters(db.Model(&Task{}), scope)
		query = applySyncTaskFilters(query.Where("user_id = ?", userId), queryParams)
		_ = query.Count(&count).Error
		total += count
	}
	return total
}
func (t *Task) ToOpenAIVideo() *dto.OpenAIVideo {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = t.TaskID
	openAIVideo.Status = t.Status.ToVideoStatus()
	openAIVideo.Model = t.EffectiveOriginModelName()
	openAIVideo.SetProgressStr(t.Progress)
	openAIVideo.CreatedAt = t.CreatedAt
	openAIVideo.CompletedAt = t.UpdatedAt
	openAIVideo.SetMetadata("url", t.GetResultURL())
	return openAIVideo
}
