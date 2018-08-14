package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
)

func WithCurrentSpec(config *Config) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		args := append([]string{
			filepath.Base(os.Args[0]),
		}, config.Args...)
		args = append(args, os.Args[1:]...)
		s, err := oci.GenerateSpec(ctx, client, c,
			oci.WithProcessArgs(args...),
			oci.WithEnv(os.Environ()),
		)
		if err != nil {
			return err
		}
		s.Linux.Resources.Devices = nil
		c.Spec, err = typeurl.MarshalAny(s)
		return err
	}
}

func WithScope(scope string) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if c.Labels == nil {
			c.Labels = make(map[string]string)
		}
		c.Labels[ScopeLabel] = scope
		return nil
	}
}
