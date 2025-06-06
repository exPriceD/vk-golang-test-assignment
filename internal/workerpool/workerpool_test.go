package workerpool

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"vk-worker-pool/internal/errors"
	"vk-worker-pool/internal/interfaces"
)

// mockWorker — мок для интерфейса Worker.
type mockWorker struct {
	mu         sync.Mutex
	runs       []runCall
	workerChan chan struct{} // Для синхронизации завершения воркеров
	runFunc    func(ctx context.Context, tasks <-chan interfaces.Task, id int32, logger interfaces.Logger)
}

type runCall struct {
	id     int32
	logger interfaces.Logger
}

func (mw *mockWorker) Run(ctx context.Context, tasks <-chan interfaces.Task, id int32, logger interfaces.Logger) {
	mw.mu.Lock()
	mw.runs = append(mw.runs, runCall{id: id, logger: logger})
	mw.mu.Unlock()

	if mw.runFunc != nil {
		mw.runFunc(ctx, tasks, id, logger)
		return
	}

	for {
		select {
		case <-ctx.Done():
			if mw.workerChan != nil {
				mw.workerChan <- struct{}{}
			}
			return
		case task, ok := <-tasks:
			if !ok || task == nil {
				if mw.workerChan != nil {
					mw.workerChan <- struct{}{}
				}
				return
			}

			err := task.Process(ctx)
			if err != nil {
				return
			}
		}
	}
}

func (mw *mockWorker) SlowRun(ctx context.Context, tasks <-chan interfaces.Task, id int32, logger interfaces.Logger) {
	mw.mu.Lock()
	mw.runs = append(mw.runs, runCall{id: id, logger: logger})
	mw.mu.Unlock()
	// Имитация медленного воркера
	time.Sleep(100 * time.Millisecond)
	mw.workerChan <- struct{}{}
}

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
	if m == nil {
		panic("mockLogger is nil")
	}
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

// TestNewWorkerPool проверяет создание WorkerPool.
func TestNewWorkerPool(t *testing.T) {
	tests := []struct {
		name           string
		initialWorkers int
		taskBufferSize int
		poolTimeout    time.Duration
		wantErr        bool
		wantWorkers    int
		wantLog        logEntry
	}{
		{
			name:           "ValidPool",
			initialWorkers: 2,
			taskBufferSize: 10,
			poolTimeout:    1000 * time.Millisecond,
			wantErr:        false,
			wantWorkers:    2,
			wantLog: logEntry{
				level:   "INFO",
				message: "Создан новый пул воркеров",
				fields:  []zap.Field{zap.Int("initial_workers", 2), zap.Int("task_buffer_size", 10)},
			},
		},
		{
			name:           "ZeroWorkers",
			initialWorkers: 0,
			taskBufferSize: 10,
			poolTimeout:    1000 * time.Millisecond,
			wantErr:        false,
			wantWorkers:    0,
			wantLog: logEntry{
				level:   "INFO",
				message: "Создан новый пул воркеров",
				fields:  []zap.Field{zap.Int("initial_workers", 0), zap.Int("task_buffer_size", 10)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.initialWorkers)}
			pool, err := NewWorkerPool(tt.initialWorkers, tt.taskBufferSize, worker, logger, tt.poolTimeout)
			time.Sleep(200 * time.Millisecond)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
				assert.Equal(t, tt.taskBufferSize, cap(pool.tasks))
				assert.Equal(t, int32(tt.initialWorkers), atomic.LoadInt32(&pool.workerCount))

				assert.Equal(t, tt.wantWorkers, len(worker.runs))

				logger.mu.Lock()
				assert.GreaterOrEqual(t, len(logger.logs), 1)
				assert.Equal(t, tt.wantLog.level, logger.logs[0].level)
				assert.Equal(t, tt.wantLog.message, logger.logs[0].message)
				for i, field := range tt.wantLog.fields {
					assert.Equal(t, field, logger.logs[0].fields[i])
				}
				logger.mu.Unlock()

				pool.Shutdown()
			}
		})
	}
}

