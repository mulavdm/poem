package main

import (
	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/examples/settings"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

func main() {
	desktop.Run(settings.App, render.AppConfig{
		Title:  "Settings",
		Width:  480,
		Height: 360,
	})
}
