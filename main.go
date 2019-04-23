package main

import (
	"crypto/tls"
	"log"
	"os"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

var logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var debug = os.Getenv("WAVEFRONT_DEBUG") == "true"

func main() {
	if debug {
		for _, pair := range os.Environ() {
			logger.Println("env:", pair)
		}
	}

	conf, err := nozzle.ParseConfig()
	if err != nil {
		logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}

	var trafficControllerURL string
	logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)

	// Set up connection to PAS API using the token we got
	api, err := nozzle.NewAPIClient(conf.Nozzle)
	if err != nil {
		logger.Fatal("[ERROR] Unable to build API client: ", err)
	}

	token, err := api.FetchAuthToken()
	if err != nil {
		logger.Fatal("[ERROR] Unable to fetch token via API: ", err)
	}
	trafficControllerURL = api.FetchTrafficControllerURL()
	if trafficControllerURL == "" {
		logger.Fatal("[ERROR] trafficControllerURL from client was blank")
	}

	logger.Printf("Consuming firehose: %v\n", trafficControllerURL)
	noaaConsumer := consumer.New(trafficControllerURL, &tls.Config{
		InsecureSkipVerify: conf.Nozzle.SkipSSL,
	}, nil)
	events, errs := noaaConsumer.Firehose(conf.Nozzle.FirehoseSubscriptionID, token)

	wavefront := nozzle.CreateEventHandler(conf.Wavefront)

	logger.Printf("Forwarding events: %s", conf.Nozzle.SelectedEvents)
	forwarder := nozzle.NewNozzle(api, wavefront, conf.Nozzle.SelectedEvents, events, errs, logger)
	err = forwarder.Run()
	if err != nil {
		logger.Fatal("[ERROR] Error forwarding", err)
	}
}
