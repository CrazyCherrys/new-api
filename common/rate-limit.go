package common

import (
	"context"
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	if l.store == nil {
		l.mutex.Lock()
		if l.store == nil {
			l.store = make(map[string]*[]int64)
			l.expirationDuration = expirationDuration
			if expirationDuration > 0 {
				go l.clearExpiredItems()
			}
		}
		l.mutex.Unlock()
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key := range l.store {
			queue := l.store[key]
			size := len(*queue)
			if size == 0 || now-(*queue)[size-1] > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		if len(*queue) < maxRequestNum {
			*queue = append(*queue, now)
			return true
		} else {
			if now-(*queue)[0] >= duration {
				*queue = (*queue)[1:]
				*queue = append(*queue, now)
				return true
			} else {
				return false
			}
		}
	} else {
		s := make([]int64, 0, maxRequestNum)
		l.store[key] = &s
		*(l.store[key]) = append(*(l.store[key]), now)
	}
	return true
}

// RedisRateLimitRequest checks whether a request can pass within a sliding window.
// duration is in seconds.
func RedisRateLimitRequest(key string, maxRequestNum int, duration int64) (bool, error) {
	if maxRequestNum <= 0 {
		return true, nil
	}

	ctx := context.Background()
	now := time.Now().Unix()

	length, err := RDB.LLen(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if length < int64(maxRequestNum) {
		txn := RDB.TxPipeline()
		txn.LPush(ctx, key, now)
		txn.LTrim(ctx, key, 0, int64(maxRequestNum-1))
		txn.Expire(ctx, key, time.Duration(duration)*time.Second)
		_, err = txn.Exec(ctx)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	oldest, err := RDB.LIndex(ctx, key, -1).Int64()
	if err != nil {
		return false, err
	}

	if now-oldest >= duration {
		txn := RDB.TxPipeline()
		txn.RPop(ctx, key)
		txn.LPush(ctx, key, now)
		txn.LTrim(ctx, key, 0, int64(maxRequestNum-1))
		txn.Expire(ctx, key, time.Duration(duration)*time.Second)
		_, err = txn.Exec(ctx)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	if err = RDB.Expire(ctx, key, time.Duration(duration)*time.Second).Err(); err != nil {
		return false, err
	}
	return false, nil
}
