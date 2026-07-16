package main

import (
	"log"

	"github.com/mulavdm/poem/examples/preferences"
	"github.com/mulavdm/poem/pkg/app/web"
)

func main() {
	log.Println("Preferences listening on http://127.0.0.1:8091")
	if err := web.Run(preferences.App, "127.0.0.1:8091", "Preferences"); err != nil {
		log.Fatal(err)
	}
}
