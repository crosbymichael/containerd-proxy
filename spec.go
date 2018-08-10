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

func WithCurrentSpec(ctx context.Context, client *containerd.Client, c *containers.Container) error {
	s, err := oci.GenerateSpec(ctx, client, c,
		oci.WithProcessArgs(append([]string{filepath.Base(os.Args[0])}, os.Args[1:]...)...),
		oci.WithEnv(os.Environ()),
	)
	if err != nil {
		return err
	}
	c.Spec, err = typeurl.MarshalAny(s)
	return err
}
