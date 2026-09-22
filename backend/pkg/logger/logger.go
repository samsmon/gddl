package logger

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type LogEntry struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`    // "INFO", "WARN", "ERROR", "SUCCESS"
	Category  string `json:"category"` // "Download", "FolderZip", "Queue", "Integrity", "Auth", "Server"
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
}

type RingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	maxSize int
	counter int64
}

var globalBuffer = &RingBuffer{
	entries: make([]LogEntry, 0, 500),
	maxSize: 500,
}

func Log(level, category, message string, details ...string) LogEntry {
	id := atomic.AddInt64(&globalBuffer.counter, 1)
	now := time.Now().Format("2006-01-02 15:04:05")

	var det string
	if len(details) > 0 {
		det = details[0]
	}

	entry := LogEntry{
		ID:        id,
		Timestamp: now,
		Level:     level,
		Category:  category,
		Message:   message,
		Details:   det,
	}

	globalBuffer.mu.Lock()
	if len(globalBuffer.entries) >= globalBuffer.maxSize {
		// Drop oldest
		globalBuffer.entries = globalBuffer.entries[1:]
	}
	globalBuffer.entries = append(globalBuffer.entries, entry)
	globalBuffer.mu.Unlock()

	// Also output to console
	log.Printf("[%s] [%s] %s", level, category, message)

	return entry
}

func Info(category, message string, details ...string) LogEntry {
	return Log("INFO", category, message, details...)
}

func Infof(category, format string, a ...interface{}) LogEntry {
	return Log("INFO", category, fmt.Sprintf(format, a...))
}

func Warn(category, message string, details ...string) LogEntry {
	return Log("WARN", category, message, details...)
}

func Warnf(category, format string, a ...interface{}) LogEntry {
	return Log("WARN", category, fmt.Sprintf(format, a...))
}

func Error(category, message string, details ...string) LogEntry {
	return Log("ERROR", category, message, details...)
}

func Errorf(category, format string, a ...interface{}) LogEntry {
	return Log("ERROR", category, fmt.Sprintf(format, a...))
}

func Success(category, message string, details ...string) LogEntry {
	return Log("SUCCESS", category, message, details...)
}

func Successf(category, format string, a ...interface{}) LogEntry {
	return Log("SUCCESS", category, fmt.Sprintf(format, a...))
}

func GetEntries(limit int) []LogEntry {
	globalBuffer.mu.RLock()
	defer globalBuffer.mu.RUnlock()

	n := len(globalBuffer.entries)
	if limit <= 0 || limit > n {
		limit = n
	}

	result := make([]LogEntry, limit)
	copy(result, globalBuffer.entries[n-limit:])
	return result
}

func Clear() {
	globalBuffer.mu.Lock()
	defer globalBuffer.mu.Unlock()
	globalBuffer.entries = make([]LogEntry, 0, globalBuffer.maxSize)
}
