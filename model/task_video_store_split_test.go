package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCanvasVideoTaskChildDB(t *testing.T, envSuffix string) *gorm.DB {
	t.Helper()

	childDSN := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), envSuffix)
	childDB, err := gorm.Open(sqlite.Open(childDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open video child sqlite db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := childDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		if envName != "CANVAS_VIDEO_SQL_DSN" {
			t.Fatalf("unexpected dedicated DB open request: %s", envName)
		}
		return childDB, nil
	}
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video-"+envSuffix)
	InitCanvasDBs()
	return childDB
}

func TestTaskInsertUsesMainDBWhenCanvasVideoDSNConfigured(t *testing.T) {
	mainDB := setupCanvasMessageStoreTestDB(t)
	if err := mainDB.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	childDB := setupCanvasVideoTaskChildDB(t, "ordinary-insert")

	task := &Task{
		TaskID:     "task_ordinary_video",
		UserId:     31,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusQueued,
		Progress:   "0%",
		SubmitTime: 100,
	}
	if err := task.Insert(); err != nil {
		t.Fatalf("failed to insert ordinary video task: %v", err)
	}
	if task.StorageScope != taskStorageScopeMain {
		t.Fatalf("expected main storage scope, got %q", task.StorageScope)
	}

	var mainCount int64
	if err := mainDB.Model(&Task{}).Where("task_id = ?", task.TaskID).Count(&mainCount).Error; err != nil {
		t.Fatalf("failed to count main tasks: %v", err)
	}
	if mainCount != 1 {
		t.Fatalf("expected ordinary task in main DB, got %d", mainCount)
	}
	var childCount int64
	if err := childDB.Model(&Task{}).Where("task_id = ?", task.TaskID).Count(&childCount).Error; err != nil {
		t.Fatalf("failed to count child tasks: %v", err)
	}
	if childCount != 0 {
		t.Fatalf("ordinary task should not be written to canvas video DB, got %d", childCount)
	}
}

func TestExplicitCanvasVideoTaskScopeUsesChildDB(t *testing.T) {
	mainDB := setupCanvasMessageStoreTestDB(t)
	if err := mainDB.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	childDB := setupCanvasVideoTaskChildDB(t, "explicit-scope")

	task := &Task{
		TaskID:     "task_canvas_video",
		UserId:     32,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusQueued,
		Progress:   "0%",
		SubmitTime: 200,
	}
	if err := task.InsertCanvasVideo(); err != nil {
		t.Fatalf("failed to insert canvas video task: %v", err)
	}
	if task.StorageScope != taskStorageScopeVideo {
		t.Fatalf("expected canvas video storage scope, got %q", task.StorageScope)
	}

	var childCount int64
	if err := childDB.Model(&Task{}).Where("task_id = ?", task.TaskID).Count(&childCount).Error; err != nil {
		t.Fatalf("failed to count child tasks: %v", err)
	}
	if childCount != 1 {
		t.Fatalf("expected canvas video task in child DB, got %d", childCount)
	}
	var mainCount int64
	if err := mainDB.Model(&Task{}).Where("task_id = ?", task.TaskID).Count(&mainCount).Error; err != nil {
		t.Fatalf("failed to count main tasks: %v", err)
	}
	if mainCount != 0 {
		t.Fatalf("canvas video task should not be written to main DB in split mode, got %d", mainCount)
	}

	if found, exists, err := GetCanvasVideoTaskByTaskID(task.UserId, task.TaskID); err != nil || !exists || found == nil {
		t.Fatalf("expected canvas video lookup to find task, exists=%v task=%#v err=%v", exists, found, err)
	}
	if found, exists, err := GetByTaskId(task.UserId, task.TaskID); err != nil || exists || found != nil {
		t.Fatalf("ordinary task lookup should not read canvas video DB, exists=%v task=%#v err=%v", exists, found, err)
	}
}

func TestVideoTaskDetailLookupsStayScopedWhenNumericIDsCollide(t *testing.T) {
	mainDB := setupCanvasMessageStoreTestDB(t)
	if err := mainDB.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	setupCanvasVideoTaskChildDB(t, "detail-id-collision")

	userID := 33
	ordinaryTask := &Task{
		TaskID:     "task_main_collision",
		UserId:     userID,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: 300,
	}
	if err := ordinaryTask.Insert(); err != nil {
		t.Fatalf("failed to insert ordinary task: %v", err)
	}
	canvasTask := &Task{
		TaskID:     "task_canvas_collision",
		UserId:     userID,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: 400,
	}
	if err := canvasTask.InsertCanvasVideo(); err != nil {
		t.Fatalf("failed to insert canvas video task: %v", err)
	}
	if ordinaryTask.ID != canvasTask.ID {
		t.Fatalf("expected numeric IDs to collide across main and canvas video DBs, got main=%d canvas=%d", ordinaryTask.ID, canvasTask.ID)
	}

	ordinaryFound, err := GetUserVideoTaskByID(userID, ordinaryTask.ID, nil)
	if err != nil {
		t.Fatalf("failed to load ordinary task by id: %v", err)
	}
	if ordinaryFound.TaskID != ordinaryTask.TaskID || ordinaryFound.StorageScope != taskStorageScopeMain {
		t.Fatalf("ordinary detail lookup read wrong task: %#v", ordinaryFound)
	}
	canvasFound, err := GetCanvasVideoTaskByID(userID, canvasTask.ID, nil)
	if err != nil {
		t.Fatalf("failed to load canvas task by id: %v", err)
	}
	if canvasFound.TaskID != canvasTask.TaskID || canvasFound.StorageScope != taskStorageScopeVideo {
		t.Fatalf("canvas detail lookup read wrong task: %#v", canvasFound)
	}
}

