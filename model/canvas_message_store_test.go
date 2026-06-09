package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCanvasMessageStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousCanvasChatDB := CANVAS_CHAT_DB
	previousCanvasImageDB := CANVAS_IMAGE_DB
	previousCanvasVideoDB := CANVAS_VIDEO_DB
	previousCanvasOpener := openCanvasMessagePostgresDBFunc
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousIsMasterNode := common.IsMasterNode

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.IsMasterNode = true
	InitCommonColumnNames()
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db
	if err := db.AutoMigrate(&CanvasSession{}, &CanvasMessage{}, &CanvasChatMessageAttachment{}); err != nil {
		t.Fatalf("failed to migrate main canvas tables: %v", err)
	}
	InitCanvasDBs()

	t.Cleanup(func() {
		_ = closeCanvasMessageDBs()
		openCanvasMessagePostgresDBFunc = previousCanvasOpener
		DB = previousDB
		LOG_DB = previousLogDB
		CANVAS_CHAT_DB = previousCanvasChatDB
		CANVAS_IMAGE_DB = previousCanvasImageDB
		CANVAS_VIDEO_DB = previousCanvasVideoDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.IsMasterNode = previousIsMasterNode
		InitCommonColumnNames()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestInitCanvasDBsCompatibilityModeUsesMainDB(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)

	InitCanvasDBs()

	if available, reason := CanvasAvailabilityStatus(); !available {
		t.Fatalf("expected canvas to be available, got reason %q", reason)
	}
	if CanvasUsesDedicatedMessageDBs() {
		t.Fatal("expected compatibility mode to use main DB")
	}

	session := &CanvasSession{UserId: 1, Mode: CanvasModeChat, Title: "chat", CreatedTime: 1, UpdatedTime: 1}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	message := &CanvasMessage{
		SessionId: session.Id,
		UserId:    1,
		Mode:      CanvasModeChat,
		Role:      CanvasMessageRoleUser,
		Prompt:    "compat prompt",
		Status:    CanvasMessageStatusSuccess,
	}
	if err := CreateCanvasMessageForMode(CanvasModeChat, message); err != nil {
		t.Fatalf("failed to create compatibility message: %v", err)
	}

	var count int64
	if err := db.Table("canvas_messages").Count(&count).Error; err != nil {
		t.Fatalf("failed to count legacy canvas_messages rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row in canvas_messages, got %d", count)
	}
}

func TestInitCanvasDBsSplitModeAutoMigratesDedicatedTables(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)

	sharedChildDSN := fmt.Sprintf("file:%s_child?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	sharedChildDB, err := gorm.Open(sqlite.Open(sharedChildDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open child sqlite db: %v", err)
	}
	defer func() {
		sqlDB, dbErr := sharedChildDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	}()

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		return sharedChildDB, nil
	}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-chat")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video")

	InitCanvasDBs()

	if available, reason := CanvasAvailabilityStatus(); !available {
		t.Fatalf("expected canvas split mode to be available, got reason %q", reason)
	}
	if !CanvasUsesDedicatedMessageDBs() {
		t.Fatal("expected dedicated canvas databases to be enabled")
	}
	for _, tableName := range []string{
		canvasChatMessagesTable,
		canvasChatMessageAttachmentsTable,
		canvasImageMessagesTable,
		canvasVideoMessagesTable,
	} {
		if !sharedChildDB.Migrator().HasTable(tableName) {
			t.Fatalf("expected child table %s to exist after canvas DB init", tableName)
		}
	}

	testCases := []struct {
		mode      string
		tableName string
	}{
		{mode: CanvasModeChat, tableName: canvasChatMessagesTable},
		{mode: CanvasModeImage, tableName: canvasImageMessagesTable},
		{mode: CanvasModeVideo, tableName: canvasVideoMessagesTable},
	}
	for index, tc := range testCases {
		message := &CanvasMessage{
			SessionId: index + 1,
			UserId:    7,
			Mode:      tc.mode,
			Role:      CanvasMessageRoleUser,
			Prompt:    tc.mode + " prompt",
			Status:    CanvasMessageStatusSuccess,
		}
		if err := CreateCanvasMessageForMode(tc.mode, message); err != nil {
			t.Fatalf("failed to create %s message: %v", tc.mode, err)
		}
		var count int64
		if err := sharedChildDB.Table(tc.tableName).Count(&count).Error; err != nil {
			t.Fatalf("failed to count %s rows: %v", tc.tableName, err)
		}
		if count != 1 {
			t.Fatalf("expected 1 row in %s, got %d", tc.tableName, count)
		}
	}

	var legacyCount int64
	if err := db.Table("canvas_messages").Count(&legacyCount).Error; err != nil {
		t.Fatalf("failed to count legacy canvas_messages rows: %v", err)
	}
	if legacyCount != 0 {
		t.Fatalf("expected 0 rows in legacy canvas_messages during split mode, got %d", legacyCount)
	}
}

