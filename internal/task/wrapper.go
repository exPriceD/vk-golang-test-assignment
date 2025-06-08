package task

import (
	"context"
	"sync"
	"vk-worker-pool/internal/interfaces"
)

// WrappedTask обертка над задачей, которая вызывает wg.Done() после завершения
type WrappedTask struct {
	task interfaces.Task
	wg   *sync.WaitGroup
}

func WrapWithWaitGroup(task interfaces.Task, wg *sync.WaitGroup) interfaces.Task {
	return WrappedTask{task: task, wg: wg}
}

func (wt WrappedTask) Process(ctx context.Context) error {
	defer wt.wg.Done()
	return wt.task.Process(ctx)
}
