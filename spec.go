package main

import (
	"context"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
)

func WithCurrentSpec(config *Config) func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		s, err := oci.GenerateSpec(ctx, client, c,
			oci.WithProcessArgs(config.GetArgs()...),
			oci.WithEnv(os.Environ()),
			oci.WithParentCgroupDevices,
		)
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
