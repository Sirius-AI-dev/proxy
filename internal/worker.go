package internal

import (
	"github.com/unibackend/uniproxy/internal/worker"
)

func (c *Container) RunWorker(workerMode string) error {
	// Run DI container
	if err := c.container.Invoke(func(w worker.Interface) error {
		w.SetMode(workerMode)

		// Run worker listen
		return w.Run()
	}); err != nil {
		return err
	}

	return nil
}
