package main

import (
	"log"
	"wimm/config"
	"wimm/internal/app"
)

func main() {
	log.Print("config initializing")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("Unmarshal config error: #%v ", err)
	}

	a, err := app.NewApp(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Running Application")
	a.Run()
}
