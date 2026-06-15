package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCanvasFullSplitTestDBs(t *testing.T) (*gorm.DB, map[string]*gorm.DB) {
	t.Helper()

	mainDB := setupCanvasMessageStoreTestDB(t)
	childDBs := make(map[string]*gorm.DB, 3)
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		childDSN := fmt.Sprintf("file:%s_%s_full_split?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), mode)
		childDB, err := gorm.Open(sqlite.Open(childDSN), &gorm.Config{})
		if err != nil {
			t.Fatalf("failed to open %s child sqlite db: %v", mode, err)
		}
		childDBs[mode] = childDB
		t.Cleanup(func(db *gorm.DB) func() {
			return func() {
				sqlDB, dbErr := db.DB()
				if dbErr == nil {
					_ = sqlDB.Close()
				}
			}
		}(childDB))
	}

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		switch envName {
		case "CANVAS_CHAT_SQL_DSN":
			return childDBs[CanvasModeChat], nil
		case "CANVAS_IMAGE_SQL_DSN":
			return childDBs[CanvasModeImage], nil
		case "CANVAS_VIDEO_SQL_DSN":
			return childDBs[CanvasModeVideo], nil
		default:
			t.Fatalf("unexpected dedicated DB open request: %s", envName)
			return nil, nil
		}
	}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-chat-full-split")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-full-split")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video-full-split")

	InitCanvasDBs()
	return mainDB, childDBs
}

func TestCanvasSessionsSplitModeAutoMigratesAndRoutesByMode(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)

	childDBs := make(map[string]*gorm.DB, 3)
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		childDSN := fmt.Sprintf("file:%s_%s_sessions?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), mode)
		childDB, err := gorm.Open(sqlite.Open(childDSN), &gorm.Config{})
		if err != nil {
			t.Fatalf("failed to open %s child sqlite db: %v", mode, err)
		}
		childDBs[mode] = childDB
		defer func(mode string, db *gorm.DB) {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				_ = sqlDB.Close()
			}
		}(mode, childDB)
	}

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		switch envName {
		case "CANVAS_CHAT_SQL_DSN":
			return childDBs[CanvasModeChat], nil
		case "CANVAS_IMAGE_SQL_DSN":
			return childDBs[CanvasModeImage], nil
		case "CANVAS_VIDEO_SQL_DSN":
			return childDBs[CanvasModeVideo], nil
		default:
			t.Fatalf("unexpected dedicated DB open request: %s", envName)
			return nil, nil
		}
	}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://canvas-chat-sessions")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-sessions")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://canvas-video-sessions")

	InitCanvasDBs()

	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		if !childDBs[mode].Migrator().HasTable("canvas_sessions") {
			t.Fatalf("expected %s dedicated session table after canvas DB init", mode)
		}
	}

	sessions := []*CanvasSession{
		{UserId: 7, Mode: CanvasModeChat, Title: "chat session"},
		{UserId: 7, Mode: CanvasModeImage, Title: "image session"},
		{UserId: 7, Mode: CanvasModeVideo, Title: "video session"},
	}
	for _, session := range sessions {
		if err := CreateCanvasSession(session); err != nil {
			t.Fatalf("failed to create %s session: %v", session.Mode, err)
		}
		if !strings.HasPrefix(session.PublicId, "cs_"+session.Mode+"_") {
			t.Fatalf("expected mode-prefixed public_id for %s session, got %q", session.Mode, session.PublicId)
		}
	}

	for _, session := range sessions {
		var childCount int64
		if err := childDBs[session.Mode].Model(&CanvasSession{}).Where("user_id = ?", session.UserId).Count(&childCount).Error; err != nil {
			t.Fatalf("failed to count %s dedicated sessions: %v", session.Mode, err)
		}
		if childCount != 1 {
			t.Fatalf("expected 1 dedicated %s session row, got %d", session.Mode, childCount)
		}
		reloaded, err := GetCanvasSessionByIdentifier(session.UserId, session.PublicId)
		if err != nil {
			t.Fatalf("failed to reload %s session by public_id: %v", session.Mode, err)
		}
		if reloaded == nil || reloaded.Mode != session.Mode || reloaded.PublicId != session.PublicId {
			t.Fatalf("unexpected reloaded %s session: %#v", session.Mode, reloaded)
		}
	}

	var mainCount int64
	if err := db.Model(&CanvasSession{}).Where("user_id = ?", 7).Count(&mainCount).Error; err != nil {
		t.Fatalf("failed to count main-db sessions: %v", err)
	}
	if mainCount != 0 {
		t.Fatalf("expected 0 main-db sessions in full split mode, got %d", mainCount)
	}

	items, hasMore, err := ListCanvasSessions(7, "", 20, 0)
	if err != nil {
		t.Fatalf("failed to list split sessions: %v", err)
	}
	if hasMore {
		t.Fatalf("did not expect hasMore for split session list")
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 split sessions, got %#v", items)
	}
}

