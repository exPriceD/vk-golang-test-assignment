package config_test

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
	"vk-worker-pool/internal/config"
	"vk-worker-pool/internal/constants"
	"vk-worker-pool/internal/errors"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr error
	}{
		{
			name: "Negative InitialWorkers",
			cfg: config.Config{
				Worker: config.WorkerConfig{InitialWorkers: -1},
			},
			wantErr: errors.WrapNegativeInitialWorkers(-1),
		},
		{
			name: "Negative TaskBufferSize",
			cfg: config.Config{
				Worker: config.WorkerConfig{TaskBufferSize: -1},
			},
			wantErr: errors.WrapNegativeTaskBufferSize(-1),
		},
		{
			name: "Negative TaskTimeout",
			cfg: config.Config{
				Worker: config.WorkerConfig{TaskTimeout: -1},
			},
			wantErr: errors.WrapNegativeTaskTimeout(-1),
		},
		{
			name: "Negative PoolTimeout",
			cfg: config.Config{
				Worker: config.WorkerConfig{PoolTimeout: -1},
			},
			wantErr: errors.WrapNegativePoolTimeout(-1),
		},
		{
			name: "Invalid Logger Level",
			cfg: config.Config{
				Logger: config.LoggerConfig{Level: "invalid"},
			},
			wantErr: errors.WrapInvalidLogLevel("invalid"),
		},
		{
			name: "Invalid Logger Format",
			cfg: config.Config{
				Logger: config.LoggerConfig{Level: "INFO", Format: "invalid"},
			},
			wantErr: errors.WrapInvalidLogFormat("invalid"),
		},
		{
			name:    "Valid Config",
			cfg:     config.DefaultConfig(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("Empty file path", func(t *testing.T) {
		cfg, err := config.LoadConfig("")
		assert.NoError(t, err)
		assert.Equal(t, config.DefaultConfig(), cfg)
	})

	t.Run("File does not exist", func(t *testing.T) {
		cfg, err := config.LoadConfig("nonexistent.yaml")
		assert.NoError(t, err)
		assert.Equal(t, config.DefaultConfig(), cfg)
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		filePath := "invalid.yaml"
		createTempFile(t, filePath, "invalid_yaml: :")
		defer func(name string) {
			_ = os.Remove(name)
		}(filePath)

		_, err := config.LoadConfig(filePath)
		assert.ErrorIs(t, err, errors.ErrParseYAML)
	})

	t.Run("Valid YAML but invalid config", func(t *testing.T) {
		filePath := "invalid_config.yaml"
		createTempFile(t, filePath, "worker:\n  initial_workers: -1")
		defer func(name string) {
			_ = os.Remove(name)
		}(filePath)

		_, err := config.LoadConfig(filePath)
		assert.ErrorIs(t, err, errors.ErrInvalidConfig)
	})

	t.Run("Valid YAML and valid config", func(t *testing.T) {
		filePath := "valid_config.yaml"
		createTempFile(t, filePath, `
worker:
  initial_workers: 5
  task_buffer_size: 20
  task_timeout: 2s
  pool_timeout: 3s
logger:
  logging_level: "INFO"
  format: "text"
  disable_stacktrace: false
  disable_caller: false
`)
		defer func(name string) {
			_ = os.Remove(name)
		}(filePath)

		cfg, err := config.LoadConfig(filePath)
		assert.NoError(t, err)
		assert.Equal(t, 5, cfg.Worker.InitialWorkers)
		assert.Equal(t, 20, cfg.Worker.TaskBufferSize)
		assert.Equal(t, 2*time.Second, cfg.Worker.TaskTimeout)
		assert.Equal(t, 3*time.Second, cfg.Worker.PoolTimeout)
		assert.Equal(t, constants.LogLevelInfo, cfg.Logger.Level)
		assert.Equal(t, constants.LogFormatText, cfg.Logger.Format)
	})
}

// createTempFile вспомогательная функция для создания временного файла.
func createTempFile(t *testing.T, filePath, content string) {
	t.Helper()
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("ошибка создания временного файла: %v", err)
	}
}
