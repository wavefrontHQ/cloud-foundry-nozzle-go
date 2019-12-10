package wavefront

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/filter"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
)

var trace = os.Getenv("WAVEFRONT_TRACE") == "true"

type Wavefront interface {
	SendMetric(name string, value float64, ts int64, source string, tags map[string]string)
	ReportError(err error)
}

type wavefront struct {
	sender   senders.Sender
	reporter reporting.WavefrontMetricsReporter
	filter   filter.Filter

	numMetricsSent     metrics.Counter
	metricsSendFailure metrics.Counter
	metricsFiltered    metrics.Counter
	handleErrorMetric  metrics.Counter
}

func NewWavefront(conf *config.WavefrontConfig) Wavefront {
	var sender senders.Sender
	var err error

	if len(conf.ProxyAddr) == 0 {
		conf.ProxyAddr = os.Getenv("PROXY_CONN_HOST")
	}

	if len(conf.URL) > 0 && len(conf.Token) > 0 {
		utils.Logger.Printf("Direct connetion to Wavefront: %s", conf.URL)
		directCfg := &senders.DirectConfiguration{
			Server:               strings.Trim(conf.URL, " "),
			Token:                strings.Trim(conf.Token, " "),
			BatchSize:            conf.BatchSize,
			MaxBufferSize:        conf.MaxBufferSize,
			FlushIntervalSeconds: conf.FlushInterval,
		}
		sender, err = senders.NewDirectSender(directCfg)
		if err != nil {
			utils.Logger.Fatal(err)
		}
	} else if len(conf.ProxyAddr) > 0 && conf.ProxyPort > 0 {
		utils.Logger.Printf("Connecting to Wavefront proxy: '%s:%d'", conf.ProxyAddr, conf.ProxyPort)
		proxyCfg := &senders.ProxyConfiguration{
			Host:                 strings.Trim(conf.ProxyAddr, " "),
			MetricsPort:          conf.ProxyPort,
			FlushIntervalSeconds: conf.FlushInterval,
		}
		sender, err = senders.NewProxySender(proxyCfg)
		if err != nil {
			utils.Logger.Fatal(err)
		}
	} else {
		utils.Logger.Printf("Direct configuration: %s", conf.URL)
		utils.Logger.Printf("Proxy configuration: '%s:%d'", conf.ProxyAddr, conf.ProxyPort)
		utils.Logger.Fatal(errors.New("No Wavefront configuration detected"))
	}

	internalTags := utils.GetInternalTags()
	utils.Logger.Printf("internalTags: %v", internalTags)

	numMetricsSent := NewCounter("total-metrics-sent", internalTags)
	metricsSendFailure := NewCounter("metrics-send-failure", internalTags)
	metricsFiltered := NewCounter("metrics-filtered", internalTags)
	handleErrorMetric := NewCounter("firehose-connection-error", internalTags)

	reporter := reporting.NewReporter(
		sender,
		application.New("pcf-nozzle", "internal-metrics"),
		reporting.Prefix("wavefront-firehose-nozzle.app"),
	)

	wf := &wavefront{
		sender:             sender,
		filter:             filter.NewGlobFilter(conf.Filters),
		reporter:           reporter,
		numMetricsSent:     numMetricsSent,
		metricsSendFailure: metricsSendFailure,
		metricsFiltered:    metricsFiltered,
		handleErrorMetric:  handleErrorMetric,
	}
	wf.startHealthReport()
	return wf
}

func (w *wavefront) SendMetric(name string, value float64, ts int64, source string, tags map[string]string) {
	if trace {
		line, err := senders.MetricLine(name, value, ts, source, tags, "")
		if err != nil {
			utils.Logger.Printf("[ERROR] error preparing the metric '%s': %v", name, err)
		}

		status := "filtered"
		if w.filter.Match(name, tags) {
			status = "accepted"
		}
		utils.Logger.Printf("[DEBUG] [%s] metric: %s", status, line)
	}

	if w.filter.Match(name, tags) {
		err := w.sender.SendMetric(name, value, ts, source, tags)
		if err != nil {
			w.metricsSendFailure.Inc(1)
			if utils.Debug {
				utils.Logger.Printf("[ERROR] error sending the metric '%s': %v", name, err)
			}
		} else {
			w.numMetricsSent.Inc(1)
		}
	} else {
		w.metricsFiltered.Inc(1)
	}
}

func (w *wavefront) startHealthReport() {
	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			utils.Logger.Printf("total metrics sent: %d  filtered: %d  failures: %d", w.numMetricsSent.Count(), w.metricsFiltered.Count(), w.metricsSendFailure.Count())
		}
	}()
}

//ReportError increments the error counter
func (w *wavefront) ReportError(err error) {
	w.handleErrorMetric.Inc(1)
}

func NewCounter(name string, tags map[string]string) metrics.Counter {
	return reporting.GetOrRegisterMetric(name, metrics.NewCounter(), tags).(metrics.Counter)
}
