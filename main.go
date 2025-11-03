package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
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
	loadAverageThreshold  = 30.0
	memoryUsageThreshold  = 0.8
	diskUsageThreshold    = 0.9
	networkUsageThreshold = 0.9
	bytesToMegabytes      = 1024 * 1024
	bytesToMegabits       = 125000
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
	logger := log.New(os.Stdout, "", 0)
	logger.Println("Starting server monitoring...")
	logger.Printf("Server: %s\n", serverURL)
	logger.Printf("Poll interval: %v\n\n", pollInterval)

	monitor := NewMonitor(logger)
	monitor.Start()
}

type Monitor struct {
	logger     *log.Logger
	errorCount int
}

func NewMonitor(logger *log.Logger) *Monitor {
	return &Monitor{
		logger:     logger,
		errorCount: 0,
	}
}

func (m *Monitor) Start() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		stats, err := m.fetchStats()
		if err != nil {
			m.handleError(err)
			continue
		}

		m.checkMetrics(stats)
	}
}

func (m *Monitor) fetchStats() (*ServerStats, error) {
	resp, err := http.Get(serverURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty response body")
	}

	line := strings.TrimSpace(scanner.Text())
	values := strings.Split(line, ",")

	if len(values) != 6 {
		return nil, fmt.Errorf("invalid data format: expected 6 values, got %d", len(values))
	}

	stats := &ServerStats{}
	parseErrors := []string{}

	// Парсим все значения с обработкой ошибок
	if stats.LoadAverage, err = strconv.ParseFloat(values[0], 64); err != nil {
		parseErrors = append(parseErrors, fmt.Sprintf("load average: %v", err))
	}

	if stats.TotalMemory, err = strconv.ParseUint(values[1], 10, 64); err != nil {
		parseErrors = append(parseErrors, fmt.Sprintf("total memory: %v", err))
	}

	if stats.UsedMemory, err = strconv.ParseUint(values[2], 10, 64); err != nil {
		parseErrors = append(parseErrors, fmt.Sprintf("used memory: %v", err))
	}

	if stats.TotalDisk, err = strconv.ParseUint(values[3], 10, 64); err != nil {
		parseErrors = append(parseErrors, fmt.Sprintf("total disk: %v", err))
	}

	if stats.UsedDisk, err = strconv.ParseUint(values[4], 10, 64); err != nil {
		parseErrors = append(parseErrors, fmt.Sprintf("used disk: %v", err))
	}

	if stats.TotalNetwork, err = strconv.ParseUint(values[5], 10, 64); err != nil {
		parseErrors = append(parseErrors, fmt.Sprintf("total network: %v", err))
	}

	if len(parseErrors) > 0 {
		return nil, fmt.Errorf("parse errors: %s", strings.Join(parseErrors, "; "))
	}

	// В задании только 6 значений, предполагаем used_network = total_network
	stats.UsedNetwork = stats.TotalNetwork

	return stats, nil
}

func (m *Monitor) handleError(err error) {
	m.logger.Printf("Error: %v\n", err)
	m.errorCount++

	if m.errorCount >= maxErrors {
		m.logger.Println("Unable to fetch server statistic")
		m.errorCount = 0 // Сбрасываем счетчик
	}
}

func (m *Monitor) checkMetrics(stats *ServerStats) {
	// Сбрасываем счетчик ошибок при успешном запросе
	m.errorCount = 0

	// Проверка Load Average
	if stats.LoadAverage > loadAverageThreshold {
		m.logger.Printf("Load Average is too high: %.2f\n", stats.LoadAverage)
	}

	// Проверка использования памяти
	if stats.TotalMemory > 0 {
		memoryUsage := float64(stats.UsedMemory) / float64(stats.TotalMemory)
		if memoryUsage > memoryUsageThreshold {
			percentage := memoryUsage * 100
			m.logger.Printf("Memory usage too high: %.1f%%\n", percentage)
		}
	}

	// Проверка дискового пространства
	if stats.TotalDisk > 0 {
		diskUsage := float64(stats.UsedDisk) / float64(stats.TotalDisk)
		if diskUsage > diskUsageThreshold {
			freeSpace := float64(stats.TotalDisk - stats.UsedDisk) / bytesToMegabytes
			m.logger.Printf("Free disk space is too low: %.0f Mb left\n", freeSpace)
		}
	}

	// Проверка загруженности сети
	if stats.TotalNetwork > 0 {
		networkUsage := float64(stats.UsedNetwork) / float64(stats.TotalNetwork)
		if networkUsage > networkUsageThreshold {
			availableBandwidth := float64(stats.TotalNetwork - stats.UsedNetwork) / bytesToMegabits
			m.logger.Printf("Network bandwidth usage high: %.1f Mbit/s available\n", availableBandwidth)
		}
	}
}
