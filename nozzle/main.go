package nozzle

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
)

var allSelectors = []*loggregator_v2.Selector{
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
}

func Run(conf *config.Config) {
	wfnozzle := NewNozzle(conf)

	for {
		utils.Logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)
		api, err := api.NewAPIClient(conf.Nozzle)
		if err != nil {
			utils.Logger.Fatal("[ERROR] Unable to build API client: ", err)
		}

		wfnozzle.Api = api

		token, err := api.FetchAuthToken()
		if err != nil {
			utils.Logger.Fatal("[ERROR] Unable to fetch token via API: ", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		c := loggregator.NewRLPGatewayClient(
			conf.Nozzle.LogStreamURL,
			loggregator.WithRLPGatewayClientLogger(utils.Logger),
			loggregator.WithRLPGatewayHTTPClient(&tokenAttacher{
				token:  token,
				cancel: cancel,
			}),
		)

		es := c.Stream(ctx, &loggregator_v2.EgressBatchRequest{
			Selectors: allSelectors,
			ShardId:   conf.Nozzle.FirehoseSubscriptionID,
		})

		go func() {
			for {
				for _, e := range es() {
					wfnozzle.EventsChannel <- e
				}
				if ctx.Err() != nil {
					return
				}
			}
		}()

		<-ctx.Done()
	}
}

type tokenAttacher struct {
	token  string
	cancel context.CancelFunc
}

func (a *tokenAttacher) Do(req *http.Request) (*http.Response, error) {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	req.Header.Set("Authorization", a.token)

	res, err := client.Do(req)
	if err == nil {
		if res.StatusCode == 404 {
			a.cancel()
			return nil, fmt.Errorf("Token expired, reconnecting")
		}
	}
	return res, err
}
