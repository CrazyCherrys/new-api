package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserSettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestUserUpdateReloadsWorkerSettingsAfterSave(t *testing.T) {
	db := setupUserSettingTestDB(t)

	user := &User{
		Username: "worker-cache-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{
		WorkerApiKey:  "old-key",
		WorkerApiBase: "https://old.example.com/v1",
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	user.SetSetting(dto.UserSetting{
		WorkerApiKey:  "new-key",
		WorkerApiBase: "https://new.example.com/v1",
	})
	if err := user.Update(false); err != nil {
		t.Fatalf("failed to update user settings: %v", err)
	}

	updatedSetting := user.GetSetting()
	if updatedSetting.WorkerApiKey != "new-key" {
		t.Fatalf("expected worker API key to be refreshed on user after update, got %q", updatedSetting.WorkerApiKey)
	}
	if updatedSetting.WorkerApiBase != "https://new.example.com/v1" {
		t.Fatalf("expected worker API base to be refreshed on user after update, got %q", updatedSetting.WorkerApiBase)
	}

	var reloaded User
	if err := db.First(&reloaded, user.Id).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	reloadedSetting := reloaded.GetSetting()
	if reloadedSetting.WorkerApiKey != "new-key" {
		t.Fatalf("expected stored worker API key to be updated, got %q", reloadedSetting.WorkerApiKey)
	}
	if reloadedSetting.WorkerApiBase != "https://new.example.com/v1" {
		t.Fatalf("expected stored worker API base to be updated, got %q", reloadedSetting.WorkerApiBase)
	}
}
