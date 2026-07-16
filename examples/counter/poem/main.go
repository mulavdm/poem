package main

import (
	"github.com/mulavdm/poem/pkg/render"

	"github.com/mulavdm/poem/examples/counter"
	desktop "github.com/mulavdm/poem/pkg/app/desktop"
)

func main() {
	desktop.Run(counter.App, render.AppConfig{
		Title:  "Counter",
		Width:  480,
		Height: 320,
	})
}
