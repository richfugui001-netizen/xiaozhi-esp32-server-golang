package logger

import (
	"xiaozhi-esp32-server-golang/logger"
)

// Logger 日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// SimpleLogger 简单日志实现，包装现有日志系统
type SimpleLogger struct{}

// NewSimpleLogger 创建简单日志实例
func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{}
}

func (l *SimpleLogger) Debug(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func (l *SimpleLogger) Info(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func (l *SimpleLogger) Error(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}
