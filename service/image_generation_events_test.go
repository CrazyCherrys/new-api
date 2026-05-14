package service

import (
	"testing"
	"time"
)

func TestImageGenerationTaskBroadcasterDeliversLatestUpdate(t *testing.T) {
	broadcaster := newImageGenerationTaskBroadcaster()
	ch := broadcaster.Subscribe(7, 1)
	defer broadcaster.Unsubscribe(7, ch)

	first := ImageGenerationTaskUpdate{
		Id:     1,
		UserId: 7,
		Status: "pending",
	}
	second := ImageGenerationTaskUpdate{
		Id:     1,
		UserId: 7,
		Status: "success",
	}

	broadcaster.Publish(first)
	broadcaster.Publish(second)

	select {
	case got := <-ch:
		if got.Status != "success" {
			t.Fatalf("expected latest update status success, got %q", got.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast update")
	}
}

func TestImageGenerationTaskBroadcasterIsUserScoped(t *testing.T) {
	broadcaster := newImageGenerationTaskBroadcaster()
	ch := broadcaster.Subscribe(7, 1)
	defer broadcaster.Unsubscribe(7, ch)

	broadcaster.Publish(ImageGenerationTaskUpdate{
		Id:     1,
		UserId: 8,
		Status: "pending",
	})

	select {
	case got := <-ch:
		t.Fatalf("unexpected update for different user: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
}
