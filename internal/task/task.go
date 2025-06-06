package task

import (
	"context"
	"fmt"
)

// StringTask — конкретная реализация задачи с текстовыми данными.
type StringTask struct {
	Data string
}

func (t StringTask) Process(ctx context.Context) error {
	// В дальнейшем можно добавить логику для проверки отмены контекста и т.п
	fmt.Printf("[Process()] Обработка задачи: %s\n", t.Data)
	return nil
}
