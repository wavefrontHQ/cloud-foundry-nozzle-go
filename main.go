package main

import (
	"crypto/tls"
	"log"
	"os"
	"reflect"

	"github.com/cloudfoundry/noaa/consumer"
	noaaerrors "github.com/cloudfoundry/noaa/errors"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
	"github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
	wfnozzle "github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

var logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var debug = os.Getenv("WAVEFRONT_DEBUG") == "true"

var (
	eventsChannel chan *events.Envelope
	errorsChannel chan error
	puts          = metrics.NewCounter()
)

func main() {
	conf, err := wfnozzle.ParseConfig()
	if err != nil {
		logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}
	logger.Printf("Forwarding events: %s", conf.Nozzle.SelectedEvents)

	eventsChannel = make(chan *events.Envelope, conf.Nozzle.ChannelSize)
	errorsChannel = make(chan error)

	reporting.RegisterMetric("nozzle.queue.size", metrics.NewFunctionalGauge(queueSize), nozzle.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.used", metrics.NewFunctionalGauge(queueUsed), nozzle.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.puts", puts, nozzle.GetInternalTags())

	var nozzles []*nozzle.Nozzle
	for i := 0; i < conf.Nozzle.Workers; i++ {
		nozzles = append(nozzles, wfnozzle.NewNozzle(conf, eventsChannel, errorsChannel))
	}

	for {
		var trafficControllerURL string
		logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)

		api, err := wfnozzle.NewAPIClient(conf.Nozzle)
		if err != nil {
			logger.Fatal("[ERROR] Unable to build API client: ", err)
		}

		for _, nozzle := range nozzles {
			nozzle.APIClient = api
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
		events, errs := noaaConsumer.FirehoseWithoutReconnect(conf.Nozzle.FirehoseSubscriptionID, token)

		done := make(chan struct{})
		go func() {
			for {
				select {
				case event := <-events:
					puts.Inc(1)
					eventsChannel <- event
				case err := <-errs:
					printError(err)
					errorsChannel <- err
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

func queueSize() int64 {
	return int64(cap(eventsChannel))
}

func queueUsed() int64 {
	return int64(len(eventsChannel))
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
