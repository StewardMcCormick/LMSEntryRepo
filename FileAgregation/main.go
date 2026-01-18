package main

import (
	"bufio"
	"context"
	"os"
	"strings"
	"sync"
)

// FileStats содержит статистику одного файла
type FileStats struct {
	Path        string // Путь к файлу
	Lines       int    // Количество строк
	Words       int    // Количество слов
	UniqueWords int    // Количество уникальных слов
	Size        int64  // Размер файла в байтах
}

// AggregatedStats содержит общую статистику
type AggregatedStats struct {
	TotalFiles       int         // Количество обработанных файлов
	TotalLines       int         // Общее количество строк
	TotalWords       int         // Общее количество слов
	TotalUniqueWords int         // Количество уникальных слов во всех файлах
	FileStats        []FileStats // Статистика по каждому файлу
}

func handleFile(ctx context.Context, file *os.File, totalUniqMap map[string]struct{}, minSize int64, mu *sync.Mutex) (FileStats, bool) {
	fileInfo, _ := os.Stat(file.Name())
	uniqWord := make(map[string]struct{})

	if fileInfo.Size() < minSize {
		return FileStats{}, false
	}

	result := FileStats{Path: file.Name(), Size: fileInfo.Size()}
	select {
	case <-ctx.Done():
		return result, true
	default:
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			text := strings.ToLower(scanner.Text())
			result.Lines++
			for _, s := range strings.Fields(text) {
				if _, ok := uniqWord[s]; !ok {
					uniqWord[s] = struct{}{}
					result.UniqueWords++
				}

				mu.Lock()
				if _, ok := totalUniqMap[s]; !ok {
					totalUniqMap[s] = struct{}{}
				}
				mu.Unlock()
				result.Words++
			}
		}
	}

	return result, true
}

// ProcessFiles обрабатывает список файлов параллельно
//
// Параметры:
//
//	ctx - контекст для управления временем жизни
//	filePaths - список путей к файлам
//	maxWorkers - максимальное количество одновременно обрабатываемых файлов
//	minSize - минимальный размер файла в байтах (меньшие игнорируются)
//
// Возвращает:
//
//	*AggregatedStats - агрегированная статистика
//	error - ошибка (nil при отмене контекста, возвращаются частичные результаты)
//
// Логика работы:
// 1. Для каждого файла проверить размер (os.Stat)
// 2. Если размер < minSize — пропустить файл
// 3. Прочитать файл и подсчитать статистику
// 4. Одновременно обрабатывать не более maxWorkers файлов
// 5. При отмене контекста вернуть то, что успели обработать
func ProcessFiles(ctx context.Context, filePaths []string, maxWorkers int, minSize int64) (*AggregatedStats, error) {
	pool := make(chan struct{}, maxWorkers)
	for range maxWorkers {
		pool <- struct{}{}
	}

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}
	result := &AggregatedStats{}
	totalUniqWordsMap := make(map[string]struct{})

	for _, path := range filePaths {
		_ = <-pool

		file, err := os.Open(path)
		if err != nil {
			pool <- struct{}{}
			continue
		}

		wg.Add(1)
		go func() {
			defer func() {
				pool <- struct{}{}
				wg.Done()
			}()

			select {
			case <-ctx.Done():
				return
			default:
				fileStats, status := handleFile(ctx, file, totalUniqWordsMap, minSize, mu)
				if !status {
					return
				}

				result.FileStats = append(result.FileStats, fileStats)
				result.TotalFiles++
				result.TotalLines += fileStats.Lines
				result.TotalWords += fileStats.Words
			}
		}()
	}

	wg.Wait()
	result.TotalUniqueWords = len(totalUniqWordsMap)
	return result, nil
}

//func main() {
//	ctx := context.Background()
//
//	filePaths := []string{
//		"LMSEntry/FileAgregation/tmp/file1.txt",
//	}
//
//	stats, err := ProcessFiles(ctx, filePaths, 2, 0) // maxWorkers=2, minSize=100 bytes
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Processed %d files\n", stats.TotalFiles)
//	fmt.Printf("Total lines: %d\n", stats.TotalLines)
//	fmt.Printf("Total words: %d\n", stats.TotalWords)
//	fmt.Printf("Unique words: %d\n", stats.TotalUniqueWords)
//
//	for _, fs := range stats.FileStats {
//		fmt.Printf("%s: %d lines, %d words, %d unique\n",
//			fs.Path, fs.Lines, fs.Words, fs.UniqueWords)
//	}
//}