func TestCanvasSessionListAllModesSkipsUnavailableDedicatedMode(t *testing.T) {
	setupCanvasMessageStoreTestDB(t)

	openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
		if envName == "CANVAS_IMAGE_SQL_DSN" {
			return nil, fmt.Errorf("boom")
		}
		t.Fatalf("unexpected dedicated DB open request: %s", envName)
		return nil, nil
	}
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "postgres://canvas-image-broken")

	InitCanvasDBs()

	chatSession := &CanvasSession{UserId: 9, Mode: CanvasModeChat, Title: "chat"}
	if err := CreateCanvasSession(chatSession); err != nil {
		t.Fatalf("failed to create chat compatibility session: %v", err)
	}
	videoSession := &CanvasSession{UserId: 9, Mode: CanvasModeVideo, Title: "video"}
	if err := CreateCanvasSession(videoSession); err != nil {
		t.Fatalf("failed to create video compatibility session: %v", err)
	}
	if err := CreateCanvasSession(&CanvasSession{UserId: 9, Mode: CanvasModeImage, Title: "image"}); err == nil {
		t.Fatal("expected image session create to fail when image mode DB is unavailable")
	}

	items, hasMore, err := ListCanvasSessions(9, "", 20, 0)
	if err != nil {
		t.Fatalf("failed to list sessions while one mode unavailable: %v", err)
	}
	if hasMore {
		t.Fatalf("did not expect hasMore in compatibility list")
	}
	if len(items) != 2 {
		t.Fatalf("expected only available-mode sessions, got %#v", items)
	}
	for _, item := range items {
		if item.Mode == CanvasModeImage {
			t.Fatalf("did not expect unavailable image mode session in list: %#v", items)
		}
	}
}

func TestCanvasSessionModeFailureDoesNotBlockOtherModes(t *testing.T) {
	testCases := []struct {
		name         string
		failedMode   string
		envName      string
		workingModes []string
	}{
		{name: "chat failure leaves image and video sessions available", failedMode: CanvasModeChat, envName: "CANVAS_CHAT_SQL_DSN", workingModes: []string{CanvasModeImage, CanvasModeVideo}},
		{name: "image failure leaves chat and video sessions available", failedMode: CanvasModeImage, envName: "CANVAS_IMAGE_SQL_DSN", workingModes: []string{CanvasModeChat, CanvasModeVideo}},
		{name: "video failure leaves chat and image sessions available", failedMode: CanvasModeVideo, envName: "CANVAS_VIDEO_SQL_DSN", workingModes: []string{CanvasModeChat, CanvasModeImage}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupCanvasMessageStoreTestDB(t)

			openCanvasMessagePostgresDBFunc = func(envName string, dsn string) (*gorm.DB, error) {
				if envName == tc.envName {
					return nil, fmt.Errorf("boom")
				}
				t.Fatalf("unexpected dedicated DB open request: %s", envName)
				return nil, nil
			}
			t.Setenv(tc.envName, "postgres://broken")

			InitCanvasDBs()

			if available, reason := CanvasAvailabilityStatus(); !available {
				t.Fatalf("expected overall canvas to stay available, got reason %q", reason)
			}
			if available, reason := CanvasModeAvailabilityStatus(tc.failedMode); available || !strings.Contains(reason, "boom") {
				t.Fatalf("expected failed mode %s to be unavailable with boom, got available=%v reason=%q", tc.failedMode, available, reason)
			}

			for _, mode := range tc.workingModes {
				session := &CanvasSession{UserId: 88, Mode: mode, Title: mode + " session"}
				if err := CreateCanvasSession(session); err != nil {
					t.Fatalf("expected %s session create to work, got %v", mode, err)
				}
			}

			if err := CreateCanvasSession(&CanvasSession{UserId: 88, Mode: tc.failedMode, Title: "broken"}); err == nil {
				t.Fatalf("expected %s session create to fail", tc.failedMode)
			}

			items, hasMore, err := ListCanvasSessions(88, "", 20, 0)
			if err != nil {
				t.Fatalf("failed to list sessions with %s down: %v", tc.failedMode, err)
			}
			if hasMore {
				t.Fatalf("did not expect hasMore in failure-isolation session list")
			}
			if len(items) != len(tc.workingModes) {
				t.Fatalf("expected %d working sessions, got %#v", len(tc.workingModes), items)
			}
			for _, item := range items {
				if item.Mode == tc.failedMode {
					t.Fatalf("did not expect failed mode %s in session list: %#v", tc.failedMode, items)
				}
			}
		})
	}
}