func TestOrdinaryVideoActionsRemainVisibleInMainTaskQueries(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	_ = setupCanvasVideoTaskChildDB(t, "ordinary-queries")

	tasks := []*Task{
		{
			TaskID:     "task_generate",
			UserId:     42,
			ChannelId:  901,
			Action:     constant.TaskActionGenerate,
			Status:     TaskStatusQueued,
			Progress:   "0%",
			SubmitTime: 100,
		},
		{
			TaskID:     "task_text_generate",
			UserId:     42,
			ChannelId:  902,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusQueued,
			Progress:   "0%",
			SubmitTime: 200,
		},
	}
	for _, task := range tasks {
		if err := task.Insert(); err != nil {
			t.Fatalf("failed to insert ordinary task %s: %v", task.TaskID, err)
		}
	}

	userTasks := TaskGetAllUserTask(42, 0, 20, SyncTaskQueryParams{})
	if len(userTasks) != 2 || userTasks[0].TaskID != "task_text_generate" || userTasks[1].TaskID != "task_generate" {
		t.Fatalf("expected ordinary video actions in main user task list, got %#v", userTasks)
	}
	if userTasks[0].ChannelId != 0 || userTasks[1].ChannelId != 0 {
		t.Fatalf("expected user task list to omit channel_id, got %#v", userTasks)
	}
	pagedUserTasks := TaskGetAllUserTask(42, 0, 1, SyncTaskQueryParams{})
	if len(pagedUserTasks) != 1 || pagedUserTasks[0].TaskID != "task_text_generate" {
		t.Fatalf("expected database-side user pagination in id desc order, got %#v", pagedUserTasks)
	}
	adminTasks := TaskGetAllTasks(0, 20, SyncTaskQueryParams{ChannelID: "902"})
	if len(adminTasks) != 1 || adminTasks[0].TaskID != "task_text_generate" || adminTasks[0].ChannelId != 902 {
		t.Fatalf("expected admin task query to preserve channel_id filtering, got %#v", adminTasks)
	}
	if total := TaskCountAllUserTask(42, SyncTaskQueryParams{}); total != 2 {
		t.Fatalf("expected ordinary video actions in main task count, got %d", total)
	}
	found, exists, err := GetByTaskId(42, "task_generate")
	if err != nil || !exists || found == nil || found.StorageScope != taskStorageScopeMain {
		t.Fatalf("expected ordinary fetch from main DB, exists=%v task=%#v err=%v", exists, found, err)
	}
	foundBatch, err := GetByTaskIds(42, []any{"task_generate", "task_text_generate"})
	if err != nil || len(foundBatch) != 2 {
		t.Fatalf("expected ordinary batch fetch from main DB, tasks=%#v err=%v", foundBatch, err)
	}
	page, err := GetUserVideoTasks(42, 0, 20, VideoTaskQueryParams{}, nil)
	if err != nil || len(page.Items) != 2 || page.Items[0].TaskID != "task_text_generate" {
		t.Fatalf("expected ordinary video page from main DB, page=%#v err=%v", page, err)
	}
}

func TestCanvasVideoDBUnavailableDoesNotBreakMainTaskQueriesOrPolling(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		return nil, fmt.Errorf("child DB unavailable")
	}
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video-unavailable")
	InitCanvasDBs()

	task := &Task{
		TaskID:     "task_main_polling",
		UserId:     43,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusQueued,
		Progress:   "0%",
		SubmitTime: 100,
	}
	if err := task.Insert(); err != nil {
		t.Fatalf("ordinary insert should not depend on canvas video DB: %v", err)
	}

	if total := TaskCountAllUserTask(43, SyncTaskQueryParams{}); total != 1 {
		t.Fatalf("main task count should ignore unavailable canvas video DB, got %d", total)
	}
	found, exists, err := GetByTaskId(43, task.TaskID)
	if err != nil || !exists || found == nil {
		t.Fatalf("main task fetch should ignore unavailable canvas video DB, exists=%v task=%#v err=%v", exists, found, err)
	}
	unfinished := GetAllUnFinishSyncTasks(20)
	if len(unfinished) != 1 || unfinished[0].TaskID != task.TaskID {
		t.Fatalf("polling should include main task and skip unavailable canvas video DB, got %#v", unfinished)
	}
}

func TestTaskPollingAggregatesMainAndCanvasVideoStores(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	_ = setupCanvasVideoTaskChildDB(t, "polling-aggregate")

	mainTask := &Task{
		TaskID:     "task_main_video",
		UserId:     44,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusQueued,
		Progress:   "0%",
		SubmitTime: 100,
	}
	canvasTask := &Task{
		TaskID:     "task_canvas_video_poll",
		UserId:     44,
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusQueued,
		Progress:   "0%",
		SubmitTime: 200,
	}
	if err := mainTask.Insert(); err != nil {
		t.Fatalf("failed to insert main task: %v", err)
	}
	if err := canvasTask.InsertCanvasVideo(); err != nil {
		t.Fatalf("failed to insert canvas video task: %v", err)
	}

	unfinished := GetAllUnFinishSyncTasks(20)
	seen := map[string]string{}
	for _, task := range unfinished {
		if task != nil {
			seen[task.TaskID] = task.StorageScope
		}
	}
	if seen[mainTask.TaskID] != taskStorageScopeMain || seen[canvasTask.TaskID] != taskStorageScopeVideo {
		t.Fatalf("expected polling to aggregate main and canvas video tasks, got %#v", seen)
	}
}
