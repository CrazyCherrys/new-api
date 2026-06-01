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
	if err := db.AutoMigrate(&CanvasSession{}, &CanvasMessage{}); err != nil {
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

func TestInitCanvasDBsSplitModeMigratesDedicatedTables(t *testing.T) {
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
		canvasImageMessagesTable,
		canvasVideoMessagesTable,
	} {
		if !sharedChildDB.Migrator().HasTable(tableName) {
			t.Fatalf("expected child table %s to exist", tableName)
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

func TestInitCanvasDBsDisablesCanvasOnPartialConfig(t *testing.T) {
	setupCanvasMessageStoreTestDB(t)

	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-chat")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video")

	InitCanvasDBs()

	available, reason := CanvasAvailabilityStatus()
	if available {
		t.Fatal("expected canvas to be disabled for partial config")
	}
	if !strings.Contains(reason, "must be configured together") {
		t.Fatalf("unexpected disable reason: %q", reason)
	}
}

func TestInitCanvasDBsDisablesCanvasOnOpenFailure(t *testing.T) {
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
	if available {
		t.Fatal("expected canvas to be disabled on child DB init failure")
	}
	if !strings.Contains(reason, "boom") {
		t.Fatalf("unexpected disable reason: %q", reason)
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
