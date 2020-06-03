package nozzle

import (
	"context"
	"crypto/tls"
	"net/http"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
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
)

func Run(conf *config.Config) {
	eventsChannel = make(chan *loggregator_v2.Envelope, conf.Nozzle.ChannelSize)
	errorsChannel = make(chan error)

	reporting.RegisterMetric("nozzle.queue.size", metrics.NewFunctionalGauge(queueSize), utils.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.used", metrics.NewFunctionalGauge(queueUsed), utils.GetInternalTags())
	reporting.RegisterMetric("nozzle.queue.puts", puts, utils.GetInternalTags())

	var nozzles []*Nozzle
	for i := 0; i < conf.Nozzle.Workers; i++ {
		nozzles = append(nozzles, NewNozzle(conf, eventsChannel))
	}

	for {
		api, err := api.NewAPIClient(conf.Nozzle)
		if err != nil {
			utils.Logger.Fatal("[ERROR] Unable to build API client: ", err)
		}

		for _, nozzle := range nozzles {
			nozzle.Api = api
		}

		ctx, cancel := context.WithCancel(context.Background())

		c := loggregator.NewRLPGatewayClient(
			conf.Nozzle.LogStreamURL,
			loggregator.WithRLPGatewayClientLogger(utils.Logger),
			loggregator.WithRLPGatewayHTTPClient(&tokenAttacher{
				api:    api,
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
					puts.Inc(1)
					eventsChannel <- e
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
	api    *api.APIClient
	cancel context.CancelFunc
}

func (a *tokenAttacher) Do(req *http.Request) (*http.Response, error) {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	utils.Logger.Println("Getting token")
	token, err := a.api.FetchAuthToken()
	if err != nil {
		a.cancel()
		return nil, err
	}

	req.Header.Set("Authorization", token)

	res, err := client.Do(req)
	if err != nil {
		a.cancel()
	}
	return res, err
}
