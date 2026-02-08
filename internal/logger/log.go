package logger

import (
	"asmroner/internal/consts"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger 结构化日志器，提供层次化、上下文感知的日志输出
type Logger struct {
	mu           sync.Mutex
	errorLogFile *os.File
	errorLogger  *log.Logger
	stdLogger    *log.Logger
	level        Level
}

var defaultLogger *Logger

// Init 初始化全局日志器
func Init() {
	logPath := filepath.Join(consts.MetaDataDir, consts.FailedLogName)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("无法打开错误日志文件 %s: %v", logPath, err)
	}
	defaultLogger = &Logger{
		errorLogFile: f,
		errorLogger:  log.New(f, "", log.Ldate|log.Ltime),
		stdLogger:    log.New(os.Stderr, "", log.Ldate|log.Ltime),
		level:        LevelInfo,
	}
}

// Close 关闭日志文件
func Close() {
	if defaultLogger != nil && defaultLogger.errorLogFile != nil {
		defaultLogger.errorLogFile.Close()
	}
}

// SetLevel 设置日志级别
func SetLevel(l Level) {
	if defaultLogger != nil {
		defaultLogger.level = l
	}
}

// ─── 基础输出 ───────────────────────────────────────────

func output(level Level, icon string, format string, args ...any) {
	if defaultLogger == nil || level < defaultLogger.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	defaultLogger.stdLogger.Printf("%s %s", icon, msg)
}

// ─── 通用日志方法 ────────────────────────────────────────

// Info 输出信息级别的日志
func Info(format string, args ...any) {
	output(LevelInfo, "ℹ️ ", format, args...)
}

// Warn 输出警告级别的日志
func Warn(format string, args ...any) {
	output(LevelWarn, "⚠️ ", format, args...)
}

// Error 输出错误级别的日志，同时写入错误日志文件
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if defaultLogger != nil {
		defaultLogger.stdLogger.Printf("❌ %s", msg)
		defaultLogger.mu.Lock()
		defaultLogger.errorLogger.Printf("ERROR %s", msg)
		defaultLogger.mu.Unlock()
	}
}

// Debug 输出调试级别的日志
func Debug(format string, args ...any) {
	output(LevelDebug, "🔍", format, args...)
}

// ─── 带上下文的结构化日志 ─────────────────────────────────

// TaskContext 任务上下文日志器，为一组相关操作提供统一的前缀
type TaskContext struct {
	prefix string
}

// NewTask 创建一个带有任务前缀的上下文日志器
// 例如: NewTask("RJ356801") => 所有日志带 [RJ356801] 前缀
func NewTask(id string) *TaskContext {
	return &TaskContext{prefix: fmt.Sprintf("[%s]", id)}
}

// Info 带上下文的 Info
func (t *TaskContext) Info(format string, args ...any) {
	Info("%s %s", t.prefix, fmt.Sprintf(format, args...))
}

// Warn 带上下文的 Warn
func (t *TaskContext) Warn(format string, args ...any) {
	Warn("%s %s", t.prefix, fmt.Sprintf(format, args...))
}

// Error 带上下文的 Error
func (t *TaskContext) Error(format string, args ...any) {
	Error("%s %s", t.prefix, fmt.Sprintf(format, args...))
}

// Debug 带上下文的 Debug
func (t *TaskContext) Debug(format string, args ...any) {
	Debug("%s %s", t.prefix, fmt.Sprintf(format, args...))
}

// ─── 阶段性日志 ──────────────────────────────────────────

// Step 输出流程步骤日志，用于标记关键操作节点
func Step(format string, args ...any) {
	output(LevelInfo, "▶ ", format, args...)
}

// Done 输出完成日志
func Done(format string, args ...any) {
	output(LevelInfo, "✅", format, args...)
}

// Fail 输出失败日志，同时写入错误日志文件
func Fail(format string, args ...any) {
	Error(format, args...)
}

// Progress 输出进度日志
func Progress(current, total int, format string, args ...any) {
	pct := float64(current) / float64(total) * 100
	msg := fmt.Sprintf(format, args...)
	output(LevelInfo, "📊", "%s (%.1f%% — %d/%d)", msg, pct, current, total)
}

// ─── 下载专用日志 ─────────────────────────────────────────

// DownloadStart 下载开始
func DownloadStart(ids []string, dir string) {
	output(LevelInfo, "📥", "正在下载以下资源: %v", ids)
	output(LevelInfo, "📂", "保存路径: %s", dir)
}

// DownloadRetry 下载重试
func DownloadRetry(attempt, maxRetries int, target string, err error) {
	output(LevelWarn, "🔄", "重试下载 (%d/%d): %s — 原因: %s", attempt, maxRetries, target, SummarizeError(err))
}

// ─── 错误日志文件记录 ─────────────────────────────────────

// RecordFailure 线程安全地记录错误到日志文件
func RecordFailure(id string, url string, errMsg string) {
	if defaultLogger == nil {
		return
	}
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.errorLogger.Printf("[%s] %s | Reason: %s", id, url, errMsg)
}

// ─── 辅助函数 ─────────────────────────────────────────────

// SummarizeError 从错误链中提取用户友好的摘要
func SummarizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	// HTTP/2 stream error
	if strings.Contains(msg, "stream error") {
		return fmt.Sprintf("网络连接中断 (HTTP/2 stream error)，可能原因: 服务器过载/连接超时/CDN 限制。原始错误: %s", msg)
	}
	// context deadline exceeded
	if strings.Contains(msg, "context deadline exceeded") {
		return "请求超时，服务器响应时间过长"
	}
	// connection refused
	if strings.Contains(msg, "connection refused") {
		return "连接被拒绝，服务器未响应或端口未开放"
	}
	// DNS resolution
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup") {
		return fmt.Sprintf("DNS 解析失败: %s", msg)
	}
	// TLS
	if strings.Contains(msg, "tls") || strings.Contains(msg, "certificate") {
		return fmt.Sprintf("TLS/证书错误: %s", msg)
	}
	// EOF
	if strings.Contains(msg, "EOF") || strings.Contains(msg, "unexpected EOF") {
		return "连接意外关闭 (EOF)，服务器提前断开了连接"
	}
	// i/o timeout
	if strings.Contains(msg, "i/o timeout") {
		return "网络 I/O 超时，连接可能不稳定"
	}
	return msg
}
