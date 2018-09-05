package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"golang.org/x/sys/unix"
)

func getTask(ctx context.Context, client *containerd.Client, id string) (containerd.Task, error) {
	container, err := client.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	return container.Task(ctx, cio.NewAttach(cio.WithStdio))
}

func trySendSignal(ctx context.Context, client *containerd.Client, task containerd.Task, s os.Signal) error {
	err := task.Kill(ctx, s.(syscall.Signal))
	if err == nil {
		return nil
	}
	if !isUnavailable(err) {
		return err
	}
	return unix.Kill(int(task.Pid()), s.(syscall.Signal))
}

func reconnect(ctx context.Context, id string) (*containerd.Client, containerd.Task, error) {
	client, err := containerd.New(
		defaults.DefaultAddress,
		containerd.WithDefaultRuntime("io.containerd.process.v1"),
		containerd.WithTimeout(1*time.Second),
	)
	if err != nil {
		return nil, nil, err
	}
	t, err := getTask(ctx, client, id)
	if err != nil {
		return nil, nil, err
	}
	return client, t, nil
}

func isUnavailable(err error) bool {
	return errdefs.IsUnavailable(errdefs.FromGRPC(err))
}

type exitError struct {
	Status int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Status)
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func getID() string {
	return filepath.Base(os.Args[0])
}
