package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/crosbymichael/boss/flux"
	"golang.org/x/sys/unix"
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
	if len(os.Args) == 2 && os.Args[1] == "post-stop" {
		if err := cleanup(ctx, id); err != nil {
			exit(err)
		}
		return
	}
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
		containerd.WithTimeout(1*time.Second),
	)
	if err != nil {
		return err
	}
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
	if err := checkRunning(ctx, container); err != nil {
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

	wait, err := task.Wait(ctx)
	if err != nil {
		task.Delete(ctx)
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
			if s == unix.SIGCONT {
				continue
			}
			if err := trySendSignal(ctx, client, task, s); err != nil {
				return err
			}
		case exit := <-wait:
			if exit.Error() != nil {
				if !isUnavailable(err) {
					unix.Kill(int(task.Pid()), unix.SIGKILL)
					return err
				}
				if client, task, err = reconnect(ctx, config.ID); err != nil {
					unix.Kill(int(task.Pid()), unix.SIGKILL)
					return err
				}
				if wait, err = task.Wait(ctx); err != nil {
					return err
				}
				continue
			}
			task.Delete(ctx)
			return &exitError{
				Status: int(exit.ExitCode()),
			}
		}
	}
}
