package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	serverURL    = "http://srv.msk01.gigacorp.local/_stats"
	pollInterval = 30 * time.Second
	maxErrors    = 3
)

// Пороговые значения
const (
	loadAverageThreshold      = 30.0
	memoryUsageThreshold      = 0.8  // 80%
	diskUsageThreshold        = 0.9  // 90%
	networkUsageThreshold     = 0.9  // 90%
	bytesToMegabytes          = 1024 * 1024
	bytesToMegabits           = 125000 // 1,000,000 bits / 8
)

type ServerStats struct {
	LoadAverage        float64
	TotalMemory        uint64
	UsedMemory         uint64
	TotalDisk          uint64
	UsedDisk           uint64
	TotalNetwork       uint64
	UsedNetwork        uint64
}

func main() {
	fmt.Println("Starting server monitoring...")
	fmt.Printf("Server: %s\n", serverURL)
	fmt.Printf("Poll interval: %v\n\n", pollInterval)

	errorCount := 0
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats, err := fetchStats()
			if err != nil {
				fmt.Printf("Error fetching stats: %v\n", err)
				errorCount++
				if errorCount >= maxErrors {
					fmt.Println("Unable to fetch server statistic")
					errorCount = 0 // Сбрасываем счетчик после вывода сообщения
				}
				continue
			}

			// Сбрасываем счетчик ошибок при успешном запросе
			errorCount = 0

			// Проверяем метрики и выводим предупреждения
			checkMetrics(stats)
		}
	}
}

func fetchStats() (*ServerStats, error) {
	resp, err := http.Get(serverURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %s", resp.Status)
	}

	// Читаем тело ответа
	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty response body")
	}

	line := scanner.Text()
	values := strings.Split(line, ",")
	
	if len(values) != 6 {
		return nil, fmt.Errorf("invalid data format: expected 6 values, got %d", len(values))
	}

	stats := &ServerStats{}

	// Парсим значения
	if stats.LoadAverage, err = strconv.ParseFloat(values[0], 64); err != nil {
		return nil, fmt.Errorf("invalid load average value: %v", err)
	}

	if stats.TotalMemory, err = strconv.ParseUint(values[1], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total memory value: %v", err)
	}

	if stats.UsedMemory, err = strconv.ParseUint(values[2], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid used memory value: %v", err)
	}

	if stats.TotalDisk, err = strconv.ParseUint(values[3], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total disk value: %v", err)
	}

	if stats.UsedDisk, err = strconv.ParseUint(values[4], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid used disk value: %v", err)
	}

	if stats.TotalNetwork, err = strconv.ParseUint(values[5], 10, 64); err != nil {
		return nil, fmt.Errorf("invalid total network value: %v", err)
	}

	// Для использованной сети используем то же значение что и общая пропускная способность
	// (в задании указано только 6 значений, предполагаем что used_network = total_network)
	stats.UsedNetwork = stats.TotalNetwork

	return stats, nil
}

func checkMetrics(stats *ServerStats) {
	// Проверка Load Average
	if stats.LoadAverage > loadAverageThreshold {
		fmt.Printf("Load Average is too high: %.2f\n", stats.LoadAverage)
	}

	// Проверка использования памяти
	if stats.TotalMemory > 0 {
		memoryUsage := float64(stats.UsedMemory) / float64(stats.TotalMemory)
		if memoryUsage > memoryUsageThreshold {
			percentage := memoryUsage * 100
			fmt.Printf("Memory usage too high: %.1f%%\n", percentage)
		}
	}

	// Проверка дискового пространства
	if stats.TotalDisk > 0 {
		diskUsage := float64(stats.UsedDisk) / float64(stats.TotalDisk)
		if diskUsage > diskUsageThreshold {
			freeSpace := float64(stats.TotalDisk - stats.UsedDisk) / bytesToMegabytes
			fmt.Printf("Free disk space is too low: %.0f Mb left\n", freeSpace)
		}
	}

	// Проверка загруженности сети
	if stats.TotalNetwork > 0 {
		networkUsage := float64(stats.UsedNetwork) / float64(stats.TotalNetwork)
		if networkUsage > networkUsageThreshold {
			availableBandwidth := float64(stats.TotalNetwork - stats.UsedNetwork) / bytesToMegabits
			fmt.Printf("Network bandwidth usage high: %.1f Mbit/s available\n", availableBandwidth)
		}
	}
}
