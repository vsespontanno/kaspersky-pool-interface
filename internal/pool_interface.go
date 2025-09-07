package internal

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrPoolStopped              = errors.New("pool stopped")
	ErrChanFull                 = errors.New("chan full")
	ErrNilTask                  = errors.New("nil task")
	ErrIncorrectNumberOfWorkers = errors.New("incorrect number of workers")
	ErrIncorrectBufOfChan       = errors.New("incorrect buf size")
)

// Реализовать интерфейс Pool, который позволяет выполнять задачи параллельно.
type Pool interface {
	// Submit - добавить задачу в пул.
	// Если пул не имеет свободных воркеров, то задачу нужно добавить в очередь.
	Submit(task func()) error
	// Stop - остановить воркер пул, дождаться выполнения всех добавленных ранее в очередь задач.
	Stop() error
}

type WorkerPool struct {
	stopOnce        sync.Once
	tasks           chan func()
	done            chan struct{}
	wg              sync.WaitGroup
	wgWorker        sync.WaitGroup
	numberOfWorkers int
	hook            func()
	hookMu          sync.Mutex
}

func NewWorkerPool(numberOfWorkers int, bufOfChan int) (*WorkerPool, error) {
	if numberOfWorkers <= 0 {
		return nil, ErrIncorrectNumberOfWorkers
	}
	if bufOfChan <= 0 {
		return nil, ErrIncorrectBufOfChan
	}
	wp := &WorkerPool{
		tasks:           make(chan func(), bufOfChan),
		done:            make(chan struct{}),
		numberOfWorkers: numberOfWorkers,
	}
	wp.start()
	return wp, nil
}

func (wp *WorkerPool) start() {
	for i := 0; i < wp.numberOfWorkers; i++ {
		wp.wgWorker.Add(1)
		go func() {
			defer wp.wgWorker.Done()
			for task := range wp.tasks {
				wp.executeTask(task)
			}
		}()
	}
}

func (wp *WorkerPool) executeTask(task func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("We got panic: ", r)
		}
	}()
	task()

	wp.hookMu.Lock()
	if wp.hook != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("We got panic in hook:", r)
				}
			}()
			wp.hook()
		}()
	}
	wp.hookMu.Unlock()
}

func (wp *WorkerPool) Submit(task func()) error {
	if task == nil {
		return ErrNilTask
	}
	select {
	case <-wp.done:
		return ErrPoolStopped
	case wp.tasks <- func() {
		task()
		wp.wg.Done()
	}:
		wp.wg.Add(1)
		return nil
	default:
		return ErrChanFull
	}
}

func (wp *WorkerPool) Stop() error {
	wp.stopOnce.Do(func() {
		close(wp.done)
		wp.wg.Wait()
		close(wp.tasks)
		wp.wgWorker.Wait()
	})
	return nil
}
