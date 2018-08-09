package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
)

type exitError struct {
	Status int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Status)
}

func main() {
	var (
		id      = getID()
		signals = make(chan os.Signal, 64)
		ctx     = namespaces.WithNamespace(context.Background(), "com.docker")
	)
	signal.Notify(signals)
	if err := proxy(ctx, id, signals); err != nil {
		if eerr, ok := err.(*exitError); ok {
			os.Exit(eerr.Status)
		}
		exit(err)
	}
}

func proxy(ctx context.Context, id string, signals chan os.Signal) error {
	client, err := containerd.New(defaults.DefaultAddress)
	if err != nil {
		return err
	}
	defer client.Close()
	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
		// create new container
	}
	// cleanup old task if it is still hangin around
	if err := cleanup(ctx, container); err != nil {
		return err
	}
	// update container with new spec for current run
	if err := container.Update(ctx, WithCurrentSpec); err != nil {
		return err
	}
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	defer task.Delete(ctx, containerd.WithProcessKill)

	wait, err := task.Wait(ctx)
	if err != nil {
		return err
	}
	started := make(chan error, 1)
	go func() {
		started <- task.Start(ctx)
	}()
	for {
		select {
		case err := <-started:
			if err != nil {
				return err
			}
		case s := <-signals:
			if err := trySendSignal(ctx, client, task, s); err != nil {
				return err
			}
		case exit := <-wait:
			if exit.Error() != nil {
				if !isUnavailable(err) {
					return err
				}
				if err := reconnect(client); err != nil {
					return err
				}
				if wait, err = task.Wait(ctx); err != nil {
					return err
				}
				continue
			}
			return &exitError{
				Status: int(exit.ExitCode()),
			}
		}
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func getID() string {
	return filepath.Base(os.Args[0])
}
