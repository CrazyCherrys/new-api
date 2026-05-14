package service

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const imageGenerationTaskUpdateRedisChannel = "new-api:image_generation_task_updates:v1"

var imageGenerationTaskUpdateRelayOnce sync.Once

type ImageGenerationTaskUpdate struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id"`
	ModelId       string `json:"model_id"`
	Prompt        string `json:"prompt"`
	Status        string `json:"status"`
	ImageUrl      string `json:"image_url"`
	ThumbnailUrl  string `json:"thumbnail_url"`
	ErrorMessage  string `json:"error_message"`
	CreatedTime   int64  `json:"created_time"`
	CompletedTime int64  `json:"completed_time"`
}

type imageGenerationTaskBroadcaster struct {
	mutex       sync.RWMutex
	subscribers map[int]map[chan ImageGenerationTaskUpdate]struct{}
}

func newImageGenerationTaskBroadcaster() *imageGenerationTaskBroadcaster {
	return &imageGenerationTaskBroadcaster{
		subscribers: make(map[int]map[chan ImageGenerationTaskUpdate]struct{}),
	}
}

func (b *imageGenerationTaskBroadcaster) Subscribe(userId int, buffer int) chan ImageGenerationTaskUpdate {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan ImageGenerationTaskUpdate, buffer)

	b.mutex.Lock()
	defer b.mutex.Unlock()
	if _, ok := b.subscribers[userId]; !ok {
		b.subscribers[userId] = make(map[chan ImageGenerationTaskUpdate]struct{})
	}
	b.subscribers[userId][ch] = struct{}{}
	return ch
}

func (b *imageGenerationTaskBroadcaster) Unsubscribe(userId int, ch chan ImageGenerationTaskUpdate) {
	if ch == nil {
		return
	}

	b.mutex.Lock()
	if subs, ok := b.subscribers[userId]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(b.subscribers, userId)
		}
	}
	b.mutex.Unlock()
	close(ch)
}

func (b *imageGenerationTaskBroadcaster) Publish(update ImageGenerationTaskUpdate) {
	if update.UserId <= 0 {
		return
	}

	b.mutex.RLock()
	subs := b.subscribers[update.UserId]
	channels := make([]chan ImageGenerationTaskUpdate, 0, len(subs))
	for ch := range subs {
		channels = append(channels, ch)
	}
	b.mutex.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- update:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- update:
			default:
			}
		}
	}
}

func imageGenerationTaskPubSubEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

func ensureImageGenerationTaskUpdateRelayStarted() {
	if !imageGenerationTaskPubSubEnabled() {
		return
	}
	imageGenerationTaskUpdateRelayOnce.Do(func() {
		go runImageGenerationTaskUpdateRelay()
	})
}

func runImageGenerationTaskUpdateRelay() {
	for {
		if !imageGenerationTaskPubSubEnabled() {
			return
		}

		ctx := context.Background()
		pubsub := common.RDB.Subscribe(ctx, imageGenerationTaskUpdateRedisChannel)
		if _, err := pubsub.Receive(ctx); err != nil {
			_ = pubsub.Close()
			time.Sleep(time.Second)
			continue
		}

		ch := pubsub.Channel()
		for msg := range ch {
			var update ImageGenerationTaskUpdate
			if err := common.Unmarshal([]byte(msg.Payload), &update); err != nil {
				continue
			}
			imageGenerationTaskUpdates.Publish(update)
		}

		_ = pubsub.Close()
		time.Sleep(time.Second)
	}
}

func dispatchImageGenerationTaskUpdate(update ImageGenerationTaskUpdate) {
	if update.UserId <= 0 {
		return
	}

	if imageGenerationTaskPubSubEnabled() {
		payload, err := common.Marshal(update)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = common.RDB.Publish(ctx, imageGenerationTaskUpdateRedisChannel, payload).Err()
			cancel()
			if err == nil {
				return
			}
		}
	}

	imageGenerationTaskUpdates.Publish(update)
}

func buildImageGenerationTaskUpdate(task *model.ImageGenerationTask) ImageGenerationTaskUpdate {
	if task == nil {
		return ImageGenerationTaskUpdate{}
	}
	return ImageGenerationTaskUpdate{
		Id:            task.Id,
		UserId:        task.UserId,
		ModelId:       task.ModelId,
		Prompt:        task.Prompt,
		Status:        task.Status,
		ImageUrl:      task.ImageUrl,
		ThumbnailUrl:  task.ThumbnailUrl,
		ErrorMessage:  task.ErrorMessage,
		CreatedTime:   task.CreatedTime,
		CompletedTime: task.CompletedTime,
	}
}

func publishImageGenerationTaskUpdate(task *model.ImageGenerationTask) {
	if task == nil {
		return
	}
	dispatchImageGenerationTaskUpdate(buildImageGenerationTaskUpdate(task))
}

func publishImageGenerationTaskUpdateByID(taskId int) {
	task, err := model.GetImageTaskByID(taskId)
	if err != nil || task == nil {
		return
	}
	publishImageGenerationTaskUpdate(task)
}

func SubscribeImageGenerationTaskUpdates(userId int) chan ImageGenerationTaskUpdate {
	ensureImageGenerationTaskUpdateRelayStarted()
	return imageGenerationTaskUpdates.Subscribe(userId, 32)
}

func UnsubscribeImageGenerationTaskUpdates(userId int, ch chan ImageGenerationTaskUpdate) {
	imageGenerationTaskUpdates.Unsubscribe(userId, ch)
}

func ImageGenerationTaskHeartbeatInterval() time.Duration {
	return 30 * time.Second
}