// TestAddWorkers проверяет добавление воркеров.
func TestAddWorkers(t *testing.T) {
	tests := []struct {
		name            string
		numWorkers      int
		closed          bool
		wantRuns        int
		wantWorkerCount int32
		wantLog         logEntry
	}{
		{
			name:            "AddValidWorkers",
			numWorkers:      2,
			closed:          false,
			wantRuns:        2,
			wantWorkerCount: 2,
			wantLog: logEntry{
				level:   "INFO",
				message: "Добавлены новые воркеры",
				fields:  []zap.Field{zap.Int("num_workers", 2)},
			},
		},
		{
			name:            "AddZeroWorkers",
			numWorkers:      0,
			closed:          false,
			wantRuns:        0,
			wantWorkerCount: 0,
			wantLog: logEntry{
				level:   "WARN",
				message: "Попытка добавить неположительное количество воркеров",
				fields:  []zap.Field{zap.Int("num_workers", 0)},
			},
		},
		{
			name:            "AddToClosedPool",
			numWorkers:      2,
			closed:          true,
			wantRuns:        0,
			wantWorkerCount: 0,
			wantLog: logEntry{
				level:   "WARN",
				message: "Попытка добавить воркеров в закрытый пул",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.numWorkers)}
			pool, err := NewWorkerPool(0, 10, worker, logger, 1000*time.Millisecond)
			time.Sleep(200 * time.Millisecond)
			assert.NoError(t, err)

			if tt.closed {
				pool.Shutdown()
			}

			pool.AddWorkers(tt.numWorkers)
			time.Sleep(200 * time.Millisecond)
			assert.Equal(t, tt.wantRuns, len(worker.runs))
			assert.Equal(t, tt.wantWorkerCount, atomic.LoadInt32(&pool.workerCount))

			logger.mu.Lock()
			if len(logger.logs) > 1 {
				var log logEntry
				if tt.name == "AddToClosedPool" {
					log = logger.logs[len(logger.logs)-1]
				} else {
					log = logger.logs[1]
				}
				assert.Equal(t, tt.wantLog.level, log.level)
				assert.Equal(t, tt.wantLog.message, log.message)
				if len(tt.wantLog.fields) > 0 {
					for i, field := range tt.wantLog.fields {
						assert.Equal(t, field, log.fields[i])
					}
				}
			}
			logger.mu.Unlock()
			pool.Shutdown()
		})
	}
}

// TestRemoveWorkers проверяет удаление воркеров.
func TestRemoveWorkers(t *testing.T) {
	tests := []struct {
		name            string
		initialWorkers  int
		numWorkers      int
		closed          bool
		fillBuffer      bool
		cancelContext   bool
		wantWorkerCount int32
		wantLog         logEntry
	}{
		{
			name:            "RemoveValidWorkers",
			initialWorkers:  2,
			numWorkers:      1,
			closed:          false,
			wantWorkerCount: 1,
			wantLog: logEntry{
				level:   "INFO",
				message: "Остановлены воркеры",
				fields:  []zap.Field{zap.Int("num_workers", 1)},
			},
		},
		{
			name:            "RemoveZeroWorkers",
			initialWorkers:  2,
			numWorkers:      0,
			closed:          false,
			wantWorkerCount: 2,
			wantLog: logEntry{
				level:   "WARN",
				message: "Попытка удалить неположительное количество воркеров",
				fields:  []zap.Field{zap.Int("num_workers", 0)},
			},
		},
		{
			name:            "RemoveFromClosedPool",
			initialWorkers:  2,
			numWorkers:      1,
			closed:          true,
			wantWorkerCount: 2,
			wantLog: logEntry{
				level:   "WARN",
				message: "Попытка удалить воркеров из закрытого пула",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.initialWorkers)}
			pool, err := NewWorkerPool(tt.initialWorkers, 2, worker, logger, 100*time.Millisecond)
			assert.NoError(t, err)

			if tt.fillBuffer {
				for i := 0; i < 2; i++ {
					pool.tasks <- &mockTask{}
				}
			}

			if tt.cancelContext {
				pool.cancel()
			}

			if tt.closed {
				pool.Shutdown()
			}

			pool.RemoveWorkers(tt.numWorkers)

			// Ожидаем завершения воркеров
			for i := 0; i < tt.initialWorkers-int(tt.wantWorkerCount); i++ {
				select {
				case <-worker.workerChan:
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Воркер не завершился")
				}
			}

			assert.Equal(t, tt.wantWorkerCount, atomic.LoadInt32(&pool.workerCount))

			logger.mu.Lock()
			if len(logger.logs) > 1 { // Пропускаем лог создания пула
				log := logger.logs[len(logger.logs)-1]
				assert.Equal(t, tt.wantLog.level, log.level)
				assert.Equal(t, tt.wantLog.message, log.message)
				if len(tt.wantLog.fields) > 0 {
					for i, field := range tt.wantLog.fields {
						assert.Equal(t, field, log.fields[i])
					}
				}
			}
			logger.mu.Unlock()

			pool.Shutdown()
		})
	}
}

