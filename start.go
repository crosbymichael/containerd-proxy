package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// before starting a new task, cleanup any old tasks
// this can happen in the case that we were already running
func before(clix *cli.Context) error {
	var (
		ctx = newContext(clix)
		id  = clix.Args().First()
	)
	if id == "" {
		return errors.New("container id required")
	}
	client, err := containerd.New(defaults.DefaultAddress)
	if err != nil {
		return err
	}
	defer client.Close()
	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	_, err = task.Delete(ctx, containerd.WithProcessKill)
	return err
}

// start a new task
func start(clix *cli.Context) error {
	id := clix.Args().First()
	if id == "" {
		return errors.New("container id required")
	}
	code, err := runTask(newContext(clix), id)
	if err != nil {
		return err
	}
	os.Exit(code)
	return nil
}

func runTask(ctx context.Context, id string) (int, error) {
	var (
		signals = make(chan os.Signal, 64)
	)
	signal.Notify(signals)
	client, err := containerd.New(defaults.DefaultAddress)
	if err != nil {
		return -1, err
	}
	defer client.Close()
	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		return -1, err
	}
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return -1, err
	}
	wait, err := task.Wait(ctx)
	if err != nil {
		task.Delete(ctx, containerd.WithProcessKill)
		return -1, err
	}
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- task.Start(ctx)
	}()
	for {
		select {
		case err := <-startErrCh:
			if err != nil {
				task.Delete(ctx, containerd.WithProcessKill)
				return -1, err
			}
		case s := <-signals:
		killAgain:
			if err := task.Kill(ctx, s.(syscall.Signal)); err != nil {
				if errdefs.IsUnavailable(errdefs.FromGRPC(err)) {
					time.Sleep(100 * time.Millisecond)
					if rerr := client.Reconnect(); rerr != nil {
						task.Delete(ctx, containerd.WithProcessKill)
						return -1, err
					}
					if task, err = getTask(ctx, client, id); err != nil {
						return -1, err
					}
					goto killAgain
				}
				logrus.WithError(err).Error("signal lost")
			}
		case exit := <-wait:
			if exit.Error() != nil {
			waitAgain:
				if errdefs.IsUnavailable(errdefs.FromGRPC(exit.Error())) {
					time.Sleep(100 * time.Millisecond)
					if rerr := client.Reconnect(); rerr != nil {
						task.Delete(ctx, containerd.WithProcessKill)
						return -1, errors.Wrap(err, "reconnect")
					}
					if task, err = getTask(ctx, client, id); err != nil {
						return -1, errors.Wrap(err, "get task")
					}
					if wait, err = task.Wait(ctx); err != nil {
						goto waitAgain
					}
					continue
				}
				task.Delete(ctx, containerd.WithProcessKill)
				return -1, exit.Error()
			}
			task.Delete(ctx, containerd.WithProcessKill)
			return int(exit.ExitCode()), nil
		}
	}
}

func getTask(ctx context.Context, client *containerd.Client, id string) (containerd.Task, error) {
	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	return container.Task(ctx, cio.NewAttach(cio.WithStdio))
}
