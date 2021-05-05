package worker

import "go.uber.org/zap"

func newLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	return config.Build()
}

func getSuggaredLogger(logger *zap.Logger) *zap.SugaredLogger {
	return logger.Sugar()
}
