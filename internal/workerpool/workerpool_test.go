package workerpool

import (
	"context"
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
	runs       []int32
	workerChan chan struct{}
}

func (mw *mockWorker) Run(ctx context.Context, tasks <-chan interfaces.Task, id int32, logger interfaces.Logger) {
	mw.mu.Lock()
	mw.runs = append(mw.runs, id)
	logger.Debug("Воркер зарегистрирован", zap.Int32("worker_id", id))
	if mw.workerChan != nil {
		mw.workerChan <- struct{}{}
	}
	mw.mu.Unlock()

	select {
	case <-ctx.Done():
		logger.Debug("Воркер остановлен по контексту", zap.Int32("worker_id", id))
	case task, ok := <-tasks:
		if !ok {
			logger.Debug("Канал задач закрыт, воркер остановлен", zap.Int32("worker_id", id))
			return
		}
		if task != nil {
			logger.Debug("Обработка задачи", zap.Int32("worker_id", id))
			err := task.Process(ctx)
			if err != nil {
				return
			}
		}
	}
}

// mockTask реализует interfaces.Task для тестов.
type mockTask struct {
	processed bool
}

func (m *mockTask) Process(ctx context.Context) error {
	m.processed = true
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

// TestNewWorkerPool проверяет создание WorkerPool.
func TestNewWorkerPool(t *testing.T) {
	tests := []struct {
		name           string
		initialWorkers int
		taskBufferSize int
		poolTimeout    time.Duration
		wantErr        error
		wantWorkers    int
		wantLog        string
	}{
		{
			name:           "ValidPool",
			initialWorkers: 2,
			taskBufferSize: 10,
			poolTimeout:    100 * time.Millisecond,
			wantWorkers:    2,
			wantLog:        "Создан новый пул воркеров",
		},
		{
			name:           "PoolWithoutWorkers",
			initialWorkers: 0,
			taskBufferSize: 10,
			poolTimeout:    100 * time.Millisecond,
			wantWorkers:    0,
			wantLog:        "Создан новый пул воркеров",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.initialWorkers)}
			pool, err := NewWorkerPool(tt.initialWorkers, tt.taskBufferSize, worker, logger, tt.poolTimeout)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, pool)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, pool)
			assert.Equal(t, tt.taskBufferSize, cap(pool.tasks))
			assert.Equal(t, int32(tt.wantWorkers), atomic.LoadInt32(&pool.workerCount))

			for i := 0; i < tt.wantWorkers; i++ {
				select {
				case <-worker.workerChan:
					t.Logf("Воркер %d запущен", i+1)
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("Воркер %d не запустился вовремя", i+1)
				}
			}

			pool.Shutdown()
			_, ok := <-pool.tasks
			assert.False(t, ok, "Канал задач должен быть закрыт")

			// Проверяем лог
			assert.True(t, logger.containsLog("INFO", tt.wantLog), "Ожидается лог: %s", tt.wantLog)
		})
	}
}

// TestAddWorkers проверяет добавление воркеров.
func TestAddWorkers(t *testing.T) {
	tests := []struct {
		name        string
		numWorkers  int
		closed      bool
		wantWorkers int
		wantLog     string
	}{
		{
			name:        "AddingWorkers",
			numWorkers:  2,
			closed:      false,
			wantWorkers: 2,
			wantLog:     "Добавлены новые воркеры",
		},
		{
			name:        "AddingZeroWorkers",
			numWorkers:  0,
			closed:      false,
			wantWorkers: 0,
			wantLog:     "Попытка добавить неположительное количество воркеров",
		},
		{
			name:        "AddingToClosedPool",
			numWorkers:  2,
			closed:      true,
			wantWorkers: 0,
			wantLog:     "Попытка добавить воркеров в закрытый пул",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.numWorkers)}
			pool, err := NewWorkerPool(0, 10, worker, logger, 100*time.Millisecond)
			assert.NoError(t, err)

			if tt.closed {
				pool.Shutdown()
			}

			pool.AddWorkers(tt.numWorkers)

			for i := 0; i < tt.wantWorkers; i++ {
				select {
				case <-worker.workerChan:
					t.Logf("Воркер %d запущен", i+1)
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("Воркер %d не запустился вовремя", i+1)
				}
			}

			pool.Shutdown()
			_, ok := <-pool.tasks
			assert.False(t, ok, "Канал задач должен быть закрыт")

			if tt.wantLog != "" {
				assert.True(t, logger.containsLog("INFO", tt.wantLog) || logger.containsLog("WARN", tt.wantLog), "Ожидается лог: %s", tt.wantLog)
			}
		})
	}
}

