package worker

import (
	"context"
	"go.uber.org/zap"
)

type Task interface {
	Process() error
}

type DefaultWorker struct{}

// Run выполняет цикл обработки задач
func (w *DefaultWorker) Run(ctx context.Context, tasks <-chan Task, id int32, log *zap.Logger) {
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
			if err := task.Process(); err != nil {
				log.Error("Ошибка обработки задачи", zap.Error(err))
			}
		}
	}
}