// TestSubmit проверяет отправку задач.
func TestSubmit(t *testing.T) {
	tests := []struct {
		name       string
		task       interfaces.Task
		closed     bool
		fillBuffer bool
		wantErr    error
		wantLog    logEntry
	}{
		{
			name:    "ValidTask",
			task:    &mockTask{},
			closed:  false,
			wantErr: nil,
			wantLog: logEntry{
				level:   "DEBUG",
				message: "Задача отправлена в пул",
			},
		},
		{
			name:    "NilTask",
			task:    nil,
			closed:  false,
			wantErr: errors.ErrNilTask,
			wantLog: logEntry{
				level:   "ERROR",
				message: "Попытка отправить пустую задачу",
			},
		},
		{
			name:    "ClosedPool",
			task:    &mockTask{},
			closed:  true,
			wantErr: errors.ErrPoolStopped,
			wantLog: logEntry{
				level:   "ERROR",
				message: "Попытка отправить задачу в закрытый пул",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, 1)}
			pool, err := NewWorkerPool(1, 2, worker, logger, 100*time.Millisecond)
			time.Sleep(200 * time.Millisecond)
			assert.NoError(t, err)

			if tt.fillBuffer {
				for i := 0; i < 2; i++ {
					pool.tasks <- &mockTask{}
				}
			}

			if tt.closed {
				pool.Shutdown()
			}

			err = pool.Submit(tt.task)
			assert.Equal(t, tt.wantErr, err)

			logger.mu.Lock()
			if len(logger.logs) > 1 {
				log := logger.logs[len(logger.logs)-1]
				assert.Equal(t, tt.wantLog.level, log.level)
				assert.Equal(t, tt.wantLog.message, log.message)
			}
			logger.mu.Unlock()

			pool.Shutdown()
		})
	}
}

// TestShutdown проверяет завершение пула.
func TestShutdown(t *testing.T) {
	tests := []struct {
		name           string
		initialWorkers int
		slowWorkers    bool
		wantLogs       []logEntry
	}{
		{
			name:           "NormalShutdown",
			initialWorkers: 2,
			slowWorkers:    false,
			wantLogs: []logEntry{
				{level: "INFO", message: "Создан новый пул воркеров", fields: []zap.Field{zap.Int("initial_workers", 2), zap.Int("task_buffer_size", 10)}},
				{level: "INFO", message: "Добавлены новые воркеры", fields: []zap.Field{zap.Int("num_workers", 2)}},
				{level: "INFO", message: "Инициирована остановка пула воркеров"},
				{level: "INFO", message: "Все воркеры завершены"},
				{level: "INFO", message: "Пул воркеров успешно остановлен"},
			},
		},
		{
			name:           "TimeoutShutdown",
			initialWorkers: 1,
			slowWorkers:    true,
			wantLogs: []logEntry{
				{level: "INFO", message: "Создан новый пул воркеров", fields: []zap.Field{zap.Int("initial_workers", 1), zap.Int("task_buffer_size", 10)}},
				{level: "INFO", message: "Добавлены новые воркеры", fields: []zap.Field{zap.Int("num_workers", 1)}},
				{level: "INFO", message: "Инициирована остановка пула воркеров"},
				{level: "WARN", message: "Таймаут ожидания завершения воркеров"},
				{level: "INFO", message: "Пул воркеров успешно остановлен"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.initialWorkers)}
			if tt.slowWorkers {
				worker.runFunc = func(ctx context.Context, tasks <-chan interfaces.Task, id int32, logger interfaces.Logger) {
					// Имитация медленного воркера
					time.Sleep(100 * time.Millisecond)
					worker.workerChan <- struct{}{}
				}
			}
			pool, err := NewWorkerPool(tt.initialWorkers, 10, worker, logger, 10*time.Millisecond)
			assert.NoError(t, err)

			pool.Shutdown()

			_, ok := <-pool.tasks
			assert.False(t, ok)

			logger.mu.Lock()
			fmt.Println(logger.logs)
			assert.Equal(t, len(tt.wantLogs), len(logger.logs))
			for i, wantLog := range tt.wantLogs {
				assert.Equal(t, wantLog.level, logger.logs[i].level)
				assert.Equal(t, wantLog.message, logger.logs[i].message)
				if len(wantLog.fields) > 0 {
					for j, field := range wantLog.fields {
						assert.Equal(t, field, logger.logs[i].fields[j])
					}
				}
			}
			logger.mu.Unlock()

			pool.Shutdown()
			logger.mu.Lock()
			if len(logger.logs) > len(tt.wantLogs) {
				lastLog := logger.logs[len(logger.logs)-1]
				assert.Equal(t, "WARN", lastLog.level)
				assert.Equal(t, "Пул уже остановлен", lastLog.message)
			}
			logger.mu.Unlock()
		})
	}
}
