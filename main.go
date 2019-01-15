package main

import (
	"crypto/tls"
	"log"
	"os"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/wavefrontnozzle"
)

func main() {
	logger := log.New(os.Stdout, ">>> ", 0)

	conf, err := config.Parse()
	if err != nil {
		logger.Fatal("Unable to build config from environment", err)
	}

	var token, trafficControllerURL string
	logger.Printf("Fetching auth token via API: %v\n", conf.Nozzel.APIURL)

	fetcher, err := api.NewAPIClient(conf.Nozzel.APIURL, conf.Nozzel.Username, conf.Nozzel.Password, conf.Nozzel.SkipSSL)
	if err != nil {
		logger.Fatal("Unable to build API client", err)
	}
	token, err = fetcher.FetchAuthToken()
	if err != nil {
		logger.Fatal("Unable to fetch token via API", err)
	}

	trafficControllerURL = fetcher.FetchTrafficControllerURL()
	if trafficControllerURL == "" {
		logger.Fatal("trafficControllerURL from client was blank")
	}

	logger.Printf("Consuming firehose: %v\n", trafficControllerURL)
	noaaConsumer := consumer.New(trafficControllerURL, &tls.Config{
		InsecureSkipVerify: conf.Nozzel.SkipSSL,
	}, nil)
	events, errs := noaaConsumer.Firehose(conf.Nozzel.FirehoseSubscriptionID, token)

	wavefront := wavefrontnozzle.CreateWavefrontEventHandler(conf.WaveFront)

	logger.Printf("Forwarding events: %s", conf.Nozzel.SelectedEvents)
	forwarder := nozzle.NewForwarder(fetcher, wavefront, conf.Nozzel.SelectedEvents, events, errs, logger)
	err = forwarder.Run()
	if err != nil {
		logger.Fatal("Error forwarding", err)
	}
}
