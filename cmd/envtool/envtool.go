package main

import (
	"context"
	"log"
	"wimm/config"
	"wimm/pkg/client/postgresql"
)

func main() {

	log.Println("Waitng coonect DB")
	sc := config.StorageConfig{
		Host:     "localhost",
		Port:     "5432",
		Database: "wimm",
		Username: "wimm",
		Password: "wimm",
	}

	_, err := postgresql.NewClient(context.Background(), sc, 10)
	if err != nil {
		log.Fatalln("failed to connect DB")
	}
}
