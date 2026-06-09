package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestImageGenerationTasksCompatibilityModeUseMainDB(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&ImageGenerationTask{}); err != nil {
		t.Fatalf("failed to migrate main image task table: %v", err)
	}

	task := &ImageGenerationTask{
		UserId:          11,
		ModelId:         "gpt-image-main",
		Prompt:          "compat task",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusPending,
	}
	if err := task.Insert(); err != nil {
		t.Fatalf("failed to insert compatibility image task: %v", err)
	}

	reloaded, err := GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload compatibility image task: %v", err)
	}
	if reloaded == nil || reloaded.UserId != task.UserId {
		t.Fatalf("unexpected compatibility image task: %#v", reloaded)
	}

	var count int64
	if err := db.Model(&ImageGenerationTask{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count main image tasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 main image task row, got %d", count)
	}
}

func TestImageGenerationTasksSplitModeUseDedicatedImageDB(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&ImageGenerationTask{}); err != nil {
		t.Fatalf("failed to migrate main image task table: %v", err)
	}

	childDSN := fmt.Sprintf("file:%s_image_tasks?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	childDB, err := gorm.Open(sqlite.Open(childDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open image child sqlite db: %v", err)
	}
	defer func() {
		sqlDB, dbErr := childDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	}()

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		if envName != "CANVAS_IMAGE_SQL_DSN" {
			t.Fatalf("unexpected dedicated DB open request: %s", envName)
		}
		return childDB, nil
	}
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-tasks")

	InitCanvasDBs()

	if !childDB.Migrator().HasTable("image_generation_tasks") {
		t.Fatal("expected dedicated image task table after canvas DB init")
	}

	task := &ImageGenerationTask{
		UserId:          22,
		ModelId:         "gpt-image-split",
		Prompt:          "split task",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusPending,
	}
	if err := task.Insert(); err != nil {
		t.Fatalf("failed to insert split image task: %v", err)
	}

	reloaded, err := GetImageTaskByID(task.Id)
	if err != nil {
		t.Fatalf("failed to reload split image task: %v", err)
	}
	if reloaded == nil || reloaded.UserId != task.UserId {
		t.Fatalf("unexpected split image task: %#v", reloaded)
	}

	var childCount int64
	if err := childDB.Model(&ImageGenerationTask{}).Count(&childCount).Error; err != nil {
		t.Fatalf("failed to count dedicated image tasks: %v", err)
	}
	if childCount != 1 {
		t.Fatalf("expected 1 dedicated image task row, got %d", childCount)
	}

	var mainCount int64
	if err := db.Model(&ImageGenerationTask{}).Count(&mainCount).Error; err != nil {
		t.Fatalf("failed to count main image tasks: %v", err)
	}
	if mainCount != 0 {
		t.Fatalf("expected 0 main image task rows in split mode, got %d", mainCount)
	}
}
