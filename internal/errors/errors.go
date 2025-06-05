package errors

import (
	"errors"
	"fmt"
	"time"
)

// Ошибки для конфигурации
var (
	ErrNegativeInitialWorkers = errors.New("InitialWorkers не может быть отрицательным")
	ErrNegativeTaskBufferSize = errors.New("TaskBufferSize не может быть отрицательным")
	ErrInvalidLogLevel        = errors.New("неподдерживаемый уровень логирования")
	ErrInvalidLogFormat       = errors.New("неподдерживаемый формат логирования")
	ErrReadConfigFile         = errors.New("ошибка чтения файла конфигурации")
	ErrParseYAML              = errors.New("ошибка парсинга YAML")
	ErrInvalidConfig          = errors.New("невалидная конфигурация")
	ErrNegativeTaskTimeout    = errors.New("TaskTimeout не может быть отрицательным")
	ErrNegativePoolTimeout    = errors.New("PoolTimeout не может быть отрицательным")
)

// Ошибки для логгера
var (
	ErrLoggerInit = errors.New("ошибка инициализации логгера")
)

// Ошибки для WorkerPool
var (
	ErrPoolStopped = errors.New("пул воркеров остановлен")
	ErrNilTask     = errors.New("задача не может быть nil")
	ErrTimeout     = errors.New("превышен таймаут")
)

// WrapInvalidLogLevel оборачивает ErrInvalidLogLevel с указанием уровня.
func WrapInvalidLogLevel(level string) error {
	return fmt.Errorf("%w: %s", ErrInvalidLogLevel, level)
}

// WrapInvalidLogFormat оборачивает ErrInvalidLogFormat с указанием формата.
func WrapInvalidLogFormat(format string) error {
	return fmt.Errorf("%w: %s", ErrInvalidLogFormat, format)
}

// WrapNegativeInitialWorkers оборачивает ErrNegativeInitialWorkers с указанием значения.
func WrapNegativeInitialWorkers(workers int) error {
	return fmt.Errorf("%w: %d", ErrNegativeInitialWorkers, workers)
}

// WrapNegativeTaskBufferSize оборачивает ErrNegativeTaskBufferSize с указанием значения.
func WrapNegativeTaskBufferSize(size int) error {
	return fmt.Errorf("%w: %d", ErrNegativeTaskBufferSize, size)
}

// WrapNegativeTaskTimeout оборачивает ErrNegativeTaskTimeout с указанием значения.
func WrapNegativeTaskTimeout(timeout time.Duration) error {
	return fmt.Errorf("%w: %v", ErrNegativeTaskTimeout, timeout)
}

// WrapNegativePoolTimeout оборачивает ErrNegativePoolTimeout с указанием значения.
func WrapNegativePoolTimeout(timeout time.Duration) error {
	return fmt.Errorf("%w: %v", ErrNegativePoolTimeout, timeout)
}
