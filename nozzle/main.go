package nozzle

import (
	"context"
	"crypto/tls"
	"net/http"
	"reflect"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	noaaerrors "github.com/cloudfoundry/noaa/errors"
	"github.com/gorilla/websocket"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
)

var allSelectors = []*loggregator_v2.Selector{
	// {
	// 	Message: &loggregator_v2.Selector_Log{
	// 		Log: &loggregator_v2.LogSelector{},
	// 	},
	// },
	// {
	// 	Message: &loggregator_v2.Selector_Counter{
	// 		Counter: &loggregator_v2.CounterSelector{},
	// 	},
	// },
	// {
	// 	Message: &loggregator_v2.Selector_Gauge{
	// 		Gauge: &loggregator_v2.GaugeSelector{},
	// 	},
	// },
	{
		Message: &loggregator_v2.Selector_Timer{
			Timer: &loggregator_v2.TimerSelector{},
		},
	},
	// {
	// 	Message: &loggregator_v2.Selector_Event{
	// 		Event: &loggregator_v2.EventSelector{},
	// 	},
	// },
}

func Run(conf *common.Config) {
	common.Logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)

	api, err := common.NewAPIClient(conf.Nozzle)
	if err != nil {
		common.Logger.Fatal("[ERROR] Unable to build API client: ", err)
	}

	token, err := api.FetchAuthToken()
	if err != nil {
		common.Logger.Fatal("[ERROR] Unable to fetch token via API: ", err)
	}

	wfnozzle := NewNozzle(conf, api)
	c := loggregator.NewRLPGatewayClient(
		conf.Nozzle.LogStreamURL,
		loggregator.WithRLPGatewayClientLogger(common.Logger),
		loggregator.WithRLPGatewayHTTPClient(&tokenAttacher{
			token: token,
		}),
	)

	es := c.Stream(context.Background(), &loggregator_v2.EgressBatchRequest{
		Selectors: allSelectors,
	})

	for {
		for _, e := range es() {
			wfnozzle.EventsChannel <- e
		}
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
