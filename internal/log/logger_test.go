package log

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func setup() *observer.ObservedLogs {
	core, recorded := observer.New(zap.DebugLevel)
	l.logger = zap.New(core).Sugar()
	return recorded
}

func TestInfo(t *testing.T) {
	logs := setup()

	Infof("this is an info message")
	if logs.Len() != 1 {
		t.Errorf("expected 1 log entry, got %d", logs.Len())
	}

	if logs.All()[0].Level != zap.InfoLevel {
		t.Errorf("expected level %v, got %v", zap.InfoLevel, logs.All()[0].Level)
	}

	if logs.All()[0].Message != "this is an info message" {
		t.Errorf("expected message to be 'this is an info message', got '%s'", logs.All()[0].Message)
	}
}

func TestDebug(t *testing.T) {
	logs := setup()

	Debugf("this is a debug message")
	if logs.Len() != 1 {
		t.Errorf("expected 1 log entry, got %d", logs.Len())
	}
}

func TestWarn(t *testing.T) {
	logs := setup()

	Warnf("this is a warning message")
	if logs.Len() != 1 {
		t.Errorf("expected 1 log entry, got %d", logs.Len())
	}
}

func TestError(t *testing.T) {
	logs := setup()

	Errorf("this is an error message")
	if logs.Len() != 1 {
		t.Errorf("expected 1 log entry, got %d", logs.Len())
	}
}

func TestFlush(t *testing.T) {
	setup()

	err := Flush()
	if err != nil {
		t.Errorf("expected no error on flush, got %v", err)
	}
}
