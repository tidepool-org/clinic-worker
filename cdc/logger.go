package cdc

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log/slog"
)

type AsynceventsLoggerAdapter struct {
	*zap.SugaredLogger
}

var logLevels map[slog.Level]zapcore.Level = map[slog.Level]zapcore.Level{
	slog.LevelDebug: zap.DebugLevel,
	slog.LevelInfo:  zap.InfoLevel,
	slog.LevelWarn:  zap.WarnLevel,
	slog.LevelError: zap.ErrorLevel,
}

func (a *AsynceventsLoggerAdapter) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	a.Logw(logLevels[level], msg, args...)
}
