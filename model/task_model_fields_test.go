package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTaskModelFieldTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	InitCommonColumnNames()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	if err := db.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate task table: %v", err)
	}

	t.Cleanup(func() {
		DB = previousDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		InitCommonColumnNames()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestTaskCreateSyncsModelFieldsFromProperties(t *testing.T) {
	db := setupTaskModelFieldTestDB(t)

	task := &Task{
		TaskID:   "task_sync_create",
		UserId:   1,
		Action:   "GENERATE",
		Status:   TaskStatusSuccess,
		Progress: "100%",
		Properties: Properties{
			OriginModelName:   "origin-alpha",
			UpstreamModelName: "upstream-alpha",
		},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	var reloaded Task
	if err := db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded.OriginModelName != "origin-alpha" {
		t.Fatalf("expected synced origin model name, got %q", reloaded.OriginModelName)
	}
	if reloaded.UpstreamModelName != "upstream-alpha" {
		t.Fatalf("expected synced upstream model name, got %q", reloaded.UpstreamModelName)
	}
}

func TestTaskUpdateSyncsModelFieldsFromProperties(t *testing.T) {
	db := setupTaskModelFieldTestDB(t)

	task := &Task{
		TaskID:            "task_sync_update",
		UserId:            1,
		Action:            "GENERATE",
		Status:            TaskStatusQueued,
		Progress:          "0%",
		OriginModelName:   "stale-origin",
		UpstreamModelName: "stale-upstream",
		Properties: Properties{
			OriginModelName:   "origin-beta",
			UpstreamModelName: "upstream-beta",
		},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	task.Properties.OriginModelName = "origin-gamma"
	task.Properties.UpstreamModelName = "upstream-gamma"
	task.Progress = "50%"
	if err := task.Update(); err != nil {
		t.Fatalf("failed to update task: %v", err)
	}

	var reloaded Task
	if err := db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloaded.OriginModelName != "origin-gamma" {
		t.Fatalf("expected updated origin model name, got %q", reloaded.OriginModelName)
	}
	if reloaded.UpstreamModelName != "upstream-gamma" {
		t.Fatalf("expected updated upstream model name, got %q", reloaded.UpstreamModelName)
	}
}

func TestTaskEffectiveModelNamesFallbackToProperties(t *testing.T) {
	task := &Task{
		Properties: Properties{
			OriginModelName:   "origin-fallback",
			UpstreamModelName: "upstream-fallback",
		},
	}
	if got := task.EffectiveOriginModelName(); got != "origin-fallback" {
		t.Fatalf("expected origin fallback, got %q", got)
	}
	if got := task.EffectiveUpstreamModelName(); got != "upstream-fallback" {
		t.Fatalf("expected upstream fallback, got %q", got)
	}
}

func TestTaskAutoMigrateCreatesVideoTaskQueryIndexes(t *testing.T) {
	db := setupTaskModelFieldTestDB(t)

	type sqliteIndexRow struct {
		Name string
	}

	var rows []sqliteIndexRow
	if err := db.Raw(`PRAGMA index_list("tasks")`).Scan(&rows).Error; err != nil {
		t.Fatalf("failed to inspect sqlite indexes: %v", err)
	}

	indexNames := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		indexNames[row.Name] = struct{}{}
	}
	for _, expected := range []string{
		"idx_tasks_user_action_submit_id",
		"idx_tasks_user_action_status_submit_id",
		"idx_tasks_user_action_status_finish_id",
		"idx_tasks_user_action_origin_model_id",
		"idx_tasks_user_action_upstream_model_id",
	} {
		if _, ok := indexNames[expected]; !ok {
			t.Fatalf("expected sqlite index %s to exist, got %#v", expected, indexNames)
		}
	}
}

func TestTaskGetUserVideoTasksCursorPaginatesBySubmitTime(t *testing.T) {
	db := setupTaskModelFieldTestDB(t)

	for index, submitTime := range []int64{300, 200, 100} {
		task := &Task{
			TaskID:     fmt.Sprintf("task_cursor_%d", index),
			UserId:     1,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: submitTime,
			Properties: Properties{
				Input:             fmt.Sprintf("cursor prompt %d", index),
				OriginModelName:   "video-alpha",
				UpstreamModelName: "video-alpha",
			},
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create cursor task: %v", err)
		}
	}

	firstPage, err := GetUserVideoTasks(1, 0, 2, VideoTaskQueryParams{}, nil)
	if err != nil {
		t.Fatalf("GetUserVideoTasks returned error: %v", err)
	}
	if !firstPage.HasTotal || firstPage.Total != 3 {
		t.Fatalf("expected first page total 3, got %#v", firstPage)
	}
	if len(firstPage.Items) != 2 || firstPage.Items[0].SubmitTime != 300 || firstPage.Items[1].SubmitTime != 200 {
		t.Fatalf("unexpected first page items: %#v", firstPage.Items)
	}
	if !firstPage.HasMore || firstPage.NextCursor == "" {
		t.Fatalf("expected first page next cursor, got %#v", firstPage)
	}

	secondPage, err := GetUserVideoTasksByCursor(1, firstPage.NextCursor, 2, VideoTaskQueryParams{}, nil)
	if err != nil {
		t.Fatalf("GetUserVideoTasksByCursor returned error: %v", err)
	}
	if secondPage.HasTotal {
		t.Fatalf("expected cursor page to skip total recount, got %#v", secondPage)
	}
	if secondPage.HasMore {
		t.Fatalf("expected second page to be terminal, got %#v", secondPage)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].SubmitTime != 100 {
		t.Fatalf("unexpected second page items: %#v", secondPage.Items)
	}
}

func TestTaskGetUserVideoTaskUpdatesReturnsRunningAndRecentCompleted(t *testing.T) {
	db := setupTaskModelFieldTestDB(t)

	for _, task := range []*Task{
		{
			TaskID:     "task_updates_running",
			UserId:     1,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusQueued,
			Progress:   "10%",
			SubmitTime: 500,
			Properties: Properties{
				Input:             "running prompt",
				OriginModelName:   "video-alpha",
				UpstreamModelName: "video-alpha",
			},
		},
		{
			TaskID:     "task_updates_recent",
			UserId:     1,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 400,
			FinishTime: 450,
			Properties: Properties{
				Input:             "recent prompt",
				OriginModelName:   "video-beta",
				UpstreamModelName: "video-beta",
			},
		},
		{
			TaskID:     "task_updates_old",
			UserId:     1,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 200,
			FinishTime: 250,
			Properties: Properties{
				Input:             "old prompt",
				OriginModelName:   "video-gamma",
				UpstreamModelName: "video-gamma",
			},
		},
		{
			TaskID:     "task_updates_other_action",
			UserId:     1,
			Action:     "other-action",
			Status:     TaskStatusQueued,
			Progress:   "5%",
			SubmitTime: 600,
		},
		{
			TaskID:     "task_updates_other_user",
			UserId:     2,
			Action:     constant.TaskActionTextGenerate,
			Status:     TaskStatusQueued,
			Progress:   "5%",
			SubmitTime: 700,
		},
	} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create updates task: %v", err)
		}
	}

	items, err := GetUserVideoTaskUpdates(1, 300, 10, nil)
	if err != nil {
		t.Fatalf("GetUserVideoTaskUpdates returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 update items, got %d", len(items))
	}
	if items[0].TaskID != "task_updates_running" || items[0].Status != TaskStatusQueued {
		t.Fatalf("expected running task first, got %#v", items[0])
	}
	if items[1].TaskID != "task_updates_recent" || items[1].Status != TaskStatusSuccess {
		t.Fatalf("expected recent completed task second, got %#v", items[1])
	}
}
