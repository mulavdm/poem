package main

import (
	"log"

	"github.com/mulavdm/poem/examples/settings"
	"github.com/mulavdm/poem/pkg/app/web"
)

func main() {
	log.Println("Settings listening on http://127.0.0.1:8092")
	if err := web.Run(settings.App, "127.0.0.1:8092", "Settings"); err != nil {
		log.Fatal(err)
	}
}
