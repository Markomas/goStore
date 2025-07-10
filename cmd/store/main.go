package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"store/internal"
	"store/internal/server"
)

func main() {

	configPath := flag.String("config", "config/config.yml", "path to config file (yaml or toml)")
	flag.Parse()
	f, err := os.Open(*configPath)
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
