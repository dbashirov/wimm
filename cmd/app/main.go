package main

import (
	"fmt"
	"wimm/config"
	"wimm/internal/app"
)

func main() {
	cs, err := config.GetConfig()
	if err != nil {
		fmt.Printf("Unmarshal config error: #%v ", err)
	}
	app.Start(cs)
}
