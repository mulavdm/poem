//go:build windows

package main

import (
	"github.com/mulavdm/poem/pkg/render"
	poemwindows "github.com/mulavdm/poem/pkg/windows"
)

func init() {
	config := render.AppConfig{
		Title:        "P.O.E.M. Operational Engine Matrix (Modular)",
		Width:        1024,
		Height:       768,
		BuildPagesFn: BuildAllPages,
	}
	poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.Engine", Title: config.Title, Width: config.Width, Height: config.Height})
}

func main() {}
