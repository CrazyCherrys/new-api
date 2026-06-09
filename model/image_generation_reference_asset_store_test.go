package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func migrateMainImageReferenceAssetTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&ImageGenerationReferenceAsset{}, &ImageGenerationTaskReferenceAsset{}); err != nil {
		t.Fatalf("failed to migrate image reference asset tables: %v", err)
	}
}

func TestImageReferenceAssetsCompatibilityModeUseMainDB(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	migrateMainImageReferenceAssetTables(t, db)

	asset := &ImageGenerationReferenceAsset{
		ContentHash: "compat-hash",
		StoragePath: "/tmp/reference.png",
	}
	if err := CreateImageGenerationReferenceAsset(asset); err != nil {
		t.Fatalf("failed to create reference asset: %v", err)
	}
	if err := CreateTaskReferenceAssetLink(42, asset.Id); err != nil {
		t.Fatalf("failed to create task reference asset link: %v", err)
	}

	reloaded, err := GetImageGenerationReferenceAssetByHash("compat-hash")
	if err != nil {
		t.Fatalf("failed to load reference asset by hash: %v", err)
	}
	if reloaded == nil || reloaded.Id != asset.Id {
		t.Fatalf("unexpected reference asset reload: %#v", reloaded)
	}

	links, err := ListTaskReferenceAssetLinks(42)
	if err != nil {
		t.Fatalf("failed to list task reference asset links: %v", err)
	}
	if len(links) != 1 || links[0].AssetId != asset.Id {
		t.Fatalf("unexpected task reference asset links: %#v", links)
	}

	var assetCount int64
	if err := db.Model(&ImageGenerationReferenceAsset{}).Count(&assetCount).Error; err != nil {
		t.Fatalf("failed to count main-db reference assets: %v", err)
	}
	if assetCount != 1 {
		t.Fatalf("expected 1 main-db reference asset, got %d", assetCount)
	}
}

func TestImageReferenceAssetsSplitModeUseDedicatedImageDB(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)
	migrateMainImageReferenceAssetTables(t, db)

	childDSN := fmt.Sprintf("file:%s_image_assets?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	childDB, err := gorm.Open(sqlite.Open(childDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open child sqlite db: %v", err)
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
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-assets")

	InitCanvasDBs()

	for _, tableName := range []string{
		"image_generation_reference_assets",
		"image_generation_task_reference_assets",
	} {
		if !childDB.Migrator().HasTable(tableName) {
			t.Fatalf("expected dedicated image table %s after canvas DB init", tableName)
		}
	}

	asset := &ImageGenerationReferenceAsset{
		ContentHash: "split-hash",
		StoragePath: "/tmp/split-reference.png",
	}
	if err := CreateImageGenerationReferenceAsset(asset); err != nil {
		t.Fatalf("failed to create dedicated reference asset: %v", err)
	}
	if err := CreateTaskReferenceAssetLink(77, asset.Id); err != nil {
		t.Fatalf("failed to create dedicated task reference asset link: %v", err)
	}

	var childAssetCount int64
	if err := childDB.Model(&ImageGenerationReferenceAsset{}).Count(&childAssetCount).Error; err != nil {
		t.Fatalf("failed to count dedicated reference assets: %v", err)
	}
	if childAssetCount != 1 {
		t.Fatalf("expected 1 dedicated reference asset, got %d", childAssetCount)
	}

	var mainAssetCount int64
	if err := db.Model(&ImageGenerationReferenceAsset{}).Count(&mainAssetCount).Error; err != nil {
		t.Fatalf("failed to count main-db reference assets: %v", err)
	}
	if mainAssetCount != 0 {
		t.Fatalf("expected 0 main-db reference assets in split mode, got %d", mainAssetCount)
	}

	links, err := ListTaskReferenceAssetLinks(77)
	if err != nil {
		t.Fatalf("failed to list dedicated task reference asset links: %v", err)
	}
	if len(links) != 1 || links[0].AssetId != asset.Id {
		t.Fatalf("unexpected dedicated task reference asset links: %#v", links)
	}
}
