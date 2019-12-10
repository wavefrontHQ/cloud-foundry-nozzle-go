package main

import (
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/legacy"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

func main() {
	conf, err := config.ParseConfig()
	if err != nil {
		utils.Logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}

	if conf.Nozzle.Legacy {
		utils.Logger.Println("Using deprecated v1 Cloud Foundry API")
		legacy.Run(conf)
	} else {
		nozzle.Run(conf)
	}
}
