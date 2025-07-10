package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"store/internal"
	"store/internal/server"
)

func main() {

	f, err := os.Open("config/config.yml")
	if err != nil {
		processError(err)
	}
	defer f.Close()

	var cfg internal.Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		processError(err)
	}

	server.Run(cfg)
}

func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}
