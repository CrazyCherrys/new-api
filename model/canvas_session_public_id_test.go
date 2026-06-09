package model

import "testing"

func TestEnsureSessionPublicIDStaleCopiesReturnPersistedID(t *testing.T) {
	db := setupCanvasMessageStoreTestDB(t)

	session := &CanvasSession{
		UserId:      81,
		Mode:        CanvasModeImage,
		Title:       "stale public id",
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("failed to create stale session: %v", err)
	}

	store, err := canvasSessionStoreForMode(CanvasModeImage)
	if err != nil {
		t.Fatalf("failed to load canvas session store: %v", err)
	}

	firstStaleCopy := *session
	secondStaleCopy := *session
	if err := store.ensureSessionPublicID(&firstStaleCopy); err != nil {
		t.Fatalf("failed to ensure first public_id: %v", err)
	}
	if firstStaleCopy.PublicId == "" {
		t.Fatal("expected first stale copy to receive persisted public_id")
	}
	if err := store.ensureSessionPublicID(&secondStaleCopy); err != nil {
		t.Fatalf("failed to ensure second public_id: %v", err)
	}
	if secondStaleCopy.PublicId != firstStaleCopy.PublicId {
		t.Fatalf("expected both stale copies to return same persisted public_id, got %q and %q", firstStaleCopy.PublicId, secondStaleCopy.PublicId)
	}

	var persisted CanvasSession
	if err := db.First(&persisted, session.Id).Error; err != nil {
		t.Fatalf("failed to reload persisted session: %v", err)
	}
	if persisted.PublicId != firstStaleCopy.PublicId {
		t.Fatalf("expected persisted public_id %q, got %q", firstStaleCopy.PublicId, persisted.PublicId)
	}
}
