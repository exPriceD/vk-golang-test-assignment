package main

import (
	"errors"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"sync"
	"time"
	"vk-worker-pool/internal/config"
	"vk-worker-pool/internal/logger"
	"vk-worker-pool/internal/task"
	"vk-worker-pool/internal/worker"
	"vk-worker-pool/internal/workerpool"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "путь до конфиг-файла")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Ошибка загрузки конфигурации: %v\n", err)
		return
	}

	log, err := logger.NewLogger(cfg.Logger)
	if err != nil {
		fmt.Printf("Ошибка создания логгера: %v\n", err)
		return
	}
	defer func() {
		if err := log.Sync(); err != nil {
			fmt.Printf("Ошибка завершения логгера: %v\n", err)
		}
	}()

	w := worker.NewDefaultWorker(cfg.Worker.TaskTimeout)

	pool, err := workerpool.NewWorkerPool(
		cfg.Worker.InitialWorkers,
		cfg.Worker.TaskBufferSize,
		w,
		log,
		cfg.Worker.PoolTimeout,
	)
	if err != nil {
		log.Error("Ошибка создания пула воркеров", zap.Error(err))
		return
	}

	var wg sync.WaitGroup

	// Отправляем первые 5 задач
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		t := task.StringTask{Data: fmt.Sprintf("Задача %d", i)}
		wrappedTask := task.WrapWithWaitGroup(t, &wg)
		if err := pool.Submit(wrappedTask); err != nil {
			if errors.Is(err, errors.ErrUnsupported) {
				log.Error("Пул воркеров остановлен", zap.Error(err))
				return
			}
			log.Error("Ошибка при отправке задачи", zap.Error(err), zap.Int("task_number", i))
		}
	}

	wg.Wait()

	// Добавляем 2 новых воркера
	log.Info("Добавляем 2 воркера")
	pool.AddWorkers(2)

	// Даем время для наблюдения за работой
	time.Sleep(1 * time.Second)

	// Удаляем 1 воркера
	log.Info("Удаляем 1 воркера")
	pool.RemoveWorkers(1)

	// Отправляем еще 5 задач
	for i := 6; i <= 10; i++ {
		wg.Add(1)
		t := task.StringTask{Data: fmt.Sprintf("Задача %d", i)}
		wrappedTask := task.WrapWithWaitGroup(t, &wg)
		if err := pool.Submit(wrappedTask); err != nil {
			if errors.Is(err, errors.ErrUnsupported) {
				log.Error("Пул воркеров остановлен", zap.Error(err))
				return
			}
			log.Error("Ошибка при отправке задачи", zap.Error(err), zap.Int("task_number", i))
		}
	}

	wg.Wait()

	// Выполняем graceful shutdown пула
	pool.Shutdown()
	pool.Shutdown() // Не отработает, получим WARN, а не панику
}
