package main

import (
	"fmt"
	"io"
	"math"
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
	bitsInMb      = 125000       // 1 мегабит = 1,000,000 бит / 8 = 125,000 байт
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
	
	if len(values) < 6 {
		return nil, fmt.Errorf("invalid data format: expected at least 6 values, got %d", len(values))
	}

	stats := &ServerStats{}
	
	// Парсим значения в правильном порядке согласно описанию:
	// 1. Load Average
	if stats.LoadAverage, err = strconv.ParseFloat(values[0], 64); err != nil {
		return nil, fmt.Errorf("invalid load average format: %v", err)
	}
	
	// 2. Total Memory
	if stats.TotalMemory, err = strconv.ParseUint(values[1], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total memory format: %v", err)
	}
	
	// 3. Used Memory
	if stats.UsedMemory, err = strconv.ParseUint(values[2], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid used memory format: %v", err)
	}
	
	// 4. Total Disk
	if stats.TotalDisk, err = strconv.ParseUint(values[3], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total disk format: %v", err)
	}
	
	// 5. Used Disk
	if stats.UsedDisk, err = strconv.ParseUint(values[4], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid used disk format: %v", err)
	}
	
	// 6. Network Bandwidth (пропускная способность)
	if stats.NetworkBandwidth, err = strconv.ParseUint(values[5], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid network bandwidth format: %v", err)
	}

	// 7. Network Usage (текущая загруженность) - если есть
	if len(values) >= 7 {
		if stats.NetworkUsage, err = strconv.ParseUint(values[6], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid network usage format: %v", err)
		}
	} else {
		// Если нет отдельного значения использования сети, не проверяем сеть
		stats.NetworkUsage = 0
		stats.NetworkBandwidth = 0
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
			percentage := math.Round(memoryUsage * 100)
			fmt.Printf("Memory usage too high: %.0f%%\n", percentage)
		}
	}

	// Проверка дискового пространства
	if stats.TotalDisk > 0 {
		diskUsage := float64(stats.UsedDisk) / float64(stats.TotalDisk)
		if diskUsage > diskUsageThreshold {
			freeSpace := float64(stats.TotalDisk - stats.UsedDisk) / float64(bytesInMb)
			fmt.Printf("Free disk space is too low: %.0f Mb left\n", math.Round(freeSpace))
		}
	}

	// Проверка загруженности сети (только если есть оба значения)
	if stats.NetworkBandwidth > 0 && stats.NetworkUsage > 0 {
		networkUsage := float64(stats.NetworkUsage) / float64(stats.NetworkBandwidth)
		if networkUsage > networkUsageThreshold {
			// Свободная полоса = общая пропускная способность - текущее использование
			freeBandwidth := float64(stats.NetworkBandwidth - stats.NetworkUsage)
			// Конвертируем из байт/сек в мегабит/сек: байты * 8 / 1,000,000
			freeMbits := (freeBandwidth * 8) / 1000000
			fmt.Printf("Network bandwidth usage high: %.0f Mbit/s available\n", math.Round(freeMbits))
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
	NetworkBandwidth uint64 // Пропускная способность (байт/сек)
	NetworkUsage     uint64 // Текущее использование (байт/сек)
}