func TestCanvasSessionResolvedWritesUseSessionModeWhenNumericIDsCollide(t *testing.T) {
	_, childDBs := setupCanvasFullSplitTestDBs(t)

	chatSession := &CanvasSession{UserId: 7, Mode: CanvasModeChat, Title: "chat", UpdatedTime: 10}
	imageSession := &CanvasSession{UserId: 7, Mode: CanvasModeImage, Title: "image", UpdatedTime: 20}
	videoSession := &CanvasSession{UserId: 7, Mode: CanvasModeVideo, Title: "video", UpdatedTime: 30}
	for _, session := range []*CanvasSession{chatSession, imageSession, videoSession} {
		if err := CreateCanvasSession(session); err != nil {
			t.Fatalf("failed to create %s session: %v", session.Mode, err)
		}
	}
	if chatSession.Id != imageSession.Id || chatSession.Id != videoSession.Id {
		t.Fatalf("expected child DB session ids to collide, got chat=%d image=%d video=%d", chatSession.Id, imageSession.Id, videoSession.Id)
	}

	if err := UpdateCanvasSessionFieldsWithSession(7, chatSession, map[string]interface{}{"title": "chat renamed"}); err != nil {
		t.Fatalf("failed to update resolved chat session: %v", err)
	}
	if err := SoftDeleteCanvasSessionWithSession(7, videoSession, 99); err != nil {
		t.Fatalf("failed to soft delete resolved video session: %v", err)
	}

	var reloadedChat CanvasSession
	if err := childDBs[CanvasModeChat].First(&reloadedChat, chatSession.Id).Error; err != nil {
		t.Fatalf("failed to reload chat session: %v", err)
	}
	if reloadedChat.Title != "chat renamed" || reloadedChat.DeletedTime != 0 {
		t.Fatalf("chat session was not updated correctly: %#v", reloadedChat)
	}
	var reloadedImage CanvasSession
	if err := childDBs[CanvasModeImage].First(&reloadedImage, imageSession.Id).Error; err != nil {
		t.Fatalf("failed to reload image session: %v", err)
	}
	if reloadedImage.Title != "image" || reloadedImage.DeletedTime != 0 {
		t.Fatalf("image session should not be affected by chat/video writes: %#v", reloadedImage)
	}
	var reloadedVideo CanvasSession
	if err := childDBs[CanvasModeVideo].First(&reloadedVideo, videoSession.Id).Error; err != nil {
		t.Fatalf("failed to reload video session: %v", err)
	}
	if reloadedVideo.Title != "video" || reloadedVideo.DeletedTime != 99 {
		t.Fatalf("video session was not soft deleted correctly: %#v", reloadedVideo)
	}
}

func TestListCanvasSessionsAllModesUsesWindowedStablePaginationInSplitDB(t *testing.T) {
	setupCanvasFullSplitTestDBs(t)

	userID := 17
	modes := []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo}
	for _, seed := range []struct {
		title   string
		pinned  bool
		updated int64
	}{
		{title: "pinned", pinned: true, updated: 500},
		{title: "recent", updated: 400},
		{title: "older", updated: 300},
	} {
		for _, mode := range modes {
			session := &CanvasSession{
				UserId:      userID,
				PublicId:    fmt.Sprintf("cs_%s_%s", mode, seed.title),
				Mode:        mode,
				Title:       seed.title + "-" + mode,
				Pinned:      seed.pinned,
				CreatedTime: seed.updated,
				UpdatedTime: seed.updated,
			}
			if err := CreateCanvasSession(session); err != nil {
				t.Fatalf("failed to create %s %s session: %v", seed.title, mode, err)
			}
		}
	}

	items, hasMore, err := ListCanvasSessions(userID, "", 4, 2)
	if err != nil {
		t.Fatalf("failed to list all-mode split sessions: %v", err)
	}
	if !hasMore {
		t.Fatal("expected hasMore for middle all-mode page")
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %#v", items)
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.Title)
	}
	expected := []string{
		"pinned-video",
		"recent-chat",
		"recent-image",
		"recent-video",
	}
	if fmt.Sprint(got) != fmt.Sprint(expected) {
		t.Fatalf("unexpected stable all-mode page order, got %v want %v", got, expected)
	}

	tail, tailHasMore, err := ListCanvasSessions(userID, "", 3, 6)
	if err != nil {
		t.Fatalf("failed to list all-mode split tail page: %v", err)
	}
	if tailHasMore {
		t.Fatal("did not expect hasMore on tail page")
	}
	tailGot := make([]string, 0, len(tail))
	for _, item := range tail {
		tailGot = append(tailGot, item.Title)
	}
	tailExpected := []string{
		"older-chat",
		"older-image",
		"older-video",
	}
	if fmt.Sprint(tailGot) != fmt.Sprint(tailExpected) {
		t.Fatalf("unexpected stable tail page order, got %v want %v", tailGot, tailExpected)
	}
}

