package main

import (
	"context"
	"errors"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
)

func cleanup(ctx context.Context, container containerd.Container) error {
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	status, err := task.Status(ctx)
	if err != nil {
		return err
	}
	if status.Status != containerd.Stopped {
		return errors.New("unable to start running container")
	}
	_, err = task.Delete(ctx)
	return err
}
