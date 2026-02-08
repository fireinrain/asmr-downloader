package logger

import (
	"asmroner/internal/consts"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	errorLogFile *os.File
	ErrorLogger  *log.Logger
	mu           sync.Mutex
)

func InitErrorLogger() {
	var err error
	logPath := filepath.Join(consts.MetaDataDir, consts.FailedLogName)
	errorLogFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