func TestDeleteCanvasSessionDataWithSessionDeletesTasksFromDedicatedDomainDB(t *testing.T) {
	mainDB, childDBs := setupCanvasFullSplitTestDBs(t)
	if err := mainDB.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}

	imageSession := &CanvasSession{UserId: 11, Mode: CanvasModeImage, Title: "image delete"}
	if err := CreateCanvasSession(imageSession); err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}
	imageTask := &ImageGenerationTask{
		UserId:          11,
		ModelId:         "image-model",
		Prompt:          "image",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "/image.png",
	}
	if err := imageTask.Insert(); err != nil {
		t.Fatalf("failed to create image task: %v", err)
	}
	imageMessage := &CanvasMessage{
		SessionId: imageSession.Id,
		UserId:    11,
		Mode:      CanvasModeImage,
		Role:      CanvasMessageRoleAssistant,
		Status:    ImageTaskStatusSuccess,
		TaskId:    fmt.Sprintf("%d", imageTask.Id),
		TaskType:  CanvasTaskTypeImage,
	}
	if err := CreateCanvasMessageForMode(CanvasModeImage, imageMessage); err != nil {
		t.Fatalf("failed to create image message: %v", err)
	}
	imageJobs := []*CanvasAssetCleanupJob{{
		UserId:       11,
		TaskType:     CanvasTaskTypeImage,
		TaskRecordId: fmt.Sprintf("%d", imageTask.Id),
		Payload:      "{}",
	}}
	if err := DeleteCanvasSessionDataWithSession(11, imageSession, []int{imageMessage.Id}, []int{imageTask.Id}, nil, imageJobs, 100); err != nil {
		t.Fatalf("failed to delete image session data: %v", err)
	}
	var imageTaskCount int64
	if err := childDBs[CanvasModeImage].Model(&ImageGenerationTask{}).Where("id = ?", imageTask.Id).Count(&imageTaskCount).Error; err != nil {
		t.Fatalf("failed to count dedicated image tasks: %v", err)
	}
	if imageTaskCount != 0 {
		t.Fatalf("expected image task to be deleted from image DB, got %d", imageTaskCount)
	}

	videoSession := &CanvasSession{UserId: 11, Mode: CanvasModeVideo, Title: "video delete"}
	if err := CreateCanvasSession(videoSession); err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}
	videoTask := &Task{
		UserId:     11,
		TaskID:     "video-delete",
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusSuccess,
		SubmitTime: 1,
	}
	if err := videoTask.InsertCanvasVideo(); err != nil {
		t.Fatalf("failed to create video task: %v", err)
	}
	videoMessage := &CanvasMessage{
		SessionId: videoSession.Id,
		UserId:    11,
		Mode:      CanvasModeVideo,
		Role:      CanvasMessageRoleAssistant,
		Status:    string(TaskStatusSuccess),
		TaskId:    fmt.Sprintf("%d", videoTask.ID),
		TaskType:  CanvasTaskTypeVideo,
	}
	if err := CreateCanvasMessageForMode(CanvasModeVideo, videoMessage); err != nil {
		t.Fatalf("failed to create video message: %v", err)
	}
	videoJobs := []*CanvasAssetCleanupJob{{
		UserId:       11,
		TaskType:     CanvasTaskTypeVideo,
		TaskRecordId: fmt.Sprintf("%d", videoTask.ID),
		Payload:      "{}",
	}}
	if err := DeleteCanvasSessionDataWithSession(11, videoSession, []int{videoMessage.Id}, nil, []int64{videoTask.ID}, videoJobs, 101); err != nil {
		t.Fatalf("failed to delete video session data: %v", err)
	}
	var videoTaskCount int64
	if err := childDBs[CanvasModeVideo].Model(&Task{}).Where("id = ?", videoTask.ID).Count(&videoTaskCount).Error; err != nil {
		t.Fatalf("failed to count dedicated video tasks: %v", err)
	}
	if videoTaskCount != 0 {
		t.Fatalf("expected video task to be deleted from video DB, got %d", videoTaskCount)
	}
	var mainTaskCount int64
	if err := mainDB.Model(&Task{}).Count(&mainTaskCount).Error; err != nil {
		t.Fatalf("failed to count main tasks: %v", err)
	}
	if mainTaskCount != 0 {
		t.Fatalf("expected main task table to remain untouched, got %d", mainTaskCount)
	}
}

