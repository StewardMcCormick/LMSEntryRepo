package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancer распределяет запросы между backend'ами
type LoadBalancer struct {
	Backends []string
	Counter  int64
	Stats    BalancerStats
	Client   http.Client
}

// NewLoadBalancer создаёт новый балансировщик
// backends — список URL backend-серверов (например, []string{"http://server1:8080", "http://server2:8080"})
func NewLoadBalancer(backends []string) *LoadBalancer {
	lb := &LoadBalancer{
		Backends: backends,
		Stats:    BalancerStats{RequestsPerBackend: make(map[string]int)},
		Client:   http.Client{Timeout: time.Second * 3},
	}

	return lb
}

// Get выполняет HTTP GET запрос к указанному пути
// Запросы распределяются по Round-Robin алгоритму
//
// Параметры:
//
//	ctx - контекст для управления временем жизни запроса
//	path - путь запроса (например, "/api/users")
//
// Возвращает:
//
//	[]byte - полное тело ответа (даже если оно было chunked)
//	error - ошибка, если все попытки исчерпаны
//
// Алгоритм работы:
// 1. Выбрать следующий backend по Round-Robin
// 2. Сделать GET запрос с таймаутом 3 секунды
// 3. Если получен статус 2xx:
//   - Прочитать всё тело ответа (даже если chunked)
//   - Вернуть данные
//
// 4. Если получен статус 5xx или ошибка:
//   - Попробовать следующий backend (до 3 попыток)
//
// 5. Если все попытки исчерпаны — вернуть ошибку
func (lb *LoadBalancer) Get(ctx context.Context, path string) ([]byte, error) {
	backendNum := lb.Counter % int64(len(lb.Backends))
	url := lb.Backends[backendNum] + path
	var tryCount int

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		for tryCount <= 2 {
			lb.Stats.AddRequest(lb.Backends[backendNum])
			atomic.AddInt64(&lb.Counter, 1)

			resp, err := lb.Client.Get(url)

			if err != nil {
				tryCount++
				resp.Body.Close()
				continue
			}

			if resp.Status[0] == '5' {
				tryCount++
				resp.Body.Close()
				continue
			}

			respBody, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				return nil, err
			}

			return respBody, nil
		}
	}

	return nil, errors.New("")
}

// Post выполняет HTTP POST запрос
// Работает аналогично Get, но с телом запроса
//
// Параметры:
//
//	ctx - контекст
//	path - путь запроса
//	body - тело запроса
//
// Возвращает:
//
//	[]byte - тело ответа
//	error - ошибка
func (lb *LoadBalancer) Post(ctx context.Context, path string, body []byte) ([]byte, error) {
	backendNum := lb.Counter % int64(len(lb.Backends))
	url := lb.Backends[backendNum] + path
	var tryCount int

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		for tryCount <= 2 {
			lb.Stats.AddRequest(lb.Backends[backendNum])
			atomic.AddInt64(&lb.Counter, 1)

			resp, err := lb.Client.Post(url, "text/json", bytes.NewBuffer(body))
			if err != nil {
				tryCount++
				resp.Body.Close()
				continue
			}

			if resp.Status[0] == '5' {
				tryCount++
				resp.Body.Close()
				continue
			}

			respBody, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				return nil, err
			}

			return respBody, nil
		}
	}

	return nil, errors.New("")
}

// GetStats возвращает статистику запросов
func (lb *LoadBalancer) GetStats() BalancerStats {
	return lb.Stats
}

// BalancerStats Stats содержит статистику балансировщика
type BalancerStats struct {
	TotalRequests      int            // Общее количество запросов
	RequestsPerBackend map[string]int // Количество запросов на каждый backend
}

func (bs *BalancerStats) AddRequest(backend string) {
	mu := &sync.Mutex{}
	mu.Lock()
	defer mu.Unlock()

	bs.TotalRequests++
	bs.RequestsPerBackend[backend]++
}

func main() {

}
