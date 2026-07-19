//go:build windows

package main

import (
	"github.com/mulavdm/poem/pkg/render"
	poemwindows "github.com/mulavdm/poem/pkg/windows"

	"github.com/mulavdm/poem/examples/counter"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

func init() {
	config := desktop.Configure(counter.App, render.AppConfig{
		Title:  "Counter",
		Width:  480,
		Height: 320,
	})
	poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.Counter", Title: config.Title, Width: config.Width, Height: config.Height})
}

func main() {}
