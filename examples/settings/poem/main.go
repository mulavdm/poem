//go:build windows

package main

import (
	"github.com/mulavdm/poem/pkg/render"
	poemwindows "github.com/mulavdm/poem/pkg/windows"

	"github.com/mulavdm/poem/examples/settings"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

func init() {
	config := desktop.Configure(settings.App, render.AppConfig{
		Title:  "Settings",
		Width:  480,
		Height: 360,
	})
	poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.Settings", Title: config.Title, Width: config.Width, Height: config.Height})
}

func main() {}
