package main

import (
	"log"

	"github.com/mateoferrari97/belo-challenge/internal/platform/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	srv := web.NewServer()
	return srv.Run()
}
