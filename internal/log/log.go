package log

import (
	"os"
	"github.com/paularlott/logger"
	logslog "github.com/paularlott/logger/slog"
)

var defaultLogger logger.Logger

func init() {
	defaultLogger = logslog.New(logslog.Config{
		Level:  "info",
		Format: "console",
		Writer: os.Stdout,
	})
}

func Configure(level, format string) {
	defaultLogger = logslog.New(logslog.Config{
		Level:  level,
		Format: format,
		Writer: os.Stdout,
	})
}

func Info(msg string, keysAndValues ...any) {
	defaultLogger.Info(msg, keysAndValues...)
}

func Error(msg string, keysAndValues ...any) {
	defaultLogger.Error(msg, keysAndValues...)
}

func Debug(msg string, keysAndValues ...any) {
	defaultLogger.Debug(msg, keysAndValues...)
}