func TestInitCanvasDBsMigrationFailureDoesNotBlockOtherModes(t *testing.T) {
	setupCanvasMessageStoreTestDB(t)

	workingChildDSN := fmt.Sprintf("file:%s_migration_working?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	workingChildDB, err := gorm.Open(sqlite.Open(workingChildDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open working child sqlite db: %v", err)
	}
	defer func() {
		sqlDB, dbErr := workingChildDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	}()

	brokenChildDSN := fmt.Sprintf("file:%s_migration_broken?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	brokenChildDB, err := gorm.Open(sqlite.Open(brokenChildDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open broken child sqlite db: %v", err)
	}
	sqlDB, err := brokenChildDB.DB()
	if err != nil {
		t.Fatalf("failed to get broken sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("failed to close broken sql db: %v", err)
	}

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		if envName == "CANVAS_IMAGE_SQL_DSN" {
			return brokenChildDB, nil
		}
		return workingChildDB, nil
	}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-chat")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video")

	InitCanvasDBs()

	if available, reason := CanvasAvailabilityStatus(); !available {
		t.Fatalf("expected canvas to remain available when one dedicated migration fails, got reason %q", reason)
	}
	if available, reason := CanvasModeAvailabilityStatus(CanvasModeImage); available || !strings.Contains(reason, "closed") {
		t.Fatalf("expected image mode to be unavailable after migration failure, got available=%v reason=%q", available, reason)
	}
	for _, mode := range []string{CanvasModeChat, CanvasModeVideo} {
		if available, reason := CanvasModeAvailabilityStatus(mode); !available {
			t.Fatalf("expected %s mode to remain available, got reason %q", mode, reason)
		}
	}
	for _, tableName := range []string{canvasChatMessagesTable, canvasVideoMessagesTable} {
		if !workingChildDB.Migrator().HasTable(tableName) {
			t.Fatalf("expected working child table %s to be migrated", tableName)
		}
	}
}

func TestInitCanvasDBsSplitModeReusesSharedDSNPool(t *testing.T) {
	setupCanvasMessageStoreTestDB(t)

	sharedChildDSN := fmt.Sprintf("file:%s_reuse?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	sharedChildDB, err := gorm.Open(sqlite.Open(sharedChildDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open child sqlite db: %v", err)
	}
	defer func() {
		sqlDB, dbErr := sharedChildDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	}()

	openCount := 0
	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		openCount++
		return sharedChildDB, nil
	}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-shared")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-shared")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-shared")

	InitCanvasDBs()

	if openCount != 1 {
		t.Fatalf("expected one shared dedicated DB open for identical DSNs, got %d", openCount)
	}
	if CANVAS_CHAT_DB == nil || CANVAS_IMAGE_DB == nil || CANVAS_VIDEO_DB == nil {
		t.Fatal("expected dedicated canvas DB handles to be initialized")
	}
	if CANVAS_CHAT_DB != CANVAS_IMAGE_DB || CANVAS_CHAT_DB != CANVAS_VIDEO_DB {
		t.Fatal("expected identical dedicated DSNs to reuse the same gorm DB handle")
	}
}

func TestInitCanvasDBsKeepsOtherModesAvailableOnOpenFailure(t *testing.T) {
	setupCanvasMessageStoreTestDB(t)

	childDSN := fmt.Sprintf("file:%s_open_failure?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
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
		if envName == "CANVAS_IMAGE_SQL_DSN" {
			return nil, fmt.Errorf("boom")
		}
		return childDB, nil
	}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-chat")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video")

	InitCanvasDBs()

	available, reason := CanvasAvailabilityStatus()
	if !available {
		t.Fatalf("expected canvas to remain available when only one mode fails, got reason %q", reason)
	}
	if modeAvailable, _ := CanvasModeAvailabilityStatus(CanvasModeImage); modeAvailable {
		t.Fatal("expected image mode to be unavailable after dedicated DB open failure")
	}
	if modeAvailable, reason := CanvasModeAvailabilityStatus(CanvasModeChat); !modeAvailable {
		t.Fatalf("expected chat mode to remain available, got reason %q", reason)
	}
	if modeAvailable, reason := CanvasModeAvailabilityStatus(CanvasModeVideo); !modeAvailable {
		t.Fatalf("expected video mode to remain available, got reason %q", reason)
	}
}

