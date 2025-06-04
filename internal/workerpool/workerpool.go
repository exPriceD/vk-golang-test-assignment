package workerpool

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"vk-worker-pool/internal/errors"
)

type Task struct {
	Data string
}

type WorkerPool struct {
	tasks       chan Task      // Канал для задач
	workers     sync.WaitGroup // WaitGroup для отслеживания активных воркеров
	workerCount int32          // Счетчик активных воркеров
	ctx         context.Context
	cancel      context.CancelFunc
	log         *zap.Logger
}

func NewWorkerPool(initialWorkers int, taskBufferSize int, log *zap.Logger) (*WorkerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		tasks:  make(chan Task, taskBufferSize),
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
		go wp.worker(workerID)
	}
}

// RemoveWorkers останавливает указанное количество воркеров.
func (wp *WorkerPool) RemoveWorkers(numWorkers int) {
	if numWorkers <= 0 {
		wp.log.Warn("Попытка удалить неположительное количество воркеров", zap.Int("num_workers", numWorkers))
		return
	}
	for i := 0; i < numWorkers; i++ {
		if atomic.LoadInt32(&wp.workerCount) > 0 {
			wp.tasks <- Task{Data: "STOP"}
		}
	}
	wp.log.Info("Остановлены воркеры", zap.Int("num_workers", numWorkers))

}

// Submit добавляет новую задачу в пул.
func (wp *WorkerPool) Submit(task Task) error {
	select {
	case <-wp.tasks:
		wp.tasks <- task
		wp.log.Debug("Задача отправлена в пул", zap.String("task_data", task.Data))
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

// worker горутина, которая обрабатывает задачи.
func (wp *WorkerPool) worker(id int32) {
	defer wp.workers.Done()
	wp.log.Debug("Воркер запущен", zap.Int32("worker_id", id))
	for {
		select {
		case <-wp.ctx.Done():
			wp.log.Debug("Воркер остановлен по контексту", zap.Int32("worker_id", id))
			return
		case task, ok := <-wp.tasks:
			if !ok {
				wp.log.Debug("Канал задач закрыт, воркер остановлен", zap.Int32("worker_id", id))
				return
			}
			if task.Data == "STOP" {
				atomic.AddInt32(&wp.workerCount, -1)
				wp.log.Debug("Воркер остановлен по сигнальной задаче", zap.Int32("worker_id", id))
				return
			}
			wp.log.Info("Воркер обрабатывает задачу",
				zap.Int32("worker_id", id),
				zap.String("task_data", task.Data))
			fmt.Printf("Воркер %d обрабатывает задачу: %s\n", id, task.Data)
		}
	}
}
