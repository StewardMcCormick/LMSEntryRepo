package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// RateLimiter ограничивает скорость запросов
type RateLimiter struct {
	Rate             int
	Capacity         int
	CurrentTokensNum int
	LastUpdate       time.Time
	mu               *sync.Mutex
}

// NewRateLimiter создаёт новый Rate limiter
//
// Параметры:
//
//	Rate - количество токенов, добавляемых в секунду
//	Capacity - максимальное количество токенов в ведре
//
// Пример: Rate=10, Capacity=20
//   - Каждую секунду добавляется 10 токенов
//   - Максимум может накопиться 20 токенов
//   - Можно сделать "burst" из 20 запросов, потом 10 запросов/сек
func NewRateLimiter(rate int, capacity int) *RateLimiter {
	limiter := &RateLimiter{
		Rate:             rate,
		Capacity:         capacity,
		CurrentTokensNum: capacity,
		LastUpdate:       time.Now(),
		mu:               &sync.Mutex{},
	}

	return limiter
}

func (rl *RateLimiter) UpdateTokensNum() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.LastUpdate)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.Rate))
	rl.CurrentTokensNum = min(rl.CurrentTokensNum+tokensToAdd, rl.Capacity)
	rl.LastUpdate = time.Now()
}

// Allow проверяет, можно ли выполнить запрос прямо сейчас
// Возвращает true, если токен доступен (и забирает его)
// Возвращает false, если токенов нет (не блокируется)
func (rl *RateLimiter) Allow() bool {
	rl.UpdateTokensNum()

	if rl.CurrentTokensNum >= 1 {
		rl.mu.Lock()
		rl.CurrentTokensNum--
		rl.mu.Unlock()

		return true
	}

	return false
}

// Wait блокируется до тех пор, пока не появится токен
// Возвращает nil при успехе
// Возвращает error при отмене контекста
func (rl *RateLimiter) Wait(ctx context.Context) error {
	timeToWait := time.Duration(1.0/rl.Rate) * time.Second
	ticker := time.NewTicker(timeToWait)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		if rl.GetAvailableTokens() >= 1 {
			break
		}
	}

	rl.mu.Lock()
	rl.CurrentTokensNum--
	rl.mu.Unlock()
	return nil
}

// WaitN блокируется до получения n токенов
// Полезно для операций, требующих несколько токенов
func (rl *RateLimiter) WaitN(ctx context.Context, n int) error {
	timeToWait := time.Duration(n/rl.Rate) * time.Second
	ticker := time.NewTicker(timeToWait)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		if rl.GetAvailableTokens() >= n {
			break
		}
	}

	rl.mu.Lock()
	rl.CurrentTokensNum -= n
	rl.mu.Unlock()
	return nil
}

// GetAvailableTokens возвращает текущее количество доступных токенов
func (rl *RateLimiter) GetAvailableTokens() int {
	rl.UpdateTokensNum()
	return rl.CurrentTokensNum
}

// Reset сбрасывает limiter (заполняет ведро до Capacity)
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.CurrentTokensNum = rl.Capacity
}

func main() {
	// 5 запросов в секунду, burst до 10
	limiter := NewRateLimiter(5, 10)

	// Неблокирующая проверка
	if limiter.Allow() {
		fmt.Println("Request allowed")
	} else {
		fmt.Println("Rate limit exceeded")
	}

	// Блокирующее ожидание
	ctx := context.Background()
	err := limiter.Wait(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Token acquired, processing request...")

	// Проверка доступных токенов
	available := limiter.GetAvailableTokens()
	fmt.Printf("Available tokens: %d\n", available)
}
