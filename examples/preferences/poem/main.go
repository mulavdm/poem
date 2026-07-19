//go:build windows

package main

import (
	"github.com/mulavdm/poem/pkg/render"
	poemwindows "github.com/mulavdm/poem/pkg/windows"

	"github.com/mulavdm/poem/examples/preferences"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

func init() {
	config := desktop.Configure(preferences.App, render.AppConfig{
		Title:  "Preferences",
		Width:  480,
		Height: 360,
	})
	poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.Preferences", Title: config.Title, Width: config.Width, Height: config.Height})
}

func main() {}
