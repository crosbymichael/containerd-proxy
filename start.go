package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
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
	var (
		signals = make(chan os.Signal, 64)
		ctx     = newContext(clix)
		id      = clix.Args().First()
	)
	signal.Notify(signals)
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
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	defer task.Delete(ctx, containerd.WithProcessKill)
	wait, err := task.Wait(ctx)
	if err != nil {
		return err
	}
	// handle signals
	go func() {
		for s := range signals {
			if err := task.Kill(ctx, s.(syscall.Signal)); err != nil {
				logrus.WithError(err).Error("signal lost")
			}
		}
	}()

	if err := task.Start(ctx); err != nil {
		return err
	}
	exit := <-wait
	if exit.Error() != nil {
		return exit.Error()
	}
	return cli.NewExitError(nil, int(exit.ExitCode()))
}