// TestRemoveWorkers проверяет удаление воркеров.
func TestRemoveWorkers(t *testing.T) {
	tests := []struct {
		name           string
		initialWorkers int
		numWorkers     int
		closed         bool
		wantWorkers    int
		wantLog        string
	}{
		{
			name:           "RemoveWorker",
			initialWorkers: 2,
			numWorkers:     1,
			closed:         false,
			wantWorkers:    1,
			wantLog:        "Остановлены воркеры",
		},
		{
			name:           "RemoveZeroWorkers",
			initialWorkers: 2,
			numWorkers:     0,
			closed:         false,
			wantWorkers:    2,
			wantLog:        "Попытка удалить неположительное количество воркеров",
		},
		{
			name:           "RemoveFromClosedPool",
			initialWorkers: 2,
			numWorkers:     1,
			closed:         true,
			wantWorkers:    2,
			wantLog:        "Попытка удалить воркеров из закрытого пула",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.initialWorkers)}
			pool, err := NewWorkerPool(tt.initialWorkers, 2, worker, logger, 100*time.Millisecond)
			assert.NoError(t, err)

			if tt.closed {
				pool.Shutdown()
			}

			pool.RemoveWorkers(tt.numWorkers)

			for i := 0; i < tt.initialWorkers-tt.wantWorkers; i++ {
				select {
				case <-worker.workerChan:
					t.Logf("Воркер %d завершен", i+1)
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("Воркер %d не завершился вовремя", i+1)
				}
			}

			pool.Shutdown()
			_, ok := <-pool.tasks
			assert.False(t, ok, "Канал задач должен быть закрыт")

			if tt.wantLog != "" {
				assert.True(t, logger.containsLog("INFO", tt.wantLog) || logger.containsLog("WARN", tt.wantLog), "Ожидается лог: %s", tt.wantLog)
			}
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
		wantLog    string
	}{
		{
			name:    "ValidTask",
			task:    &mockTask{},
			closed:  false,
			wantLog: "Задача отправлена в пул",
		},
		{
			name:    "NilTask",
			task:    nil,
			closed:  false,
			wantErr: errors.ErrNilTask,
			wantLog: "Попытка отправить пустую задачу",
		},
		{
			name:    "ClosedPool",
			task:    &mockTask{},
			closed:  true,
			wantErr: errors.ErrPoolStopped,
			wantLog: "Попытка отправить задачу в закрытый пул",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, 1)}
			pool, err := NewWorkerPool(1, 2, worker, logger, 100*time.Millisecond)
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
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			// Завершаем пул
			pool.Shutdown()
			_, ok := <-pool.tasks
			assert.False(t, ok, "Канал задач должен быть закрыт")

			// Проверяем лог
			if tt.wantLog != "" {
				assert.True(t, logger.containsLog("DEBUG", tt.wantLog) || logger.containsLog("ERROR", tt.wantLog) || logger.containsLog("WARN", tt.wantLog), "Ожидается лог: %s", tt.wantLog)
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	tests := []struct {
		name           string
		initialWorkers int
		wantLogs       []string
	}{
		{
			name:           "NormalCompletion",
			initialWorkers: 2,
			wantLogs: []string{
				"Создан новый пул воркеров",
				"Инициирована остановка пула воркеров",
				"Все воркеры завершены",
				"Пул воркеров успешно остановлен",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			worker := &mockWorker{workerChan: make(chan struct{}, tt.initialWorkers)}
			pool, err := NewWorkerPool(tt.initialWorkers, 10, worker, logger, 100*time.Millisecond)
			assert.NoError(t, err)

			for i := 0; i < tt.initialWorkers; i++ {
				select {
				case <-worker.workerChan:
					t.Logf("Воркер %d запущен", i+1)
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("Воркер %d не запустился вовремя", i+1)
				}
			}

			pool.Shutdown()
			_, ok := <-pool.tasks
			assert.False(t, ok, "Канал задач должен быть закрыт")

			for _, wantLog := range tt.wantLogs {
				assert.True(t, logger.containsLog("INFO", wantLog) || logger.containsLog("WARN", wantLog), "Ожидается лог: %s", wantLog)
			}

			pool.Shutdown()
			assert.True(t, logger.containsLog("WARN", "Пул уже остановлен"), "Ожидается предупреждение о повторной остановке")
		})
	}
}
