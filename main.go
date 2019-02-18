package main

import (
	"crypto/tls"
	"log"
	"os"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

func main() {
	logger := log.New(os.Stdout, "[WAVEFRONT] ", 0)

	conf, err := nozzle.ParseConfig()
	if err != nil {
		logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}

	var token, trafficControllerURL string
	logger.Printf("Fetching auth token via API: %v\n", conf.Nozzel.APIURL)

	fetcher, err := nozzle.NewAPIClient(conf.Nozzel.APIURL, conf.Nozzel.Username, conf.Nozzel.Password, conf.Nozzel.SkipSSL)
	if err != nil {
		logger.Fatal("[ERROR] Unable to build API client", err)
	}
	token, err = fetcher.FetchAuthToken()
	if err != nil {
		logger.Fatal("[ERROR] Unable to fetch token via API", err)
	}

	trafficControllerURL = fetcher.FetchTrafficControllerURL()
	if trafficControllerURL == "" {
		logger.Fatal("[ERROR] trafficControllerURL from client was blank")
	}

	logger.Printf("Consuming firehose: %v\n", trafficControllerURL)
	noaaConsumer := consumer.New(trafficControllerURL, &tls.Config{
		InsecureSkipVerify: conf.Nozzel.SkipSSL,
	}, nil)
	events, errs := noaaConsumer.Firehose(conf.Nozzel.FirehoseSubscriptionID, token)

	wavefront := nozzle.CreateEventHandler(conf.WaveFront)

	logger.Printf("Forwarding events: %s", conf.Nozzel.SelectedEvents)
	forwarder := nozzle.NewNozzle(fetcher, wavefront, conf.Nozzel.SelectedEvents, events, errs, logger)
	err = forwarder.Run()
	if err != nil {
		logger.Fatal("[ERROR] Error forwarding", err)
	}
}