func TestDeleteCanvasChatSessionDataWithSessionDeletesAttachmentsInDedicatedChatDB(t *testing.T) {
	_, childDBs := setupCanvasFullSplitTestDBs(t)

	chatSession := &CanvasSession{UserId: 12, Mode: CanvasModeChat, Title: "chat delete"}
	if err := CreateCanvasSession(chatSession); err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	userMessage := &CanvasMessage{
		SessionId: chatSession.Id,
		UserId:    12,
		Mode:      CanvasModeChat,
		Role:      CanvasMessageRoleUser,
		Prompt:    "chat",
		Metadata:  `{"attachments":[{"kind":"image","name":"ref.png","mime_type":"image/png","data":"data:image/png;base64,AA=="}]}`,
		Status:    CanvasMessageStatusSuccess,
	}
	if err := CreateCanvasMessageForMode(CanvasModeChat, userMessage); err != nil {
		t.Fatalf("failed to create chat user message: %v", err)
	}
	assistantMessage := &CanvasMessage{
		SessionId: chatSession.Id,
		UserId:    12,
		Mode:      CanvasModeChat,
		Role:      CanvasMessageRoleAssistant,
		Prompt:    "reply",
		Status:    CanvasMessageStatusSuccess,
	}
	if err := CreateCanvasMessageForMode(CanvasModeChat, assistantMessage); err != nil {
		t.Fatalf("failed to create chat assistant message: %v", err)
	}

	var beforeCount int64
	if err := childDBs[CanvasModeChat].Model(&CanvasChatMessageAttachment{}).
		Where("user_id = ? AND session_id = ?", 12, chatSession.Id).
		Count(&beforeCount).Error; err != nil {
		t.Fatalf("failed to count dedicated chat attachments before delete: %v", err)
	}
	if beforeCount != 1 {
		t.Fatalf("expected 1 dedicated chat attachment before delete, got %d", beforeCount)
	}

	if err := DeleteCanvasSessionDataWithSession(12, chatSession, []int{userMessage.Id, assistantMessage.Id}, nil, nil, nil, 120); err != nil {
		t.Fatalf("failed to delete chat session data: %v", err)
	}

	var afterCount int64
	if err := childDBs[CanvasModeChat].Model(&CanvasChatMessageAttachment{}).
		Where("user_id = ? AND session_id = ?", 12, chatSession.Id).
		Count(&afterCount).Error; err != nil {
		t.Fatalf("failed to count dedicated chat attachments after delete: %v", err)
	}
	if afterCount != 0 {
		t.Fatalf("expected dedicated chat attachments to be deleted, got %d", afterCount)
	}

	reloadedSession, err := GetCanvasSessionByID(12, chatSession.Id)
	if err != nil {
		t.Fatalf("failed to reload deleted chat session: %v", err)
	}
	if reloadedSession != nil {
		t.Fatalf("expected chat session to be deleted, got %#v", reloadedSession)
	}
	messageRefs, err := ListCanvasSessionMessageTaskRefsForMode(CanvasModeChat, 12, chatSession.Id)
	if err != nil {
		t.Fatalf("failed to list chat message refs after delete: %v", err)
	}
	if len(messageRefs) != 0 {
		t.Fatalf("expected dedicated chat messages to be deleted, got %d", len(messageRefs))
	}
	var messageCount int64
	if err := childDBs[CanvasModeChat].Table(canvasChatMessagesTable).Where("user_id = ? AND session_id = ?", 12, chatSession.Id).Count(&messageCount).Error; err != nil {
		t.Fatalf("failed to count dedicated chat messages after delete: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("expected dedicated chat message rows to be deleted, got %d", messageCount)
	}
}

func TestDeleteCanvasChatSessionDataWithSessionRollsBackAttachmentDeleteOnSessionFailureInDedicatedChatDB(t *testing.T) {
	_, childDBs := setupCanvasFullSplitTestDBs(t)

	chatSession := &CanvasSession{UserId: 13, Mode: CanvasModeChat, Title: "chat rollback"}
	if err := CreateCanvasSession(chatSession); err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	userMessage := &CanvasMessage{
		SessionId: chatSession.Id,
		UserId:    13,
		Mode:      CanvasModeChat,
		Role:      CanvasMessageRoleUser,
		Prompt:    "chat",
		Metadata:  `{"attachments":[{"kind":"image","name":"ref.png","mime_type":"image/png","data":"data:image/png;base64,AA=="}]}`,
		Status:    CanvasMessageStatusSuccess,
	}
	if err := CreateCanvasMessageForMode(CanvasModeChat, userMessage); err != nil {
		t.Fatalf("failed to create chat user message: %v", err)
	}
	assistantMessage := &CanvasMessage{
		SessionId: chatSession.Id,
		UserId:    13,
		Mode:      CanvasModeChat,
		Role:      CanvasMessageRoleAssistant,
		Prompt:    "reply",
		Status:    CanvasMessageStatusSuccess,
	}
	if err := CreateCanvasMessageForMode(CanvasModeChat, assistantMessage); err != nil {
		t.Fatalf("failed to create chat assistant message: %v", err)
	}

	deleteCallbackName := "fail_dedicated_canvas_chat_session_delete"
	if err := childDBs[CanvasModeChat].Callback().Delete().Before("gorm:delete").Register(deleteCallbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "canvas_sessions" {
			tx.AddError(fmt.Errorf("boom"))
		}
	}); err != nil {
		t.Fatalf("failed to register dedicated delete callback: %v", err)
	}
	defer func() {
		_ = childDBs[CanvasModeChat].Callback().Delete().Remove(deleteCallbackName)
	}()

	if err := DeleteCanvasSessionDataWithSession(13, chatSession, []int{userMessage.Id, assistantMessage.Id}, nil, nil, nil, 121); err == nil {
		t.Fatal("expected dedicated chat delete to fail when session delete fails")
	}

	reloadedSession, err := GetCanvasSessionByID(13, chatSession.Id)
	if err != nil {
		t.Fatalf("failed to reload chat session after rollback: %v", err)
	}
	if reloadedSession == nil || reloadedSession.DeletedTime != 0 {
		t.Fatalf("expected chat session to remain undeleted after rollback, got %#v", reloadedSession)
	}

	messageRefs, err := ListCanvasSessionMessageTaskRefsForMode(CanvasModeChat, 13, chatSession.Id)
	if err != nil {
		t.Fatalf("failed to list chat message refs after rollback: %v", err)
	}
	if len(messageRefs) != 2 {
		t.Fatalf("expected dedicated chat messages to remain visible after rollback, got %d", len(messageRefs))
	}

	var attachmentCount int64
	if err := childDBs[CanvasModeChat].Model(&CanvasChatMessageAttachment{}).
		Where("user_id = ? AND session_id = ?", 13, chatSession.Id).
		Count(&attachmentCount).Error; err != nil {
		t.Fatalf("failed to count dedicated chat attachments after rollback: %v", err)
	}
	if attachmentCount != 1 {
		t.Fatalf("expected dedicated chat attachment delete to roll back, got %d", attachmentCount)
	}
}

