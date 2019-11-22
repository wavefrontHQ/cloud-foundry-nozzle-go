package main

import (
	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/legacy"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

func main() {
	conf, err := common.ParseConfig()
	if err != nil {
		common.Logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}

	if conf.Nozzle.Legacy {
		common.Logger.Println("Using LEGACY mode")
		legacy.Run(conf)
	} else {
		nozzle.Run(conf)
	}
}
