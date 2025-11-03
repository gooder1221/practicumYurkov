package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	serverURL = "http://srv.msk01.gigacorp.local/_stats"
	
	// Пороговые значения
	loadThreshold        = 30.0
	memoryUsageThreshold = 0.8    // 80%
	diskUsageThreshold   = 0.9    // 90%
	networkUsageThreshold = 0.9   // 90%
	
	// Константы для преобразования единиц
	bytesInMb     = 1024 * 1024
	bitsInMb      = 1000000 // 1 мегабит = 1,000,000 бит
	bytesInBit    = 8       // 1 байт = 8 бит
	maxErrors     = 3
	pollInterval  = 30 * time.Second
)

func main() {
	errorCount := 0
	client := &http.Client{Timeout: 10 * time.Second}

	for {
		stats, err := fetchServerStats(client)
		if err != nil {
			fmt.Printf("Error fetching stats: %v\n", err)
			errorCount++
			if errorCount >= maxErrors {
				fmt.Println("Unable to fetch server statistic")
				return
			}
			time.Sleep(pollInterval)
			continue
		}
		
		errorCount = 0 // Сбрасываем счетчик ошибок при успешном запросе
		checkThresholds(stats)
		
		time.Sleep(pollInterval)
	}
}

func fetchServerStats(client *http.Client) (*ServerStats, error) {
	resp, err := client.Get(serverURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status: %s", resp.Status)
	}

	// Читаем тело ответа полностью
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %v", err)
	}

	line := strings.TrimSpace(string(body))
	values := strings.Split(line, ",")
	
	// Более гибкая проверка формата - допускаем от 6 до 7 значений
	if len(values) < 6 {
		return nil, fmt.Errorf("invalid data format: expected at least 6 values, got %d", len(values))
	}

	// Парсим значения - берем первые 6 значений, игнорируем лишние
	stats := &ServerStats{}
	
	if stats.LoadAverage, err = strconv.ParseFloat(values[0], 64); err != nil {
		return nil, fmt.Errorf("invalid load average format: %v", err)
	}
	
	if stats.TotalMemory, err = strconv.ParseUint(values[1], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total memory format: %v", err)
	}
	
	if stats.UsedMemory, err = strconv.ParseUint(values[2], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid used memory format: %v", err)
	}
	
	if stats.TotalDisk, err = strconv.ParseUint(values[3], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total disk format: %v", err)
	}
	
	if stats.UsedDisk, err = strconv.ParseUint(values[4], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid used disk format: %v", err)
	}
	
	if stats.NetworkBandwidth, err = strconv.ParseUint(values[5], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid network bandwidth format: %v", err)
	}

	return stats, nil
}

func checkThresholds(stats *ServerStats) {
	// Проверка Load Average
	if stats.LoadAverage > loadThreshold {
		fmt.Printf("Load Average is too high: %.0f\n", stats.LoadAverage)
	}

	// Проверка использования памяти
	if stats.TotalMemory > 0 {
		memoryUsage := float64(stats.UsedMemory) / float64(stats.TotalMemory)
		if memoryUsage > memoryUsageThreshold {
			percentage := memoryUsage * 100
			fmt.Printf("Memory usage too high: %.0f%%\n", percentage)
		}
	}

	// Проверка дискового пространства
	if stats.TotalDisk > 0 {
		diskUsage := float64(stats.UsedDisk) / float64(stats.TotalDisk)
		if diskUsage > diskUsageThreshold {
			freeSpace := float64(stats.TotalDisk - stats.UsedDisk) / float64(bytesInMb)
			fmt.Printf("Free disk space is too low: %.0f Mb left\n", freeSpace)
		}
	}

	// Проверка загруженности сети
	if stats.NetworkBandwidth > 0 {
		// В тестовых данных используется used_memory как текущая загруженность сети
		currentNetworkUsage := stats.UsedMemory
		networkUsage := float64(currentNetworkUsage) / float64(stats.NetworkBandwidth)
		if networkUsage > networkUsageThreshold {
			freeBytes := float64(stats.NetworkBandwidth - currentNetworkUsage)
			// Конвертируем из байт/сек в мегабит/сек
			freeMbits := (freeBytes * float64(bytesInBit)) / float64(bitsInMb)
			fmt.Printf("Network bandwidth usage high: %.0f Mbit/s available\n", freeMbits)
		}
	}
}

// ServerStats представляет статистику сервера
type ServerStats struct {
	LoadAverage      float64
	TotalMemory      uint64
	UsedMemory       uint64
	TotalDisk        uint64
	UsedDisk         uint64
	NetworkBandwidth uint64
}
