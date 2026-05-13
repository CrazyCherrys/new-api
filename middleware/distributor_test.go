package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/worker_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDistributorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled
	previousMode := gin.Mode()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.ModelMapping{}); err != nil {
		t.Fatalf("failed to migrate distributor test tables: %v", err)
	}

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled
		gin.SetMode(previousMode)

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newImageGenerationRelayContext(path string, userId int, requestEndpoint string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Request.Header.Set("X-New-API-Image-Generation-Task", "true")
	if requestEndpoint != "" {
		c.Request.Header.Set("X-New-API-Image-Request-Endpoint", requestEndpoint)
	}
	c.Set("id", userId)
	return c
}

func TestGetUserCustomImageGenerationChannelUsesUserWorkerSettings(t *testing.T) {
	db := setupDistributorTestDB(t)

	user := &model.User{
		Username: "middleware-worker-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{
		WorkerApiKey:  " sk-user-key ",
		WorkerApiBase: "https://custom.example.com/v1/",
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousKeyEnabled := cfg.UserCustomKeyEnabled
	previousBaseAllowed := cfg.UserCustomBaseURLAllowed
	t.Cleanup(func() {
		cfg.UserCustomKeyEnabled = previousKeyEnabled
		cfg.UserCustomBaseURLAllowed = previousBaseAllowed
	})
	cfg.UserCustomKeyEnabled = true
	cfg.UserCustomBaseURLAllowed = true

	c := newImageGenerationRelayContext("/v1/images/generations", user.Id, "gemini")
	channel, err := getUserCustomImageGenerationChannel(c, &ModelRequest{Model: "gemini-image"})
	if err != nil {
		t.Fatalf("unexpected custom channel error: %v", err)
	}
	if channel == nil {
		t.Fatalf("expected custom image generation channel")
	}
	if channel.Type != constant.ChannelTypeGemini {
		t.Fatalf("expected Gemini channel type, got %d", channel.Type)
	}
	if channel.Key != "sk-user-key" {
		t.Fatalf("expected trimmed user API key, got %q", channel.Key)
	}
	if channel.GetBaseURL() != "https://custom.example.com/v1" {
		t.Fatalf("expected custom base URL, got %q", channel.GetBaseURL())
	}
}

func TestGetUserCustomImageGenerationChannelFallsBackToDefaultBaseWhenBaseDisabled(t *testing.T) {
	db := setupDistributorTestDB(t)

	user := &model.User{
		Username: "middleware-worker-default-base",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{
		WorkerApiKey:  "sk-user-key",
		WorkerApiBase: "https://custom.example.com/v1",
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousKeyEnabled := cfg.UserCustomKeyEnabled
	previousBaseAllowed := cfg.UserCustomBaseURLAllowed
	t.Cleanup(func() {
		cfg.UserCustomKeyEnabled = previousKeyEnabled
		cfg.UserCustomBaseURLAllowed = previousBaseAllowed
	})
	cfg.UserCustomKeyEnabled = true
	cfg.UserCustomBaseURLAllowed = false

	c := newImageGenerationRelayContext("/v1/images/generations", user.Id, "openai")
	channel, err := getUserCustomImageGenerationChannel(c, &ModelRequest{Model: "gpt-image-1"})
	if err != nil {
		t.Fatalf("unexpected custom channel error: %v", err)
	}
	if channel == nil {
		t.Fatalf("expected custom image generation channel")
	}
	if channel.Type != constant.ChannelTypeOpenAI {
		t.Fatalf("expected OpenAI channel type, got %d", channel.Type)
	}
	if channel.Key != "sk-user-key" {
		t.Fatalf("expected user API key, got %q", channel.Key)
	}
	if channel.GetBaseURL() != constant.ChannelBaseURLs[constant.ChannelTypeOpenAI] {
		t.Fatalf("expected default OpenAI base URL, got %q", channel.GetBaseURL())
	}
}

func TestGetUserCustomImageGenerationChannelRequiresUserKey(t *testing.T) {
	db := setupDistributorTestDB(t)

	user := &model.User{
		Username: "middleware-worker-no-key",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{
		WorkerApiBase: "https://custom.example.com/v1",
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cfg := worker_setting.GetWorkerSetting()
	previousKeyEnabled := cfg.UserCustomKeyEnabled
	previousBaseAllowed := cfg.UserCustomBaseURLAllowed
	t.Cleanup(func() {
		cfg.UserCustomKeyEnabled = previousKeyEnabled
		cfg.UserCustomBaseURLAllowed = previousBaseAllowed
	})
	cfg.UserCustomKeyEnabled = true
	cfg.UserCustomBaseURLAllowed = true

	c := newImageGenerationRelayContext("/v1/images/generations", user.Id, "openai")
	channel, err := getUserCustomImageGenerationChannel(c, &ModelRequest{Model: "gpt-image-1"})
	if err != nil {
		t.Fatalf("unexpected custom channel error: %v", err)
	}
	if channel != nil {
		t.Fatalf("expected no custom channel without user key, got %+v", channel)
	}
}
