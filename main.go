package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/flux"
)

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
		image, err := config.GetImage(ctx, client)
		if err != nil {
			return err
		}
		// create new container
		if container, err = client.NewContainer(ctx, config.ID,
			WithCurrentSpec(config),
			flux.WithNewSnapshot(image),
			WithScope(config.Scope),
		); err != nil {
			return err
		}
	}
	// cleanup old task if it is still hangin around
	if err := cleanup(ctx, container); err != nil {
		return err
	}
	// update container with new spec for current run
	if err := container.Update(ctx, WithCurrentSpec(config)); err != nil {
		return err
	}
	info, err := container.Info(ctx)
	if err != nil {
		return err
	}
	if info.Labels == nil {
		info.Labels = make(map[string]string)
	}
	if config.ShouldUpgrade(info.Image, info.Labels[ScopeLabel]) {
		image, err := config.GetImage(ctx, client)
		if err != nil {
			return err
		}
		if err := container.Update(ctx, flux.WithUpgrade(image), WithScope(config.Scope)); err != nil {
			return err
		}
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