func TestBackfillCanvasDedicatedDBsCopiesMainRowsIdempotently(t *testing.T) {
	mainDB, childDBs := setupCanvasFullSplitTestDBs(t)
	if err := mainDB.AutoMigrate(
		&Task{},
		&ImageGenerationTask{},
		&ImageGenerationReferenceAsset{},
		&ImageGenerationTaskReferenceAsset{},
		&CanvasAssetCleanupJob{},
	); err != nil {
		t.Fatalf("failed to migrate main backfill tables: %v", err)
	}

	chatSession := &CanvasSession{Id: 101, UserId: 21, Mode: CanvasModeChat, Title: "legacy chat", CreatedTime: 1, UpdatedTime: 1}
	imageSession := &CanvasSession{Id: 102, UserId: 21, Mode: CanvasModeImage, Title: "legacy image", CreatedTime: 2, UpdatedTime: 2}
	videoSession := &CanvasSession{Id: 103, UserId: 21, Mode: CanvasModeVideo, Title: "legacy video", CreatedTime: 3, UpdatedTime: 3}
	if err := mainDB.Create([]*CanvasSession{chatSession, imageSession, videoSession}).Error; err != nil {
		t.Fatalf("failed to seed main sessions: %v", err)
	}
	messages := []*CanvasMessage{
		{Id: 201, SessionId: chatSession.Id, UserId: 21, Mode: CanvasModeChat, Role: CanvasMessageRoleUser, Prompt: "chat", Status: CanvasMessageStatusSuccess, CreatedTime: 1, UpdatedTime: 1},
		{Id: 202, SessionId: imageSession.Id, UserId: 21, Mode: CanvasModeImage, Role: CanvasMessageRoleAssistant, TaskId: "301", TaskType: CanvasTaskTypeImage, Status: ImageTaskStatusSuccess, CreatedTime: 2, UpdatedTime: 2},
		{Id: 203, SessionId: videoSession.Id, UserId: 21, Mode: CanvasModeVideo, Role: CanvasMessageRoleAssistant, TaskId: "401", TaskType: CanvasTaskTypeVideo, Status: string(TaskStatusSuccess), CreatedTime: 3, UpdatedTime: 3},
	}
	if err := mainDB.Table("canvas_messages").Create(messages).Error; err != nil {
		t.Fatalf("failed to seed main messages: %v", err)
	}
	if err := mainDB.Create(&CanvasChatMessageAttachment{
		Id:          301,
		MessageId:   201,
		UserId:      21,
		SessionId:   chatSession.Id,
		SortOrder:   0,
		Kind:        "image",
		Name:        "ref.png",
		MimeType:    "image/png",
		Data:        "data:image/png;base64,AA==",
		CreatedTime: 1,
		UpdatedTime: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed main chat attachment: %v", err)
	}
	if err := mainDB.Create(&ImageGenerationTask{
		Id:              301,
		UserId:          21,
		ModelId:         "image-model",
		Prompt:          "image",
		RequestEndpoint: "openai",
		Status:          ImageTaskStatusSuccess,
		ImageUrl:        "/image.png",
		CreatedTime:     2,
		StartedTime:     2,
		CompletedTime:   2,
	}).Error; err != nil {
		t.Fatalf("failed to seed main image task: %v", err)
	}
	if err := mainDB.Create(&ImageGenerationReferenceAsset{
		Id:            501,
		ContentHash:   "hash-backfill",
		StorageType:   "local",
		StoragePath:   "reference.png",
		ContentType:   "image/png",
		FileSizeBytes: 12,
		RefCount:      1,
		CreatedTime:   2,
		LastUsedTime:  2,
	}).Error; err != nil {
		t.Fatalf("failed to seed main reference asset: %v", err)
	}
	if err := mainDB.Create(&ImageGenerationTaskReferenceAsset{
		Id:          601,
		TaskId:      301,
		AssetId:     501,
		CreatedTime: 2,
	}).Error; err != nil {
		t.Fatalf("failed to seed main task reference asset link: %v", err)
	}
	if err := mainDB.Create(&Task{
		ID:         401,
		UserId:     21,
		TaskID:     "video-backfill",
		Action:     constant.TaskActionTextGenerate,
		Status:     TaskStatusSuccess,
		SubmitTime: 3,
	}).Error; err != nil {
		t.Fatalf("failed to seed main video task: %v", err)
	}
	if err := mainDB.Create([]*CanvasAssetCleanupJob{
		{Id: 701, UserId: 21, TaskType: CanvasTaskTypeImage, TaskRecordId: "301", Status: CanvasAssetCleanupJobStatusPending, Payload: "{}", CreatedTime: 2, UpdatedTime: 2},
		{Id: 702, UserId: 21, TaskType: CanvasTaskTypeVideo, TaskRecordId: "401", Status: CanvasAssetCleanupJobStatusPending, Payload: "{}", CreatedTime: 3, UpdatedTime: 3},
	}).Error; err != nil {
		t.Fatalf("failed to seed main cleanup jobs: %v", err)
	}

	stats, err := BackfillCanvasDedicatedDBs()
	if err != nil {
		t.Fatalf("backfill failed: %v", err)
	}
	if stats[CanvasModeChat].Sessions != 1 || stats[CanvasModeChat].Messages != 1 || stats[CanvasModeChat].ChatAttachments != 1 {
		t.Fatalf("unexpected chat backfill stats: %#v", stats[CanvasModeChat])
	}
	if stats[CanvasModeImage].Sessions != 1 || stats[CanvasModeImage].Messages != 1 || stats[CanvasModeImage].ImageTasks != 1 || stats[CanvasModeImage].ImageReferenceAssets != 1 || stats[CanvasModeImage].ImageTaskReferenceAssets != 1 || stats[CanvasModeImage].AssetCleanupJobs != 1 {
		t.Fatalf("unexpected image backfill stats: %#v", stats[CanvasModeImage])
	}
	if stats[CanvasModeVideo].Sessions != 1 || stats[CanvasModeVideo].Messages != 1 || stats[CanvasModeVideo].VideoTasks != 1 || stats[CanvasModeVideo].AssetCleanupJobs != 1 {
		t.Fatalf("unexpected video backfill stats: %#v", stats[CanvasModeVideo])
	}

	stats, err = BackfillCanvasDedicatedDBs()
	if err != nil {
		t.Fatalf("second backfill failed: %v", err)
	}
	for mode, modeStats := range stats {
		if modeStats.Sessions != 0 || modeStats.Messages != 0 || modeStats.ChatAttachments != 0 || modeStats.ImageTasks != 0 || modeStats.ImageReferenceAssets != 0 || modeStats.ImageTaskReferenceAssets != 0 || modeStats.AssetCleanupJobs != 0 || modeStats.VideoTasks != 0 {
			t.Fatalf("expected idempotent second backfill for %s, got %#v", mode, modeStats)
		}
	}

	expectedCounts := map[string]map[string]int64{
		CanvasModeChat: {
			"canvas_sessions":                 1,
			canvasChatMessagesTable:           1,
			canvasChatMessageAttachmentsTable: 1,
		},
		CanvasModeImage: {
			"canvas_sessions":                        1,
			canvasImageMessagesTable:                 1,
			"image_generation_tasks":                 1,
			"image_generation_reference_assets":      1,
			"image_generation_task_reference_assets": 1,
			"canvas_asset_cleanup_jobs":              1,
		},
		CanvasModeVideo: {
			"canvas_sessions":           1,
			canvasVideoMessagesTable:    1,
			"tasks":                     1,
			"canvas_asset_cleanup_jobs": 1,
		},
	}
	for mode, tables := range expectedCounts {
		for tableName, expected := range tables {
			var count int64
			if err := childDBs[mode].Table(tableName).Count(&count).Error; err != nil {
				t.Fatalf("failed to count %s.%s: %v", mode, tableName, err)
			}
			if count != expected {
				t.Fatalf("expected %s.%s count %d, got %d", mode, tableName, expected, count)
			}
		}
	}
	var copiedChatSession CanvasSession
	if err := childDBs[CanvasModeChat].First(&copiedChatSession, chatSession.Id).Error; err != nil {
		t.Fatalf("failed to reload copied chat session: %v", err)
	}
	if strings.TrimSpace(copiedChatSession.PublicId) == "" {
		t.Fatalf("expected copied legacy session to receive public_id, got %#v", copiedChatSession)
	}
}

func TestCanvasDedicatedPostgresSequenceResetSQLUsesWhitelistedTargets(t *testing.T) {
	expectedTablesByMode := map[string][]string{
		CanvasModeChat: {
			"canvas_sessions",
			canvasChatMessagesTable,
			canvasChatMessageAttachmentsTable,
		},
		CanvasModeImage: {
			"canvas_sessions",
			canvasImageMessagesTable,
			"image_generation_tasks",
			"image_generation_reference_assets",
			"image_generation_task_reference_assets",
			"canvas_asset_cleanup_jobs",
		},
		CanvasModeVideo: {
			"canvas_sessions",
			canvasVideoMessagesTable,
			"tasks",
			"canvas_asset_cleanup_jobs",
		},
	}
	for mode, expectedTables := range expectedTablesByMode {
		targets := canvasDedicatedPostgresSequenceTargets(mode)
		if len(targets) != len(expectedTables) {
			t.Fatalf("expected %d sequence targets for %s, got %#v", len(expectedTables), mode, targets)
		}
		for index, expectedTable := range expectedTables {
			target := targets[index]
			if target.Table != expectedTable || target.Column != "id" {
				t.Fatalf("unexpected sequence target for %s at %d: %#v", mode, index, target)
			}
			query, args, err := canvasDedicatedPostgresSequenceResetSQL(target)
			if err != nil {
				t.Fatalf("failed to build reset SQL for %s.%s: %v", target.Table, target.Column, err)
			}
			if !strings.Contains(query, `pg_get_serial_sequence(?, ?)`) {
				t.Fatalf("expected reset SQL to bind sequence lookup args, got %q", query)
			}
			expectedMaxClause := fmt.Sprintf(`MAX("id") FROM "%s"`, expectedTable)
			if !strings.Contains(query, expectedMaxClause) {
				t.Fatalf("expected reset SQL for %s to contain %q, got %q", expectedTable, expectedMaxClause, query)
			}
			if len(args) != 2 || args[0] != expectedTable || args[1] != "id" {
				t.Fatalf("unexpected reset SQL args for %s: %#v", expectedTable, args)
			}
		}
	}
	if _, _, err := canvasDedicatedPostgresSequenceResetSQL(canvasDedicatedPostgresSequenceTarget{Table: "users", Column: "id"}); err == nil {
		t.Fatal("expected non-whitelisted sequence reset target to be rejected")
	}
}
