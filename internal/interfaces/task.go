package interfaces

import "context"

// Task определяет контракт для задач, которые может обрабатывать WorkerPool.
// Реализовано для повышения гибкости (например, если мы захотим изменить таску на JSON и многое другое).
type Task interface {
	Process(ctx context.Context) error // Выполняет задачу и возвращает ошибку, если она произошла
}
