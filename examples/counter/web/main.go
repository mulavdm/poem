package main

import (
	"log"

	"github.com/mulavdm/poem/examples/counter"
	"github.com/mulavdm/poem/pkg/app/web"
)

func main() {
	log.Println("Counter listening on http://127.0.0.1:8090")
	if err := web.Run(counter.App, "127.0.0.1:8090", "Counter"); err != nil {
		log.Fatal(err)
	}
}
