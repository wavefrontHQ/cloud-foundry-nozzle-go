package legacy

import (
	"crypto/tls"
	"reflect"

	"github.com/cloudfoundry/noaa/consumer"
	noaaerrors "github.com/cloudfoundry/noaa/errors"
	"github.com/gorilla/websocket"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
)

func Run(conf *common.Config) {
	nozzle := NewNozzle(conf)

	for {
		var trafficControllerURL string
		common.Logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)

		api, err := common.NewAPIClient(conf.Nozzle)
		if err != nil {
			common.Logger.Fatal("[ERROR] Unable to build API client: ", err)
		}

		nozzle.APIClient = api

		token, err := api.FetchAuthToken()
		if err != nil {
			common.Logger.Fatal("[ERROR] Unable to fetch token via API: ", err)
		}

		trafficControllerURL = api.FetchTrafficControllerURL()
		if trafficControllerURL == "" {
			common.Logger.Fatal("[ERROR] trafficControllerURL from client was blank")
		}

		common.Logger.Printf("Consuming firehose: %v\n", trafficControllerURL)
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
		common.Logger.Println("Reconnecting")
	}
}

func printError(err error) {
	if retryErr, ok := err.(noaaerrors.RetryError); ok {
		err = retryErr.Err
	}

	switch closeErr := err.(type) {
	case *websocket.CloseError:
		common.Logger.Printf("Error from firehose - code:'%v' - Text:'%v' - %v", closeErr.Code, closeErr.Text, err)
	default:
		common.Logger.Printf("Error from firehose - %v (%v)", err, reflect.TypeOf(err))
	}
}
