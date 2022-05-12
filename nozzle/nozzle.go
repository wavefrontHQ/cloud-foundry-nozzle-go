package nozzle

import (
	"os"
	"strings"

	"code.cloudfoundry.org/go-loggregator/v8/rpc/loggregator_v2"
	"github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/wavefront"
)

// Nozzle will read all CF events and sent it to the Forwarder
type Nozzle struct {
	eventsChannel chan *loggregator_v2.Envelope

	prefix     string
	foundation string

	numGaugeMetricReceived  metrics.Counter
	numCounterEventReceived metrics.Counter

	done chan struct{}

	wf                  wavefront.Wavefront
	Api                 api.Client
	enableAppTagLookups bool
}

var translateStrs = map[string]string{
	"pcf.container.rep.cpu.percentage":     "pcf.container.rep.cpu_percentage",
	"pcf.container.rep.disk.bytes":         "pcf.container.rep.disk_bytes",
	"pcf.container.rep.disk_quota.bytes":   "pcf.container.rep.disk_bytes_quota",
	"pcf.container.rep.memory.bytes":       "pcf.container.rep.memory_bytes",
	"pcf.container.rep.memory_quota.bytes": "pcf.container.rep.memory_bytes_quota",
}

var trace = os.Getenv("WAVEFRONT_TRACE") == "true"

// NewNozzle create a new Nozzle
func NewNozzle(conf *config.Config, eventsChannel chan *loggregator_v2.Envelope) *Nozzle {
	internalTags := utils.GetInternalTags()
	utils.Logger.Printf("internalTags: %v", internalTags)

	numGaugeMetricReceived := utils.NewCounter("gauge-metric-received", internalTags)
	numCounterEventReceived := utils.NewCounter("counter-event-received", internalTags)

	nozzle := &Nozzle{
		wf:                  wavefront.NewWavefront(conf.Wavefront),
		enableAppTagLookups: conf.Nozzle.EnableAppCache,
		eventsChannel:       eventsChannel,

		numGaugeMetricReceived:  numGaugeMetricReceived,
		numCounterEventReceived: numCounterEventReceived,

		prefix:     strings.Trim(conf.Wavefront.Prefix, " "),
		foundation: strings.Trim(conf.Wavefront.Foundation, " "),
	}

	go nozzle.run()
	return nozzle
}

func (nozzle *Nozzle) Stop() {
	close(nozzle.done)
}

func (nozzle *Nozzle) run() {
	for {
		select {
		case event := <-nozzle.eventsChannel:
			nozzle.handleEvent(event)
		case <-nozzle.done:
			return
		}
	}
}

func (nozzle *Nozzle) handleEvent(envelope *loggregator_v2.Envelope) {
	switch envelope.GetMessage().(type) {
	case *loggregator_v2.Envelope_Counter:
		nozzle.BuildCounterEvent(envelope)
	case *loggregator_v2.Envelope_Gauge:
		nozzle.BuildGaugeEvent(envelope)
	default:
		// utils.Logger.Printf("---> %v\n", envelope)
		// utils.Logger.Printf("---> %v\n", envelope.GetMessage())
		// utils.Logger.Panicf("---> %v\n", reflect.TypeOf(envelope.GetMessage()))
	}
}

//BuildCounterEvent parse and report metrics
func (nozzle *Nozzle) BuildCounterEvent(event *loggregator_v2.Envelope) {
	nozzle.numCounterEventReceived.Inc(1)

	metricName := nozzle.prefix
	if len(event.GetTags()["origin"]) > 0 {
		metricName += "." + event.GetTags()["origin"]
	}
	metricName += "." + event.GetCounter().GetName()

	source, tags, ts := nozzle.getMetricInfo(event)

	total := event.GetCounter().GetTotal()
	delta := event.GetCounter().GetDelta()

	nozzle.wf.SendMetric(metricName+".total", float64(total), ts, source, tags)
	nozzle.wf.SendMetric(metricName+".delta", float64(delta), ts, source, tags)
}

func (nozzle *Nozzle) BuildGaugeEvent(event *loggregator_v2.Envelope) {
	nozzle.numGaugeMetricReceived.Inc(1)

	for name, metric := range event.GetGauge().GetMetrics() {
		translate := false
		metricName := nozzle.prefix
		if _, ok := event.GetTags()["source_id"]; ok {
			metricName += ".container"
			translate = true
		}

		if len(event.GetTags()["origin"]) > 0 {
			metricName += "." + event.GetTags()["origin"]
		}

		metricName += "." + name
		if len(metric.GetUnit()) > 0 {
			metricName += "." + metric.GetUnit()
		}

		if translate {
			if newName, ok := translateStrs[metricName]; ok {
				metricName = newName
			}
		}

		source, tags, ts := nozzle.getMetricInfo(event)
		nozzle.wf.SendMetric(metricName, metric.Value, ts, source, tags)
	}
}

func (nozzle *Nozzle) getMetricInfo(event *loggregator_v2.Envelope) (string, map[string]string, int64) {
	source := nozzle.getSource(event)
	tags := nozzle.getTags(event)

	return source, tags, event.GetTimestamp()
}

func (nozzle *Nozzle) getSource(event *loggregator_v2.Envelope) string {
	source := event.GetTags()["ip"]
	if len(source) == 0 {
		source = event.GetTags()["job"]
		if len(source) == 0 {
			hostName, err := os.Hostname()
			if err == nil {
				source = hostName
			} else {
				source = "unknown"
			}
		}
	}
	return source
}

func (nozzle *Nozzle) getTags(event *loggregator_v2.Envelope) map[string]string {
	tags := make(map[string]string)

	if deployment, ok := event.GetTags()["deployment"]; ok {
		tags["deployment"] = deployment
	}

	if job, ok := event.GetTags()["job"]; ok {
		tags["job"] = job
	}

	if nozzle.Api != nil {
		if event.GetTags()["origin"] == "rep" {
			if appName, ok := event.GetTags()["app_name"]; ok {
				tags["applicationName"] = appName
				tags["org"] = event.GetTags()["organization_name"]
				tags["space"] = event.GetTags()["space_name"]
			} else if sourceID, ok := event.GetTags()["source_id"]; ok && nozzle.enableAppTagLookups {
				app := nozzle.Api.GetApp(sourceID)
				if app != nil {
					tags["applicationName"] = app.Name
					tags["org"] = app.Org
					tags["space"] = app.Space
				}
			}
		}
	}

	tags["foundation"] = nozzle.foundation

	for k, v := range event.GetTags() {
		if len(k) > 0 && len(v) > 0 {
			tags[k] = v
		}
	}

	delete(tags, "app_name")
	delete(tags, "organization_name")
	delete(tags, "space_name")

	return tags
}
