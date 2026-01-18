package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type CacheValue struct {
	Value      interface{}
	TTL        time.Duration
	LastAccess time.Time
}

// CacheManager управляет кешем с TTL
type CacheManager struct {
	Cache       map[string]*CacheValue
	Capacity    int
	DefaultTTL  time.Duration
	CleanupTick time.Duration
	Stats       CacheStats
	mu          *sync.RWMutex
}

// NewCacheManager создаёт новый кеш-менеджер
//
// Параметры:
//
//	maxSize - максимальное количество записей в кеше
//	defaultTTL - время жизни записи по умолчанию
func NewCacheManager(maxSize int, defaultTTL, cleanupTick time.Duration) *CacheManager {
	cm := &CacheManager{
		Cache:       make(map[string]*CacheValue),
		Capacity:    maxSize,
		DefaultTTL:  defaultTTL,
		CleanupTick: cleanupTick,
		Stats:       CacheStats{},
		mu:          &sync.RWMutex{},
	}

	return cm
}

func (cm *CacheManager) lru() {
	oldestTime := time.Now()
	var oldestKey string

	for k, v := range cm.Cache {
		if v.LastAccess.Compare(oldestTime) == -1 {
			oldestTime = v.LastAccess
			oldestKey = k
		}
	}

	delete(cm.Cache, oldestKey)
	atomic.AddInt64(&cm.Stats.Evictions, 1)
}

// Set добавляет или обновляет запись в кеше
// Если кеш переполнен, удаляется самая старая запись (LRU)
func (cm *CacheManager) Set(key string, value interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, ok := cm.Cache[key]
	if !ok && len(cm.Cache) == cm.Capacity {
		cm.lru()
	}

	cm.Cache[key] = &CacheValue{
		Value:      value,
		TTL:        cm.DefaultTTL,
		LastAccess: time.Now(),
	}
}

// SetWithTTL добавляет запись с кастомным TTL
func (cm *CacheManager) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, ok := cm.Cache[key]
	if !ok && len(cm.Cache) == cm.Capacity {
		cm.lru()
	}

	cm.Cache[key] = &CacheValue{
		Value:      value,
		TTL:        ttl,
		LastAccess: time.Now(),
	}
}

// Get получает значение из кеша
// При успешном чтении обновляет время последнего доступа (продлевает TTL)
// Возвращает (value, true) если ключ найден и не устарел
// Возвращает (nil, false) если ключ не найден или устарел
func (cm *CacheManager) Get(key string) (interface{}, bool) {
	cm.mu.RLock()
	v, ok := cm.Cache[key]
	cm.mu.RUnlock()
	if !ok || time.Since(v.LastAccess) > v.TTL {
		atomic.AddInt64(&cm.Stats.Misses, 1)
		return nil, false
	}

	atomic.AddInt64(&cm.Stats.Hits, 1)
	return v.Value, true
}

// Delete удаляет запись из кеша
func (cm *CacheManager) Delete(key string) {
	if _, ok := cm.Cache[key]; ok {
		delete(cm.Cache, key)
	}
}

// Clear очищает весь кеш
func (cm *CacheManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Cache = make(map[string]*CacheValue)
}

func (cm *CacheManager) ClearByTTL() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for k, v := range cm.Cache {
		if now.Sub(v.LastAccess) > v.TTL {
			atomic.AddInt64(&cm.Stats.Evictions, 1)
			cm.Delete(k)
		}
	}
}

// StartCleanup запускает фоновую очистку устаревших записей
// Очистка выполняется каждые cleanupInterval
// Останавливается при отмене контекста
func (cm *CacheManager) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(cm.CleanupTick)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cm.ClearByTTL()
			}
		}
	}()
}

// GetStats возвращает статистику кеша
func (cm *CacheManager) GetStats() CacheStats {
	atomic.StoreInt64(&cm.Stats.Size, 1)
	return cm.Stats
}

// CacheStats содержит статистику работы кеша
type CacheStats struct {
	Hits      int64 // Количество успешных Get
	Misses    int64 // Количество неуспешных Get
	Evictions int64 // Количество вытесненных записей (из-за переполнения или TTL)
	Size      int64 // Текущий размер кеша
}

func main() {
	cache := NewCacheManager(100, 5*time.Second, 1*time.Second)
	wg := &sync.WaitGroup{}

	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cache.Set(strconv.Itoa(i), i)
		}()
	}

	wg.Wait()
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			r, _ := cache.Get(strconv.Itoa(i))

			fmt.Println(i, r)
		}()
	}

	wg.Wait()
	fmt.Println(cache.Stats)
}
