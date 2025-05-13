package fxutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"go.uber.org/multierr"
)

// App is an interface of fx.App
type App interface {
	Start(context.Context) error
	Stop(context.Context) error
	Done() <-chan os.Signal
}

// RunServices builds and starts multiple services in parallel and waits for them to finish.
func RunServices(services []string, appBuilder func(serviceName string) App) error {
	stoppedWg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, len(services))

	for _, serv := range services {
		stoppedWg.Add(1)
		go func(s string) {
			defer stoppedWg.Done()
			fxApp := appBuilder(s)

			//  If any of the start hooks return an error, Start short-circuits, calls Stop, and returns the inciting error.
			if err := fxApp.Start(ctx); err != nil {
				// If any of the apps fails to start, immediately cancel the context so others will also stop.
				cancel()
				errChan <- fmt.Errorf("service %s start: %w", s, err)
				return
			}

			select {
			// Block until FX receives a shutdown signal
			case <-fxApp.Done():
			}

			// Stop the application
			err := fxApp.Stop(ctx)
			if err != nil {
				errChan <- fmt.Errorf("service %s stop: %w", s, err)
			}
		}(serv)
	}
	go func() {
		stoppedWg.Wait()
		// After stoppedWg unblocked all services are stopped to we no longer wait for errors.
		close(errChan)
	}()

	var resErrors error
	for err := range errChan {
		// skip canceled errors, since they are caused by context cancelation and only focus on actual errors.
		if err != nil && !errors.Is(err, context.Canceled) {
			resErrors = multierr.Append(resErrors, err)
		}
	}
	if resErrors != nil {
		return resErrors
	}
	return nil
}
