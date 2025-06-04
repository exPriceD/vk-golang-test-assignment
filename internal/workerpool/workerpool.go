package workerpool

import (
	"context"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
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
	workerCount int32                // Счетчик активных воркеров
	worker      Worker
	ctx         context.Context
	cancel      context.CancelFunc
	log         interfaces.Logger
}

func NewWorkerPool(initialWorkers int, taskBufferSize int, worker Worker, log interfaces.Logger) (*WorkerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		tasks:  make(chan interfaces.Task, taskBufferSize),
		worker: worker,
		ctx:    ctx,
		cancel: cancel,
		log:    log,
	}

	wp.log.Info("Создан новый пул воркеров",
		zap.Int("initial_workers", initialWorkers),
		zap.Int("task_buffer_size", taskBufferSize),
	)

	wp.AddWorkers(initialWorkers)

	return wp, nil
}

// AddWorkers добавляет указанное количество воркеров в пул.
func (wp *WorkerPool) AddWorkers(numWorkers int) {
	if numWorkers <= 0 {
		wp.log.Warn("Попытка добавить неположительное количество воркеров", zap.Int("num_workers", numWorkers))
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
	for i := 0; i < numWorkers; i++ {
		if atomic.LoadInt32(&wp.workerCount) > 0 {
			wp.tasks <- nil
			atomic.AddInt32(&wp.workerCount, -1)
		}
	}
	wp.log.Info("Остановлены воркеры", zap.Int("num_workers", numWorkers))

}

// Submit добавляет новую задачу в пул.
func (wp *WorkerPool) Submit(task interfaces.Task) error {
	select {
	case <-wp.tasks:
		wp.tasks <- task
		wp.log.Debug("Задача отправлена в пул")
		return nil
	case <-wp.ctx.Done():
		wp.log.Error("Не удалось отправить задачу: пул остановлен")
		return errors.ErrPoolStopped
	}
}

// Shutdown выполняет завершение всех воркеров и закрывает канал задач.
func (wp *WorkerPool) Shutdown() {
	wp.log.Info("Инициирована остановка пула воркеров")
	wp.cancel()
	wp.workers.Wait()
	close(wp.tasks)
	wp.log.Info("Пул воркеров успешно остановлен")
}
