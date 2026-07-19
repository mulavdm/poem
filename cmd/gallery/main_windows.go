//go:build windows

package main

import (
	"github.com/mulavdm/poem/pkg/render"
	poemwindows "github.com/mulavdm/poem/pkg/windows"
)

func init() {
	config := render.AppConfig{
		Title:        "POEM Component Gallery",
		Width:        1180,
		Height:       780,
		Theme:        themes,
		BuildPagesFn: buildGallery,
		Automation:   &render.AutomationConfig{Enabled: true, Mode: "http", Host: "127.0.0.1", Port: 47831},
	}
	poemwindows.MustRegister(config, poemwindows.Metadata{Identity: "POEM.Gallery", Title: config.Title, Width: config.Width, Height: config.Height})
}

func main() {}
