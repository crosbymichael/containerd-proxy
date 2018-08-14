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
	for i := 0; i < 5; i++ {
		err := task.Kill(ctx, s.(syscall.Signal))
		if err == nil {
			return nil
		}
		if !isUnavailable(err) {
			return err
		}
		if err := reconnect(client); err != nil {
			return err
		}
	}
	// fallback to get this signal sent
	return unix.Kill(int(task.Pid()), s.(syscall.Signal))
}

func reconnect(client *containerd.Client) (err error) {
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if err = client.Reconnect(); err == nil {
			return nil
		}
	}
	return err
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
