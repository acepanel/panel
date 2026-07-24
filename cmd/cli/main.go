package main

import (
	"errors"
	"os"
	_ "time/tzdata"

	"github.com/gookit/color"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/injector"
)

func main() {
	if err := run(); err != nil {
		color.Errorf("|-%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if os.Geteuid() != 0 {
		return errors.New("panel must run as root")
	}

	inj := injector.New()
	defer func() { _ = inj.Shutdown() }()

	cli, err := do.Invoke[*app.Cli](inj)
	if err != nil {
		return err
	}

	return cli.Run()
}
