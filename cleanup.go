package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func cleanup(ctx context.Context, id string) error {
	client, err := containerd.New(
		defaults.DefaultAddress,
		containerd.WithDefaultRuntime("io.containerd.process.v1"),
		containerd.WithTimeout(1*time.Second),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
		// container does not exist so there is nothing for us to do
		return nil
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			// there is no running task, nothing to kill
			return nil
		}
		return err
	}
	wait, err := task.Wait(ctx)
	if err != nil {
		task.Kill(ctx, unix.SIGKILL)
		_, err = task.Delete(ctx)
		return err
	}
	if err := task.Kill(ctx, unix.SIGKILL); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
	<-wait
	_, err = task.Delete(ctx)
	return err
}

func checkRunning(ctx context.Context, container containerd.Container) error {
	if _, err := container.Task(ctx, nil); err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	return errors.Errorf("container %s already has a running process", container.ID())
}
