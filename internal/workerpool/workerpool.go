package workerpool

import (
	"context"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"time"
	"vk-worker-pool/internal/errors"
	"vk-worker-pool/internal/interfaces"
)

// Worker определяет контракт для воркера.
type Worker interface {
	Run(ctx context.Context, tasks <-chan interfaces.Task, id int32, log interfaces.Logger)
}

type WorkerPool struct {
	tasks       chan interfaces.Task // Канал для задач
	workers     sync.WaitGroup       // WaitGroup для отслеживания активных воркеров
	workerCount int32                // Счетчик активных воркеров. Тип int32 выбран для совместимости с atomic
	worker      Worker
	closed      int32 //Флаг для отслеживания состояния пула (0 - открыт, 1 - закрыт). Тип int32 выбран для совместимости с atomic
	ctx         context.Context
	cancel      context.CancelFunc
	log         interfaces.Logger
	poolTimeout time.Duration
}

// NewWorkerPool Конструктор воркерпула. В дальнейшем при росте пула стоит использовать паттерн Functional Options для реализации гибкой и расширяемой конфигурации
func NewWorkerPool(initialWorkers int, taskBufferSize int, worker Worker, log interfaces.Logger, poolTimeout time.Duration) (*WorkerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		tasks:       make(chan interfaces.Task, taskBufferSize),
		worker:      worker,
		ctx:         ctx,
		cancel:      cancel,
		log:         log,
		poolTimeout: poolTimeout,
	}

	wp.log.Info("Создан новый пул воркеров",
		zap.Int("initial_workers", initialWorkers),
		zap.Int("task_buffer_size", taskBufferSize),
	)

	if initialWorkers > 0 {
		wp.AddWorkers(initialWorkers)
	}

	return wp, nil
}

// AddWorkers добавляет указанное количество воркеров в пул.
func (wp *WorkerPool) AddWorkers(numWorkers int) {
	if numWorkers <= 0 {
		wp.log.Warn("Попытка добавить неположительное количество воркеров", zap.Int("num_workers", numWorkers))
		return
	}
	if atomic.LoadInt32(&wp.closed) == 1 {
		wp.log.Warn("Попытка добавить воркеров в закрытый пул")
		return
	}
	for i := 0; i < numWorkers; i++ {
		wp.workers.Add(1)
		// Можно использовать sync.Mutex, но это менее эффективно, так как мьютексы требуют блокировки
		workerID := atomic.AddInt32(&wp.workerCount, 1)
		go func(id int32) {
			wp.worker.Run(wp.ctx, wp.tasks, workerID, wp.log)
			wp.workers.Done()
		}(workerID)
	}
	wp.log.Info("Добавлены новые воркеры", zap.Int("num_workers", numWorkers))
}

// RemoveWorkers останавливает указанное количество воркеров.
func (wp *WorkerPool) RemoveWorkers(numWorkers int) {
	if numWorkers <= 0 {
		wp.log.Warn("Попытка удалить неположительное количество воркеров", zap.Int("num_workers", numWorkers))
		return
	}
	if atomic.LoadInt32(&wp.closed) == 1 {
		wp.log.Warn("Попытка удалить воркеров из закрытого пула")
		return
	}
	for i := 0; i < numWorkers; i++ {
		if atomic.LoadInt32(&wp.workerCount) > 0 {
			select {
			case wp.tasks <- nil:
				atomic.AddInt32(&wp.workerCount, -1)
			case <-wp.ctx.Done():
				wp.log.Warn("Пул остановлен, не удалось отправить сигнальную задачу")
				return
			case <-time.After(wp.poolTimeout):
				wp.log.Warn("Таймаут отправки сигнальной задачи: буфер полон")
				return
			}
		}
	}
	wp.log.Info("Остановлены воркеры", zap.Int("num_workers", numWorkers))

}

// Submit добавляет новую задачу в пул.
func (wp *WorkerPool) Submit(task interfaces.Task) error {
	if task == nil {
		wp.log.Error("Попытка отправить пустую задачу")
		return errors.ErrNilTask
	}
	if atomic.LoadInt32(&wp.closed) == 1 {
		wp.log.Error("Попытка отправить задачу в закрытый пул")
		return errors.ErrPoolStopped
	}
	select {
	case wp.tasks <- task:
		wp.log.Debug("Задача отправлена в пул")
		return nil
	case <-wp.ctx.Done():
		wp.log.Error("Не удалось отправить задачу: пул остановлен")
		return errors.ErrPoolStopped
	case <-time.After(wp.poolTimeout):
		wp.log.Warn("Таймаут отправки задачи: буфер полон")
		return errors.ErrTimeout
	}
}

// Shutdown выполняет завершение всех воркеров и закрывает канал задач.
func (wp *WorkerPool) Shutdown() {
	if !atomic.CompareAndSwapInt32(&wp.closed, 0, 1) {
		wp.log.Warn("Пул уже остановлен")
		return
	}
	wp.log.Info("Инициирована остановка пула воркеров")
	wp.cancel()

	done := make(chan struct{})
	go func() {
		wp.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		wp.log.Info("Все воркеры завершены")
	case <-time.After(wp.poolTimeout):
		wp.log.Warn("Таймаут ожидания завершения воркеров")
	}

	close(wp.tasks)
	wp.log.Info("Пул воркеров успешно остановлен")
}
