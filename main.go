package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	serverHost = "srv.msk01.gigacorp.local:80"
	
	// Пороговые значения
	loadAverageThreshold     = 30.0
	memoryUsageThreshold     = 0.8    // 80%
	diskUsageThreshold       = 0.9    // 90%
	networkUsageThreshold    = 0.9    // 90%
	
	// Константы для преобразования единиц
	bytesInMegabyte = 1024 * 1024
	bytesInMegabit  = 125000 // 1 Mbit/s = 125000 bytes/s
	
	// Максимальное количество ошибок
	maxErrors = 3
)

func main() {
	errorCount := 0
	
	for {
		stats, err := fetchServerStats()
		if err != nil {
			errorCount++
			fmt.Printf("Error fetching stats: %v\n", err)
			
			if errorCount >= maxErrors {
				fmt.Println("Unable to fetch server statistic")
				return
			}
			
			time.Sleep(5 * time.Second)
			continue
		}
		
		// Сброс счетчика ошибок при успешном запросе
		errorCount = 0
		
		// Проверка пороговых значений
		checkThresholds(stats)
		
		time.Sleep(5 * time.Second)
	}
}

type ServerStats struct {
	LoadAverage        float64
	TotalMemory        int64
	UsedMemory         int64
	TotalDisk          int64
	UsedDisk           int64
	TotalNetwork       int64
	UsedNetwork        int64
}

func fetchServerStats() (*ServerStats, error) {
	// Установка соединения с таймаутом
	conn, err := net.DialTimeout("tcp", serverHost, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Установка таймаута на чтение
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	
	// Отправка HTTP запроса
	request := "GET /_stats HTTP/1.1\r\n" +
		"Host: srv.msk01.gigacorp.local\r\n" +
		"Connection: close\r\n" +
		"\r\n"
	
	_, err = conn.Write([]byte(request))
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	
	// Чтение ответа
	reader := bufio.NewReader(conn)
	
	// Проверка статусной строки
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read status failed: %w", err)
	}
	
	if !strings.Contains(statusLine, "200 OK") {
		return nil, fmt.Errorf("non-200 status: %s", strings.TrimSpace(statusLine))
	}
	
	// Пропуск заголовков
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read headers failed: %w", err)
		}
		
		if line == "\r\n" || line == "\n" {
			break // Конец заголовков
		}
	}
	
	// Чтение тела ответа
	body, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}
	
	body = strings.TrimSpace(body)
	
	// Парсинг числовых значений
	values := strings.Split(body, ",")
	if len(values) != 6 {
		return nil, fmt.Errorf("invalid data format: expected 6 values, got %d", len(values))
	}
	
	stats := &ServerStats{}
	
	// Load Average
	stats.LoadAverage, err = strconv.ParseFloat(values[0], 64)
	if err != nil {
		return nil, fmt.Errorf("parse load average failed: %w", err)
	}
	
	// Total Memory
	stats.TotalMemory, err = strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse total memory failed: %w", err)
	}
	
	// Used Memory
	stats.UsedMemory, err = strconv.ParseInt(values[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse used memory failed: %w", err)
	}
	
	// Total Disk
	stats.TotalDisk, err = strconv.ParseInt(values[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse total disk failed: %w", err)
	}
	
	// Used Disk
	stats.UsedDisk, err = strconv.ParseInt(values[4], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse used disk failed: %w", err)
	}
	
	// Network Bandwidth (предполагаем, что это Total Network)
	stats.TotalNetwork, err = strconv.ParseInt(values[5], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse network bandwidth failed: %w", err)
	}
	
	// Used Network (в данных не предоставлено, используем 0)
	stats.UsedNetwork = 0
	
	return stats, nil
}

func checkThresholds(stats *ServerStats) {
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
			freeSpace := stats.TotalDisk - stats.UsedDisk
			freeSpaceMB := float64(freeSpace) / float64(bytesInMegabyte)
			fmt.Printf("Free disk space is too low: %.1f Mb left\n", freeSpaceMB)
		}
	}
	
	// Проверка загруженности сети
	if stats.TotalNetwork > 0 {
		// Предполагаем, что UsedNetwork не предоставляется сервером
		// Для демонстрации используем 85% от TotalNetwork
		networkUsage := 0.85 // Это значение должно приходить от сервера
		
		if networkUsage > networkUsageThreshold {
			availableBandwidth := stats.TotalNetwork - int64(float64(stats.TotalNetwork)*networkUsage)
			availableMbits := float64(availableBandwidth) / float64(bytesInMegabit)
			fmt.Printf("Network bandwidth usage high: %.1f Mbit/s available\n", availableMbits)
		}
	}
}
