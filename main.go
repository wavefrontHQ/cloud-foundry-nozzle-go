package main

import (
	"crypto/tls"
	"log"
	"os"
	"reflect"

	"github.com/cloudfoundry/noaa/consumer"
	noaaerrors "github.com/cloudfoundry/noaa/errors"
	"github.com/gorilla/websocket"
	wfnozzle "github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

var logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var debug = os.Getenv("WAVEFRONT_DEBUG") == "true"

func main() {
	conf, err := wfnozzle.ParseConfig()
	if err != nil {
		logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}
	logger.Printf("Forwarding events: %s", conf.Nozzle.SelectedEvents)

	nozzle := wfnozzle.NewNozzle(conf)

	for {
		var trafficControllerURL string
		logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)

		api, err := wfnozzle.NewAPIClient(conf.Nozzle)
		if err != nil {
			logger.Fatal("[ERROR] Unable to build API client: ", err)
		}

		nozzle.APIClient = api

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
		events, errs := noaaConsumer.FirehoseWithoutReconnect(conf.Nozzle.FirehoseSubscriptionID, token)

		done := make(chan struct{})
		go func() {
			for {
				select {
				case event := <-events:
					nozzle.EventsChannel <- event
				case err := <-errs:
					printError(err)
					nozzle.ErrorsChannel <- err
					close(done)
					return
				}
			}
		}()
		<-done

		noaaConsumer.Close()
		logger.Println("Reconnecting")
	}
}

func printError(err error) {
	if retryErr, ok := err.(noaaerrors.RetryError); ok {
		err = retryErr.Err
	}

	switch closeErr := err.(type) {
	case *websocket.CloseError:
		logger.Printf("Error from firehose - code:'%v' - Text:'%v' - %v", closeErr.Code, closeErr.Text, err)
	default:
		logger.Printf("Error from firehose - %v (%v)", err, reflect.TypeOf(err))
	}
}
