package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"vk-worker-pool/internal/constants"
)

// Config содержит конфиг для WorkerPool.
type Config struct {
	Worker WorkerConfig `yaml:"worker"`
	Logger LoggerConfig `yaml:"logger"`
}

// WorkerConfig содержит конфиг для Worker.
type WorkerConfig struct {
	InitialWorkers int `yaml:"initial_workers"`
	TaskBufferSize int `yaml:"task_buffer_size"`
}

// LoggerConfig содержит конфиг для Logger.
type LoggerConfig struct {
	Level             string `yaml:"logging_level"`      // Уровень логгирования
	Format            string `yaml:"format"`             // Формат логов (json, text)
	DisableStacktrace bool   `yaml:"disable_stacktrace"` // Вывод стека вызовов (stack trace) при логировании ошибок
	DisableCaller     bool   `yaml:"disable_caller"`     // Вывод информации о том, откуда был вызван метод логирования
}

// DefaultConfig возвращает конфиг по умолчанию.
func DefaultConfig() Config {
	return Config{
		Worker: WorkerConfig{
			InitialWorkers: 3,
			TaskBufferSize: 10,
		},
		Logger: LoggerConfig{
			Level:             constants.LogLevelInfo,
			Format:            constants.LogFormatText,
			DisableStacktrace: false,
			DisableCaller:     false,
		},
	}
}

// Validate проверяет корректность конфига.
func (c Config) Validate() error {
	if c.Worker.InitialWorkers < 0 {
		return fmt.Errorf("InitialWorkers не может быть отрицательным: %d", c.Worker.InitialWorkers)
	}

	if c.Worker.TaskBufferSize < 0 {
		return fmt.Errorf("TaskBufferSize не может быть отрицательным: %d", c.Worker.TaskBufferSize)
	}

	switch c.Logger.Level {
	case constants.LogLevelDebug, constants.LogLevelInfo, constants.LogLevelWarn, constants.LogLevelError:
	default:
		return fmt.Errorf("неподдерживаемый уровень логирования: %s", c.Logger.Level)
	}

	switch c.Logger.Format {
	case constants.LogFormatJSON, constants.LogFormatText:
	default:
		return fmt.Errorf("неподдерживаемый формат логирования: %s", c.Logger.Format)
	}

	return nil
}

// LoadConfig загружает конфиг из YAML-файла или возвращает дефолтную.
func LoadConfig(filePath string) (Config, error) {
	cfg := DefaultConfig()
	if filePath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) || len(data) == 0 {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("ошибка парсинга YAML: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("невалидная конфигурация: %w", err)
	}

	return cfg, nil
}
