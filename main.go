package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/namespaces"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "containerd-system"
	app.Usage = "containerd systemd proxy for executing containers like systemd.services"
	app.Version = "1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "namespace,n",
			Usage: "containerd namespace",
			Value: "default",
		},
	}
	app.Before = before
	app.Action = start
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newContext(clix *cli.Context) context.Context {
	return namespaces.WithNamespace(context.Background(), clix.GlobalString("namespace"))
}
