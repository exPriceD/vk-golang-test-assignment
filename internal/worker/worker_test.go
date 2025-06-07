package worker

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"sync"
	"testing"
	"time"
	"vk-worker-pool/internal/interfaces"
)

// mockTask — мок для интерфейса Task, позволяющий задавать поведение Process.
type mockTask struct {
	processFunc func(ctx context.Context) error
}

func (m *mockTask) Process(ctx context.Context) error {
	if m.processFunc != nil {
		return m.processFunc(ctx)
	}
	return nil
}

// mockLogger — мок для интерфейса Logger, чтобы проверять вызовы методов логирования.
type mockLogger struct {
	logs []logEntry
	mu   sync.Mutex
}

type logEntry struct {
	level   string
	message string
	fields  []zap.Field
}

func (m *mockLogger) Debug(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "DEBUG", message: msg, fields: fields})
	m.mu.Unlock()
}

func (m *mockLogger) Info(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "INFO", message: msg, fields: fields})
	m.mu.Unlock()
}

func (m *mockLogger) Warn(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "WARN", message: msg, fields: fields})
	m.mu.Unlock()
}

func (m *mockLogger) Error(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "ERROR", message: msg, fields: fields})
	m.mu.Unlock()
}

// containsLog проверяет, есть ли лог с заданным уровнем и сообщением.
func (m *mockLogger) containsLog(level, message string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, log := range m.logs {
		if log.level == level && log.message == message {
			return true
		}
	}
	return false
}

// TestNewDefaultWorker проверяет создание DefaultWorker с разными значениями taskTimeout.
func TestNewDefaultWorker(t *testing.T) {
	tests := []struct {
		name        string
		taskTimeout time.Duration
		wantTimeout time.Duration
	}{
		{
			name:        "PositiveTimeout",
			taskTimeout: 5 * time.Second,
			wantTimeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := NewDefaultWorker(tt.taskTimeout)
			assert.Equal(t, tt.wantTimeout, worker.TaskTimeout)
		})
	}
}

// TestDefaultWorker_Run проверяет поведение метода Run в различных сценариях.
func TestDefaultWorker_Run(t *testing.T) {
	tests := []struct {
		name          string
		taskTimeout   time.Duration
		setupCtx      func() (context.Context, context.CancelFunc)
		tasks         []interfaces.Task
		closeTasks    bool
		wantLogs      []struct{ level, message string }
		wantCompleted bool
	}{
		{
			name:        "StopByContextCancel",
			taskTimeout: 50 * time.Millisecond,
			setupCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			tasks: []interfaces.Task{},
			wantLogs: []struct{ level, message string }{
				{"DEBUG", "Воркер запущен"},
				{"DEBUG", "Воркер остановлен по контексту"},
			},
			wantCompleted: true,
		},
		{
			name:        "StopByClosedTasksChannel",
			taskTimeout: 50 * time.Millisecond,
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			tasks:      []interfaces.Task{},
			closeTasks: true,
			wantLogs: []struct{ level, message string }{
				{"DEBUG", "Воркер запущен"},
				{"DEBUG", "Канал задач закрыт, воркер остановлен"},
			},
			wantCompleted: true,
		},
		{
			name:        "StopByNilTask",
			taskTimeout: 50 * time.Millisecond,
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			tasks: []interfaces.Task{nil},
			wantLogs: []struct{ level, message string }{
				{"DEBUG", "Воркер запущен"},
				{"DEBUG", "Воркер остановлен по сигнальной задаче"},
			},
			wantCompleted: true,
		},
		{
			name:        "ProcessSuccessfulTask",
			taskTimeout: 50 * time.Millisecond,
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			tasks: []interfaces.Task{
				&mockTask{
					processFunc: func(ctx context.Context) error {
						return nil
					},
				},
				nil, // Для остановки воркера
			},
			wantLogs: []struct{ level, message string }{
				{"DEBUG", "Воркер запущен"},
				{"INFO", "Воркер обрабатывает задачу"},
				{"DEBUG", "Воркер остановлен по сигнальной задаче"},
			},
			wantCompleted: true,
		},
		{
			name:        "ProcessTaskWithError",
			taskTimeout: 50 * time.Millisecond,
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			tasks: []interfaces.Task{
				&mockTask{
					processFunc: func(ctx context.Context) error {
						return errors.New("task error")
					},
				},
				nil,
			},
			wantLogs: []struct{ level, message string }{
				{"DEBUG", "Воркер запущен"},
				{"INFO", "Воркер обрабатывает задачу"},
				{"ERROR", "Ошибка обработки задачи"},
				{"DEBUG", "Воркер остановлен по сигнальной задаче"},
			},
			wantCompleted: true,
		},
		{
			name:        "ProcessTaskTimeout",
			taskTimeout: 10 * time.Millisecond,
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			tasks: []interfaces.Task{
				&mockTask{
					processFunc: func(ctx context.Context) error {
						time.Sleep(50 * time.Millisecond)
						return nil
					},
				},
				nil,
			},
			wantLogs: []struct{ level, message string }{
				{"DEBUG", "Воркер запущен"},
				{"INFO", "Воркер обрабатывает задачу"},
				{"ERROR", "Таймаут обработки задачи"},
				{"DEBUG", "Воркер остановлен по сигнальной задаче"},
			},
			wantCompleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := NewDefaultWorker(tt.taskTimeout)
			tasksChan := make(chan interfaces.Task, len(tt.tasks))
			ctx, cancel := tt.setupCtx()
			defer cancel()

			for _, task := range tt.tasks {
				tasksChan <- task
			}
			if tt.closeTasks {
				close(tasksChan)
			}

			done := make(chan struct{})
			go func() {
				worker.Run(ctx, tasksChan, 1, logger)
				close(done)
			}()

			select {
			case <-done:
				if !tt.wantCompleted {
					t.Error("Воркер неожиданно завершил работу")
				}
			case <-time.After(100 * time.Millisecond):
				if tt.wantCompleted {
					t.Fatal("Воркер не завершил работу вовремя")
				}
			}

			for _, wantLog := range tt.wantLogs {
				assert.True(t, logger.containsLog(wantLog.level, wantLog.message),
					"Expected log: level=%s, message=%s", wantLog.level, wantLog.message)
			}
		})
	}
}
