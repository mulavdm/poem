package main

import (
	"github.com/mulavdm/poem/pkg/render"
)

func main() {
	render.Run(render.AppConfig{
		Title:        "P.O.E.M. Operational Engine Matrix (Modular)",
		Width:        1024,
		Height:       768,
		BuildPagesFn: BuildAllPages,
	})
}
