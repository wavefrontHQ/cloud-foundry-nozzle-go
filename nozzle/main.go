package nozzle

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	loggregator "code.cloudfoundry.org/go-loggregator/v8"
	"code.cloudfoundry.org/go-loggregator/v8/rpc/loggregator_v2"
	"github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
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

var (
	eventsChannel chan *loggregator_v2.Envelope
	errorsChannel chan error
	puts          = metrics.NewCounter()
	drops         = metrics.NewCounter()
)

func Run(conf *config.Config) {
	eventsChannel = make(chan *loggregator_v2.Envelope, conf.Nozzle.ChannelSize)
	errorsChannel = make(chan error)

	reporting.RegisterMetric("nozzle.queue.size", metrics.NewFunctionalGauge(queueSize), utils.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.used", metrics.NewFunctionalGauge(queueUsed), utils.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.puts", puts, utils.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.drops", drops, utils.GetInternalTags())

	var nozzles []*Nozzle
	for i := 0; i < conf.Nozzle.Workers; i++ {
		nozzles = append(nozzles, NewNozzle(conf, eventsChannel))
	}

	for {
		utils.Logger.Printf("Fetching auth token via UAA: %v\n", conf.Nozzle.APIURL)
		api, err := api.NewAPIClient(conf.Nozzle)
		if err != nil {
			utils.Logger.Fatal("[ERROR] Unable to build API client: ", err)
		}

		for _, nozzle := range nozzles {
			nozzle.Api = api
		}

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
					select {
					case eventsChannel <- e:
						puts.Inc(1)
					default:
						drops.Inc(1)
					}
				}
				if ctx.Err() != nil {
					return
				}
			}
		}()

		<-ctx.Done()
	}
}

func queueSize() int64 {
	return int64(cap(eventsChannel))
}

func queueUsed() int64 {
	return int64(len(eventsChannel))
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
