package model

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func TestReconcileAllUserImageGenerationActiveTaskCountsDoesNotClearOnImageStoreError(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	if err := db.AutoMigrate(&User{}, &ImageGenerationTask{}); err != nil {
		t.Fatalf("failed to migrate user/image task tables: %v", err)
	}
	user := &User{
		Id:                         71,
		Username:                   "image-reconcile-user",
		Password:                   "password123",
		ImageGenerationActiveTasks: 7,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		if envName == "CANVAS_IMAGE_SQL_DSN" {
			return nil, fmt.Errorf("image store unavailable")
		}
		return DB, nil
	}
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-unavailable")
	InitCanvasDBs()

	if _, err := ReconcileAllUserImageGenerationActiveTaskCounts(); err == nil {
		t.Fatal("expected reconcile to fail when image task store is unavailable")
	}

	var reloaded User
	if err := db.First(&reloaded, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.ImageGenerationActiveTasks != 7 {
		t.Fatalf("expected user active task count to remain unchanged, got %d", reloaded.ImageGenerationActiveTasks)
	}
}
