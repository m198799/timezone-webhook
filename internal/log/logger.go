// Package log ...
package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const callerSkip = 1

var l logger

type logger struct {
	logger *zap.SugaredLogger
}

// Info log info
func Info(args ...interface{}) {
	l.logger.Info(args...)
}

// Warn log warn
func Warn(args ...interface{}) {
	l.logger.Warn(args...)
}

// Debug log debug
func Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

// Error log error
func Error(args ...interface{}) {
	l.logger.Error(args...)
}

// Infow log info
func Infow(msg string, kv ...interface{}) {
	l.logger.Infow(msg, kv...)
}

// Warn log warning
func Warnw(msg string, kv ...interface{}) {
	l.logger.Warnw(msg, kv...)
}

// Error log error
func Errorw(msg string, kv ...interface{}) {
	l.logger.Errorw(msg, kv...)
}

// Infof log info template
func Infof(template string, args ...interface{}) {
	l.logger.Infof(template, args...)
}

// Debugf log debug template
func Debugf(template string, args ...interface{}) {
	l.logger.Debugf(template, args...)
}

// Warnf log warning template
func Warnf(template string, args ...interface{}) {
	l.logger.Warnf(template, args...)
}

// Errorf log error template
func Errorf(template string, args ...interface{}) {
	l.logger.Errorf(template, args...)
}

// Flush log flush
func Flush() error {
	return l.logger.Sync()
}

func init() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	zapLog, _ := config.Build(zap.AddCaller(), zap.AddCallerSkip(callerSkip)) //nolint:errcheck
	l = logger{
		logger: zapLog.Sugar(),
	}
}
