package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// AcquireRPMSlot 尝试获取 RPM 限流槽位（Redis 滑窗实现）
// 返回值：acquired - 是否获取成功，retryAfter - 重试等待时间，err - 错误信息
func AcquireRPMSlot(key string, rpm int, window time.Duration) (acquired bool, retryAfter time.Duration, err error) {
	// rpm 为 0 表示不限制
	if rpm == 0 {
		return true, 0, nil
	}

	// 优先使用 Redis，失败时降级到内存
	if RedisEnabled && RDB != nil {
		acquired, retryAfter, err = acquireRPMSlotRedis(key, rpm, window)
		if err == nil {
			return acquired, retryAfter, nil
		}
		// Redis 失败，记录日志并降级到内存
		SysLog(fmt.Sprintf("Redis RPM limiter failed, fallback to memory: %v", err))
	}

	// 使用内存滑窗
	return acquireRPMSlotMemory(key, rpm, window)
}

// acquireRPMSlotRedis Redis 滑窗计数器实现
func acquireRPMSlotRedis(key string, rpm int, window time.Duration) (bool, time.Duration, error) {
	ctx := context.Background()
	now := time.Now()
	nowMs := now.UnixMilli()
	windowStart := now.Add(-window).UnixMilli()

	// 使用 Lua 脚本保证原子性
	script := redis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local max_requests = tonumber(ARGV[3])
		local ttl = tonumber(ARGV[4])

		-- 删除窗口外的旧记录
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		-- 获取当前窗口内的请求数
		local current_count = redis.call('ZCARD', key)

		if current_count < max_requests then
			-- 未达到限制，添加新请求
			redis.call('ZADD', key, now, now)
			redis.call('EXPIRE', key, ttl)
			return {1, 0}
		else
			-- 已达到限制，计算最早请求的过期时间
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			if #oldest >= 2 then
				local oldest_time = tonumber(oldest[2])
				local retry_after = oldest_time + (ttl * 1000) - now
				if retry_after < 0 then
					retry_after = 0
				end
				return {0, retry_after}
			end
			return {0, ttl * 1000}
		end
	`)

	ttlSeconds := int(window.Seconds())
	result, err := script.Run(ctx, RDB, []string{key}, nowMs, windowStart, rpm, ttlSeconds).Result()
	if err != nil {
		return false, 0, err
	}

	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) < 2 {
		return false, 0, fmt.Errorf("unexpected script result format")
	}

	acquired := resultSlice[0].(int64) == 1
	retryAfterMs := resultSlice[1].(int64)
	retryAfter := time.Duration(retryAfterMs) * time.Millisecond

	return acquired, retryAfter, nil
}

// 内存滑窗限流器
type memoryRPMSlot struct {
	timestamps []int64
	mu         sync.Mutex
}

var (
	memoryRPMStore = sync.Map{}
	memoryRPMOnce  sync.Once
)

// initMemoryRPMCleaner 初始化内存清理器
func initMemoryRPMCleaner() {
	memoryRPMOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cleanExpiredMemoryRPMSlots()
			}
		}()
	})
}

// cleanExpiredMemoryRPMSlots 清理过期的内存槽位
func cleanExpiredMemoryRPMSlots() {
	now := time.Now().UnixMilli()
	memoryRPMStore.Range(func(key, value interface{}) bool {
		slot := value.(*memoryRPMSlot)
		slot.mu.Lock()
		// 如果最新的时间戳超过 10 分钟，删除整个槽位
		if len(slot.timestamps) > 0 && now-slot.timestamps[len(slot.timestamps)-1] > 10*60*1000 {
			memoryRPMStore.Delete(key)
		}
		slot.mu.Unlock()
		return true
	})
}

// acquireRPMSlotMemory 内存滑窗计数器实现
func acquireRPMSlotMemory(key string, rpm int, window time.Duration) (bool, time.Duration, error) {
	initMemoryRPMCleaner()

	now := time.Now()
	nowMs := now.UnixMilli()
	windowStart := now.Add(-window).UnixMilli()

	// 获取或创建槽位
	value, _ := memoryRPMStore.LoadOrStore(key, &memoryRPMSlot{
		timestamps: make([]int64, 0, rpm),
	})
	slot := value.(*memoryRPMSlot)

	slot.mu.Lock()
	defer slot.mu.Unlock()

	// 移除窗口外的旧时间戳
	validIdx := 0
	for i, ts := range slot.timestamps {
		if ts > windowStart {
			validIdx = i
			break
		}
	}
	if validIdx > 0 {
		slot.timestamps = slot.timestamps[validIdx:]
	} else if len(slot.timestamps) > 0 && slot.timestamps[len(slot.timestamps)-1] <= windowStart {
		// 所有时间戳都过期了
		slot.timestamps = slot.timestamps[:0]
	}

	// 检查是否达到限制
	currentCount := len(slot.timestamps)
	if currentCount < rpm {
		// 未达到限制，添加新时间戳
		slot.timestamps = append(slot.timestamps, nowMs)
		return true, 0, nil
	}

	// 已达到限制，计算重试时间
	oldestTs := slot.timestamps[0]
	retryAfterMs := oldestTs + window.Milliseconds() - nowMs
	if retryAfterMs < 0 {
		retryAfterMs = 0
	}
	retryAfter := time.Duration(retryAfterMs) * time.Millisecond

	return false, retryAfter, nil
}
