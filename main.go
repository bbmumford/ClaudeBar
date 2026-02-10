package main

import (
	"claudebar/internal/app"
	"log"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
