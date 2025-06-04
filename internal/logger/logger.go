package logger

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"vk-worker-pool/internal/config"
	"vk-worker-pool/internal/constants"
)

// NewLogger создает новый логгер на основе конфига.
func NewLogger(loggerCfg config.LoggerConfig) (*zap.Logger, error) {
	level, err := getLogLevel(loggerCfg.Level)
	if err != nil {
		return nil, err
	}

	var zapCfg zap.Config
	if loggerCfg.Format == constants.LogFormatJSON {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.DisableStacktrace = loggerCfg.DisableStacktrace
	zapCfg.DisableCaller = loggerCfg.DisableCaller

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации логгера: %w", err)
	}

	return logger, nil
}

func getLogLevel(level string) (zapcore.Level, error) {
	switch level {
	case constants.LogLevelDebug:
		return zapcore.DebugLevel, nil
	case constants.LogLevelInfo:
		return zapcore.InfoLevel, nil
	case constants.LogLevelWarn:
		return zapcore.WarnLevel, nil
	case constants.LogLevelError:
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("неподдерживаемый уровень логирования: %s", level)
	}
}
