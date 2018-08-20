package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images/oci"
)

// Const can be assigned at buildtime with ldflags to customize the proxy
const (
	ScopeLabel = "com.crosbymichael/containerd-proxy.scope"
	AnyScope   = "*"
)

func loadConfig(id string) (*Config, error) {
	f, err := os.Open(filepath.Join("/etc/containerd-proxy", fmt.Sprintf("%s.json", id)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	c.ID = id
	return &c, nil
}

type Config struct {
	ID        string   `json:"-"`
	Namespace string   `json:"namespace"`
	Image     string   `json:"image"`
	ImagePath string   `json:"imagePath"`
	Args      []string `json:"args"`
	Scope     string   `json:"scope"`
}

// ShouldUpgrade matches the scope and image for a container to decide if an upgrade is required
func (c *Config) ShouldUpgrade(containerImage, containerScope string) bool {
	if c.Image != containerImage {
		switch {
		case containerScope == "":
			return true
		case containerScope == AnyScope:
			return true
		case c.Scope == AnyScope:
			return true
		case containerScope == c.Scope:
			return true
		default:
			return false
		}
	}
	return false
}

// GetImage returns the image for the config
func (c *Config) GetImage(ctx context.Context, client *containerd.Client) (containerd.Image, error) {
	image, err := client.GetImage(ctx, c.Image)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}
		// we don't have the image so check if we have a bundle
		switch {
		case c.ImagePath != "":
			importer := &oci.V1Importer{
				ImageName: c.Image,
			}
			f, err := os.Open(c.ImagePath)
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
			if err := image.Unpack(ctx, containerd.DefaultSnapshotter); err != nil {
				return nil, err
			}
		default:
			if image, err = client.Pull(ctx, c.Image, containerd.WithPullUnpack); err != nil {
				return nil, err
			}
		}
	}
	return image, nil
}

func (c *Config) GetArgs() []string {
	args := append([]string{
		filepath.Base(os.Args[0]),
	}, c.Args...)
	return append(args, os.Args[1:]...)
}
