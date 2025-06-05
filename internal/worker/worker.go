package worker

import (
	"context"
	"go.uber.org/zap"
	"time"
	"vk-worker-pool/internal/interfaces"
)

type DefaultWorker struct {
	taskTimeout time.Duration
}

// NewDefaultWorker создает новый DefaultWorker с заданными параметрами.
// taskTimeout определяет максимальное время выполнения задачи Process.
func NewDefaultWorker(taskTimeout time.Duration) *DefaultWorker {
	return &DefaultWorker{
		taskTimeout: taskTimeout,
	}
}

// Run выполняет цикл обработки задач
func (w DefaultWorker) Run(ctx context.Context, tasks <-chan interfaces.Task, id int32, log interfaces.Logger) {
	log.Debug("Воркер запущен", zap.Int32("worker_id", id))
	for {
		select {
		case <-ctx.Done():
			log.Debug("Воркер остановлен по контексту", zap.Int32("worker_id", id))
			return
		case task, ok := <-tasks:
			if !ok {
				log.Debug("Канал задач закрыт, воркер остановлен", zap.Int32("worker_id", id))
				return
			}
			if task == nil {
				log.Debug("Воркер остановлен по сигнальной задаче", zap.Int32("worker_id", id))
				return
			}
			log.Info("Воркер обрабатывает задачу", zap.Int32("worker_id", id))
			taskCtx, cancel := context.WithTimeout(ctx, w.taskTimeout)
			errChan := make(chan error, 1)
			go func() {
				errChan <- task.Process(taskCtx)
			}()
			select {
			case err := <-errChan:
				if err != nil {
					log.Error("Ошибка обработки задачи", zap.Error(err))
				}
			case <-taskCtx.Done():
				log.Error("Таймаут обработки задачи", zap.Int32("worker_id", id))
			}
			cancel()
		}
	}
}
