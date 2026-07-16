package main

import (
	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/examples/preferences"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

func main() {
	desktop.Run(preferences.App, render.AppConfig{
		Title:  "Preferences",
		Width:  480,
		Height: 360,
	})
}
