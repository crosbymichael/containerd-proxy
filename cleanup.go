package main

import (
	"context"

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
	_, err = task.Delete(ctx, containerd.WithProcessKill)
	return err
}
