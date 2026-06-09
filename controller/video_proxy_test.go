package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupVideoProxyControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	model.InitCommonColumnNames()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("failed to migrate channel table: %v", err)
	}

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		model.InitCommonColumnNames()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func withVideoProxyTaskLookups(
	t *testing.T,
	mainLookup func(int, string) (*model.Task, bool, error),
	canvasLookup func(int, string) (*model.Task, bool, error),
) {
	t.Helper()

	previousMainLookup := getVideoProxyTaskByTaskID
	previousCanvasLookup := getCanvasVideoProxyTaskByTaskID
	if mainLookup != nil {
		getVideoProxyTaskByTaskID = mainLookup
	}
	if canvasLookup != nil {
		getCanvasVideoProxyTaskByTaskID = canvasLookup
	}
	t.Cleanup(func() {
		getVideoProxyTaskByTaskID = previousMainLookup
		getCanvasVideoProxyTaskByTaskID = previousCanvasLookup
	})
}

func newVideoProxyTestContext(method string, path string, taskID string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, nil)
	context.Params = gin.Params{{Key: "task_id", Value: taskID}}
	context.Set("id", userID)
	return context, recorder
}

func TestVideoProxyDoesNotFallbackToCanvasLookup(t *testing.T) {
	withVideoProxyTaskLookups(
		t,
		func(userID int, taskID string) (*model.Task, bool, error) {
			if userID != 7 || taskID != "task_missing_main" {
				t.Fatalf("unexpected main lookup args user=%d task=%s", userID, taskID)
			}
			return nil, false, nil
		},
		func(userID int, taskID string) (*model.Task, bool, error) {
			t.Fatalf("ordinary video proxy should not query canvas video task store")
			return nil, false, nil
		},
	)

	context, recorder := newVideoProxyTestContext(http.MethodGet, "/v1/videos/task_missing_main/content", "task_missing_main", 7)
	VideoProxy(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected ordinary proxy to return 404 without canvas fallback, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCanvasVideoProxyUsesCanvasLookupAndStreamsDataURL(t *testing.T) {
	db := setupVideoProxyControllerTestDB(t)
	channel := &model.Channel{
		Id:     7001,
		Type:   constant.ChannelTypeCustom,
		Key:    "channel-key",
		Status: common.ChannelStatusEnabled,
		Name:   "custom-video",
		Group:  "default",
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to seed channel: %v", err)
	}

	withVideoProxyTaskLookups(
		t,
		func(userID int, taskID string) (*model.Task, bool, error) {
			t.Fatalf("canvas video proxy should not query ordinary task store")
			return nil, false, nil
		},
		func(userID int, taskID string) (*model.Task, bool, error) {
			if userID != 7 || taskID != "task_canvas_proxy" {
				t.Fatalf("unexpected canvas lookup args user=%d task=%s", userID, taskID)
			}
			return &model.Task{
				UserId:    userID,
				TaskID:    taskID,
				ChannelId: channel.Id,
				Status:    model.TaskStatusSuccess,
				PrivateData: model.TaskPrivateData{
					ResultURL: "data:video/mp4;base64,Y2FudmFzLXZpZGVv",
				},
			}, true, nil
		},
	)

	context, recorder := newVideoProxyTestContext(http.MethodGet, "/api/canvas/videos/task_canvas_proxy/content", "task_canvas_proxy", 7)
	CanvasVideoProxy(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected canvas proxy to stream data URL, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); body != "canvas-video" {
		t.Fatalf("expected decoded video bytes, got %q", body)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "video/mp4" {
		t.Fatalf("expected video/mp4 content type, got %q", contentType)
	}
}

func TestCanvasVideoProxyReturnsUnavailableForCanvasStoreError(t *testing.T) {
	withVideoProxyTaskLookups(
		t,
		func(userID int, taskID string) (*model.Task, bool, error) {
			t.Fatalf("canvas video proxy should not query ordinary task store")
			return nil, false, nil
		},
		func(userID int, taskID string) (*model.Task, bool, error) {
			return nil, false, &model.CanvasModeUnavailableError{
				Mode:   model.CanvasModeVideo,
				Reason: "child database unavailable",
			}
		},
	)

	context, recorder := newVideoProxyTestContext(http.MethodGet, "/api/canvas/videos/task_canvas_down/content", "task_canvas_down", 7)
	CanvasVideoProxy(context)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unavailable canvas video store, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "child database unavailable") {
		t.Fatalf("expected unavailable reason in response, got %s", recorder.Body.String())
	}
}

func TestCanvasVideoProxyReturnsServerErrorForUnexpectedLookupError(t *testing.T) {
	withVideoProxyTaskLookups(
		t,
		nil,
		func(userID int, taskID string) (*model.Task, bool, error) {
			return nil, false, errors.New("query failed")
		},
	)

	context, recorder := newVideoProxyTestContext(http.MethodGet, "/api/canvas/videos/task_canvas_error/content", "task_canvas_error", 7)
	CanvasVideoProxy(context)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unexpected lookup error, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
