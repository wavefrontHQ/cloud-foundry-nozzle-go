package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"reflect"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	noaaerrors "github.com/cloudfoundry/noaa/errors"
	"github.com/gorilla/websocket"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/nozzle"
)

var logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var debug = os.Getenv("WAVEFRONT_DEBUG") == "true"

var allSelectors = []*loggregator_v2.Selector{
	{
		Message: &loggregator_v2.Selector_Log{
			Log: &loggregator_v2.LogSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Counter{
			Counter: &loggregator_v2.CounterSelector{},
		},
	},
	{
		Message: &loggregator_v2.Selector_Gauge{
			Gauge: &loggregator_v2.GaugeSelector{},
		},
	},
	// {
	// 	Message: &loggregator_v2.Selector_Timer{
	// 		Timer: &loggregator_v2.TimerSelector{},
	// 	},
	// },
	{
		Message: &loggregator_v2.Selector_Event{
			Event: &loggregator_v2.EventSelector{},
		},
	},
}

func main() {
	conf, err := nozzle.ParseConfig()
	if err != nil {
		logger.Fatal("[ERROR] Unable to build config from environment: ", err)
	}
	logger.Printf("Forwarding events: %s", conf.Nozzle.SelectedEvents)

	for {
		uaaClient, err := nozzle.NewUAA(conf.Nozzle.APIURL, conf.Nozzle.Username, conf.Nozzle.Password, true)
		if err != nil {
			panic(err)
		}

		err = receive(conf, uaaClient)
		if err != nil {
			logger.Println("ERROR !!!", err)
		}
		logger.Println("Reconnecting")
	}
}

func receive(conf *nozzle.Config, uaaClient nozzle.UAA) error {
	wfNozzle := nozzle.NewNozzle(conf)

	token, err := uaaClient.GetAuthToken()
	if err != nil {
		return err
	}

	c := loggregator.NewRLPGatewayClient(
		conf.Nozzle.LogStreamUrl,
		loggregator.WithRLPGatewayClientLogger(logger),
		loggregator.WithRLPGatewayHTTPClient(&tokenAttacher{
			token: token,
		}),
	)

	es := c.Stream(context.Background(), &loggregator_v2.EgressBatchRequest{
		Selectors: allSelectors,
	})

	// marshaler := jsonpb.Marshaler{}

	for {
		for _, e := range es() {
			wfNozzle.EventsChannel <- e
			// log.Printf("---> %v\n", reflect.TypeOf(e.GetMessage()))
			// log.Printf("%+v\n", e.GetMessage())
		}
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

type tokenAttacher struct {
	token string
}

func (a *tokenAttacher) Do(req *http.Request) (*http.Response, error) {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	req.Header.Set("Authorization", a.token)
	return client.Do(req)
}
