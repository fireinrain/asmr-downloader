package logger

import (
	"log"
	"os"
	"sync"
)

var (
	errorLogFile *os.File
	ErrorLogger  *log.Logger
	mu           sync.Mutex
)

func InitErrorLogger() {
	var err error
	// 以追加模式打开，如果没有则创建
	errorLogFile, err = os.OpenFile("download_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	ErrorLogger = log.New(errorLogFile, "ERROR: ", log.Ldate|log.Ltime)
}

// RecordFailure 线程安全地记录错误
func RecordFailure(id string, url string, errMsg string) {
	mu.Lock()
	defer mu.Unlock()
	// 格式：ID | URL | 错误原因
	ErrorLogger.Printf("[%s] %s | Reason: %s\n", id, url, errMsg)
}

func Close() {
	if errorLogFile != nil {
		errorLogFile.Close()
	}
}
