package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ID        string   `json:"-"`
	Namespace string   `json:"namespace"`
	Image     string   `json:"image"`
	ImagePath string   `json:"imagePath"`
	Args      []string `json:"args"`
}

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
