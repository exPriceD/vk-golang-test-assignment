package worker

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
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
	logs      []logEntry
	mu        sync.Mutex
	zapLogger *zap.Logger
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
	m.zapLogger.Debug(msg, fields...)
}

func (m *mockLogger) Info(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "INFO", message: msg, fields: fields})
	m.mu.Unlock()
	m.zapLogger.Info(msg, fields...)
}

func (m *mockLogger) Warn(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "WARN", message: msg, fields: fields})
	m.mu.Unlock()
	m.zapLogger.Warn(msg, fields...)
}

func (m *mockLogger) Error(msg string, fields ...zap.Field) {
	m.mu.Lock()
	m.logs = append(m.logs, logEntry{level: "ERROR", message: msg, fields: fields})
	m.mu.Unlock()
	m.zapLogger.Error(msg, fields...)
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
	taskTimeout := 100 * time.Millisecond
	tests := []struct {
		name           string
		tasks          []interfaces.Task
		ctxFunc        func() context.Context
		expectExit     bool
		wantLogs       []logEntry
		workerID       int32
		taskProcessing time.Duration
	}{
		{
			name: "SuccessfulTask",
			tasks: []interfaces.Task{
				&mockTask{processFunc: func(ctx context.Context) error { return nil }},
			},
			ctxFunc:    func() context.Context { return context.Background() },
			expectExit: false,
			workerID:   1,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 1)}},
				{level: "INFO", message: "Воркер обрабатывает задачу", fields: []zap.Field{zap.Int32("worker_id", 1)}},
			},
		},
		{
			name: "TaskWithError",
			tasks: []interfaces.Task{
				&mockTask{processFunc: func(ctx context.Context) error { return errors.New("task error") }},
			},
			ctxFunc:    func() context.Context { return context.Background() },
			expectExit: false,
			workerID:   2,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 2)}},
				{level: "INFO", message: "Воркер обрабатывает задачу", fields: []zap.Field{zap.Int32("worker_id", 2)}},
				{level: "ERROR", message: "Ошибка обработки задачи", fields: []zap.Field{zap.Error(errors.New("task error"))}},
			},
		},
		{
			name: "TaskTimeout",
			tasks: []interfaces.Task{
				&mockTask{processFunc: func(ctx context.Context) error {
					time.Sleep(200 * time.Millisecond)
					return nil
				}},
			},
			ctxFunc:    func() context.Context { return context.Background() },
			expectExit: false,
			workerID:   3,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 3)}},
				{level: "INFO", message: "Воркер обрабатывает задачу", fields: []zap.Field{zap.Int32("worker_id", 3)}},
				{level: "ERROR", message: "Таймаут обработки задачи", fields: []zap.Field{zap.Int32("worker_id", 3)}},
			},
		},
		{
			name:       "NilTask",
			tasks:      []interfaces.Task{nil},
			ctxFunc:    func() context.Context { return context.Background() },
			expectExit: true,
			workerID:   4,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 4)}},
				{level: "DEBUG", message: "Воркер остановлен по сигнальной задаче", fields: []zap.Field{zap.Int32("worker_id", 4)}},
			},
		},
		{
			name:       "ClosedChannel",
			tasks:      []interfaces.Task{},
			ctxFunc:    func() context.Context { return context.Background() },
			expectExit: true,
			workerID:   5,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 5)}},
				{level: "DEBUG", message: "Канал задач закрыт, воркер остановлен", fields: []zap.Field{zap.Int32("worker_id", 5)}},
			},
		},
		{
			name:  "ContextCancelled",
			tasks: []interfaces.Task{},
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			expectExit: true,
			workerID:   6,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 6)}},
				{level: "DEBUG", message: "Воркер остановлен по контексту", fields: []zap.Field{zap.Int32("worker_id", 6)}},
			},
		},
		{
			name: "MultipleTasks",
			tasks: []interfaces.Task{
				&mockTask{processFunc: func(ctx context.Context) error { return nil }},
				&mockTask{processFunc: func(ctx context.Context) error { return errors.New("task error") }},
			},
			ctxFunc:    func() context.Context { return context.Background() },
			expectExit: false,
			workerID:   7,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 7)}},
				{level: "INFO", message: "Воркер обрабатывает задачу", fields: []zap.Field{zap.Int32("worker_id", 7)}},
				{level: "INFO", message: "Воркер обрабатывает задачу", fields: []zap.Field{zap.Int32("worker_id", 7)}},
				{level: "ERROR", message: "Ошибка обработки задачи", fields: []zap.Field{zap.Error(errors.New("task error"))}},
			},
		},
		{
			name: "FastTask",
			tasks: []interfaces.Task{
				&mockTask{processFunc: func(ctx context.Context) error { return nil }},
			},
			ctxFunc:        func() context.Context { return context.Background() },
			expectExit:     false,
			workerID:       8,
			taskProcessing: 0,
			wantLogs: []logEntry{
				{level: "DEBUG", message: "Воркер запущен", fields: []zap.Field{zap.Int32("worker_id", 8)}},
				{level: "INFO", message: "Воркер обрабатывает задачу", fields: []zap.Field{zap.Int32("worker_id", 8)}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{zapLogger: zaptest.NewLogger(t)}
			worker := NewDefaultWorker(taskTimeout)
			taskChan := make(chan interfaces.Task, len(tt.tasks))
			ctx := tt.ctxFunc()

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				worker.Run(ctx, taskChan, tt.workerID, logger)
				wg.Done()
			}()

			// Отправляем задачи
			for _, task := range tt.tasks {
				taskChan <- task
			}
			if len(tt.tasks) == 0 && tt.name == "ClosedChannel" {
				close(taskChan)
			}

			// Ждем завершения воркера или таймаута
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				if !tt.expectExit {
					t.Errorf("Воркер завершился неожиданно")
				}
			case <-time.After(500 * time.Millisecond):
				if tt.expectExit {
					t.Errorf("Воркер не завершился, как ожидалось")
				}
			}

			// Проверяем логи
			logger.mu.Lock()
			assert.Equal(t, len(tt.wantLogs), len(logger.logs), "Неверное количество логов")
			for i, wantLog := range tt.wantLogs {
				if i < len(logger.logs) {
					assert.Equal(t, wantLog.level, logger.logs[i].level, "Неверный уровень лога на позиции %d", i)
					assert.Equal(t, wantLog.message, logger.logs[i].message, "Неверное сообщение лога на позиции %d", i)
					if len(wantLog.fields) > 0 {
						for j, wantField := range wantLog.fields {
							if j < len(logger.logs[i].fields) {
								assert.Equal(t, wantField, logger.logs[i].fields[j], "Неверное поле лога на позиции %d", i)
							}
						}
					}
				}
			}
			logger.mu.Unlock()
		})
	}
}
