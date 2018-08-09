package main

import (
	"context"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
)

func WithCurrentSpec(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	s, err := oci.GenerateSpec(ctx, client, c,
		oci.WithProcessArgs(os.Args[0:]...),
		oci.WithProcessCwd(cwd),
		oci.WithEnv(os.Environ()),
	)
	if err != nil {
		return err
	}
	c.Spec, err = typeurl.MarshalAny(s)
	return err
}
