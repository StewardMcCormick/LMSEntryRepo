package main

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow_Burst(t *testing.T) {
	limiter := NewRateLimiter(5, 5)

	// burst: первые 5 должны пройти
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// дальше токенов нет
	if limiter.Allow() {
		t.Fatal("request should be rejected when bucket is empty")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	limiter := NewRateLimiter(2, 5)

	// забираем все токены
	for i := 0; i < 5; i++ {
		limiter.Allow()
	}

	// ждём 1 секунду → +2 токена
	time.Sleep(1 * time.Second)

	count := 0
	for limiter.Allow() {
		count++
	}

	if count != 2 {
		t.Fatalf("expected 2 tokens after refill, got %d", count)
	}
}
