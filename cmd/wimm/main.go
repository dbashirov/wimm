package main

import (
	"fmt"
	"wimm/configs"
	"wimm/internal/apiserver"
)

func main() {
	cs, err := config.GetConfig()
	if err != nil {
		fmt.Printf("Unmarshal config error: #%v ", err)
	}
	apiserver.Start(cs)
}
