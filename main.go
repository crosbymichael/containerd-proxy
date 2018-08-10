package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images/oci"
	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/flux"
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
	)
	config, err := loadConfig(id)
	if err != nil {
		exit(err)
	}
	ctx := namespaces.WithNamespace(context.Background(), config.Namespace)
	signal.Notify(signals)
	if err := proxy(ctx, config, signals); err != nil {
		if eerr, ok := err.(*exitError); ok {
			os.Exit(eerr.Status)
		}
		exit(err)
	}
}

func proxy(ctx context.Context, config *Config, signals chan os.Signal) error {
	client, err := containerd.New(
		defaults.DefaultAddress,
		containerd.WithDefaultRuntime("io.containerd.process.v1"),
	)
	if err != nil {
		return err
	}
	defer client.Close()
	container, err := client.LoadContainer(ctx, config.ID)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
		// create new container
		if container, err = create(ctx, client, config); err != nil {
			return err
		}
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
	defer task.Delete(ctx)

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

func create(ctx context.Context, client *containerd.Client, config *Config) (containerd.Container, error) {
	image, err := client.GetImage(ctx, config.Image)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		// we don't have the image so check if we have a bundle
		switch {
		case config.ImagePath != "":
			importer := &oci.V1Importer{
				ImageName: config.Image,
			}
			f, err := os.Open(config.ImagePath)
			if err != nil {
				return nil, err
			}
			images, err := client.Import(ctx, importer, f)
			f.Close()
			if err != nil {
				return nil, err
			}
			if len(images) != 1 {
				return nil, errors.New("no image imported")
			}
			image = images[0]
		default:
			if image, err = client.Pull(ctx, config.Image, containerd.WithPullUnpack); err != nil {
				return nil, err
			}
		}
	}
	return client.NewContainer(ctx, config.ID, WithCurrentSpec, flux.WithNewSnapshot(image))
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func getID() string {
	return filepath.Base(os.Args[0])
}