func TestCanvasModeOpenFailureDoesNotBlockOtherModes(t *testing.T) {
	testCases := []struct {
		name         string
		failedMode   string
		workingModes []string
		envName      string
	}{
		{name: "chat failure leaves image video available", failedMode: CanvasModeChat, workingModes: []string{CanvasModeImage, CanvasModeVideo}, envName: "CANVAS_CHAT_SQL_DSN"},
		{name: "image failure leaves chat video available", failedMode: CanvasModeImage, workingModes: []string{CanvasModeChat, CanvasModeVideo}, envName: "CANVAS_IMAGE_SQL_DSN"},
		{name: "video failure leaves chat image available", failedMode: CanvasModeVideo, workingModes: []string{CanvasModeChat, CanvasModeImage}, envName: "CANVAS_VIDEO_SQL_DSN"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupCanvasMessageStoreTestDB(t)
			openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
				if envName == tc.envName {
					return nil, fmt.Errorf("boom")
				}
				return nil, fmt.Errorf("unexpected dedicated open for %s", envName)
			}
			t.Setenv(tc.envName, "postgres://broken")

			InitCanvasDBs()

			if available, reason := CanvasAvailabilityStatus(); !available {
				t.Fatalf("expected overall canvas availability to stay up, got reason %q", reason)
			}
			if available, reason := CanvasModeAvailabilityStatus(tc.failedMode); available || !strings.Contains(reason, "boom") {
				t.Fatalf("expected %s mode to be unavailable with boom, got available=%v reason=%q", tc.failedMode, available, reason)
			}
			for index, mode := range tc.workingModes {
				if available, reason := CanvasModeAvailabilityStatus(mode); !available {
					t.Fatalf("expected %s mode to remain available, got reason %q", mode, reason)
				}
				message := &CanvasMessage{
					SessionId: index + 1,
					UserId:    99,
					Mode:      mode,
					Role:      CanvasMessageRoleUser,
					Prompt:    mode + " prompt",
					Status:    CanvasMessageStatusSuccess,
				}
				if err := CreateCanvasMessageForMode(mode, message); err != nil {
					t.Fatalf("expected %s mode to keep writing, got %v", mode, err)
				}
			}

			var count int64
			if err := db.Table("canvas_messages").Count(&count).Error; err != nil {
				t.Fatalf("failed to count compatibility rows: %v", err)
			}
			if count != int64(len(tc.workingModes)) {
				t.Fatalf("expected %d compatibility rows, got %d", len(tc.workingModes), count)
			}
		})
	}
}

func TestOpenCanvasMessagePostgresDBRejectsNonPostgresDSN(t *testing.T) {
	setupCanvasMessageStoreTestDB(t)

	if _, err := openCanvasMessagePostgresDB("CANVAS_CHAT_SQL_DSN", "local"); err == nil {
		t.Fatal("expected non-postgres canvas DSN to be rejected")
	}
	if _, err := openCanvasMessagePostgresDB("CANVAS_CHAT_SQL_DSN", "mysql://example"); err == nil {
		t.Fatal("expected mysql canvas DSN to be rejected")
	}
}

func TestMigrateDBCreatesCanvasChatMessageAttachmentTable(t *testing.T) {
	previousDB := DB
	previousLogDB := LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	InitCommonColumnNames()

	dsn := fmt.Sprintf("file:%s_migrate_db?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		InitCommonColumnNames()
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	if db.Migrator().HasTable(canvasChatMessageAttachmentsTable) {
		t.Fatalf("expected %s table to be absent before migrateDB", canvasChatMessageAttachmentsTable)
	}
	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB failed: %v", err)
	}
	if !db.Migrator().HasTable(canvasChatMessageAttachmentsTable) {
		t.Fatalf("expected migrateDB to create %s", canvasChatMessageAttachmentsTable)
	}
}
