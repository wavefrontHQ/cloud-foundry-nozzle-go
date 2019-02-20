package nozzle

import (
	"errors"
	"log"
	"os"

	metrics "github.com/rcrowley/go-metrics"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
)

// EventHandler receive CF events and send metrics to WF
type EventHandler interface {
	BuildHTTPStartStopEvent(event *events.Envelope)
	BuildLogMessageEvent(event *events.Envelope)
	BuildValueMetricEvent(event *events.Envelope)
	BuildCounterEvent(event *events.Envelope)
	BuildErrorEvent(event *events.Envelope)
	BuildContainerEvent(event *events.Envelope, appInfo *AppInfo)
}

type eventHandlerImpl struct {
	sender     senders.Sender
	logger     *log.Logger
	reporter   reporting.WavefrontMetricsReporter
	prefix     string
	foundation string
	filter     Filter
	debug      bool

	numMetricsSent             metrics.Counter
	metricsSendFailure         metrics.Counter
	numValueMetricReceived     metrics.Counter
	numCounterEventReceived    metrics.Counter
	numContainerMetricReceived metrics.Counter
}

// CreateEventHandler create a new EventHandler
func CreateEventHandler(conf *WaveFrontConfig) EventHandler {
	var sender senders.Sender
	var err error
	logger := log.New(os.Stdout, ">>> ", 0)

	if conf.URL != "" {
		directCfg := &senders.DirectConfiguration{
			Server:               conf.URL,
			Token:                conf.Token,
			BatchSize:            10000,
			MaxBufferSize:        50000,
			FlushIntervalSeconds: conf.FlushInterval,
		}
		sender, err = senders.NewDirectSender(directCfg)
		if err != nil {
			logger.Fatal(err)
		}
	} else if conf.ProxyAddr != "" {
		proxyCfg := &senders.ProxyConfiguration{
			Host:                 conf.ProxyAddr,
			MetricsPort:          conf.ProxyPort,
			FlushIntervalSeconds: conf.FlushInterval,
		}
		sender, err = senders.NewProxySender(proxyCfg)
		if err != nil {
			logger.Fatal(err)
		}
	} else {
		logger.Fatal(errors.New("One of NOZZLE_WF_URL or NOZZLE_WF_PROXY are required"))
	}

	reporter := reporting.NewReporter(
		sender,
		application.New("pcf-nozzle", "internal-metrics"),
		reporting.Prefix("wavefront-firehose-nozzle.app"),
	)

	numMetricsSent := metrics.NewCounter()
	metrics.Register("total-metrics-sent", numMetricsSent)
	metricsSendFailure := metrics.NewCounter()
	metrics.Register("metrics-send-failure", metricsSendFailure)
	numValueMetricReceived := metrics.NewCounter()
	metrics.Register("value-metric-received", numValueMetricReceived)
	numCounterEventReceived := metrics.NewCounter()
	metrics.Register("counter-event-received", numCounterEventReceived)
	numContainerMetricReceived := metrics.NewCounter()
	metrics.Register("container-metric-received", numContainerMetricReceived)

	return &eventHandlerImpl{
		sender:                     sender,
		logger:                     logger,
		reporter:                   reporter,
		prefix:                     conf.Prefix,
		foundation:                 conf.Foundation,
		debug:                      conf.Debug,
		filter:                     NewGlobFilter(conf.Filters),
		numMetricsSent:             numMetricsSent,
		metricsSendFailure:         metricsSendFailure,
		numValueMetricReceived:     numValueMetricReceived,
		numCounterEventReceived:    numCounterEventReceived,
		numContainerMetricReceived: numContainerMetricReceived,
	}
}

func (w *eventHandlerImpl) BuildHTTPStartStopEvent(event *events.Envelope) {
	// genericSerializer(event)
}

func (w *eventHandlerImpl) BuildLogMessageEvent(event *events.Envelope) {
	// genericSerializer(event)
}

func (w *eventHandlerImpl) BuildValueMetricEvent(event *events.Envelope) {
	w.numValueMetricReceived.Inc(1)

	metricName := w.prefix
	metricName += "." + event.GetOrigin()
	metricName += "." + event.GetValueMetric().GetName()
	metricName += "." + event.GetValueMetric().GetUnit()
	source, tags, ts := w.getMetricInfo(event)

	value := event.GetValueMetric().GetValue()

	w.sendMetric(metricName, value, ts, source, tags)

	w.genericSerializer(event)
}

func (w *eventHandlerImpl) BuildCounterEvent(event *events.Envelope) {
	w.numCounterEventReceived.Inc(1)

	metricName := w.prefix
	metricName += "." + event.GetOrigin()
	metricName += "." + event.GetCounterEvent().GetName()
	source, tags, ts := w.getMetricInfo(event)

	total := event.GetCounterEvent().GetTotal()
	delta := event.GetCounterEvent().GetDelta()

	w.sendMetric(metricName+".total", float64(total), ts, source, tags)
	w.sendMetric(metricName+".delta", float64(delta), ts, source, tags)
}

func (w *eventHandlerImpl) BuildErrorEvent(event *events.Envelope) {
	// genericSerializer(event)
}

func (w *eventHandlerImpl) BuildContainerEvent(event *events.Envelope, appInfo *AppInfo) {
	w.numContainerMetricReceived.Inc(1)

	metricName := w.prefix + ".container." + event.GetOrigin()
	source, tags, ts := w.getMetricInfo(event)

	tags["applicationId"] = event.GetContainerMetric().GetApplicationId()
	tags["instanceIndex"] = string(event.GetContainerMetric().GetInstanceIndex())
	tags["applicationName"] = appInfo.Name
	tags["space"] = appInfo.Space
	tags["org"] = appInfo.Org

	cpuPercentage := event.GetContainerMetric().GetCpuPercentage()
	diskBytes := event.GetContainerMetric().GetDiskBytes()
	diskBytesQuota := event.GetContainerMetric().GetDiskBytesQuota()
	memoryBytes := event.GetContainerMetric().GetMemoryBytes()
	memoryBytesQuota := event.GetContainerMetric().GetMemoryBytesQuota()

	w.sendMetric(metricName+".cpu_percentage", cpuPercentage, ts, source, tags)
	w.sendMetric(metricName+".disk_bytes", float64(diskBytes), ts, source, tags)
	w.sendMetric(metricName+".disk_bytes_quota", float64(diskBytesQuota), ts, source, tags)
	w.sendMetric(metricName+".memory_bytes", float64(memoryBytes), ts, source, tags)
	w.sendMetric(metricName+".memory_bytes_quota", float64(memoryBytesQuota), ts, source, tags)
}

func (w *eventHandlerImpl) genericSerializer(event *events.Envelope) {
	// w.logger.Printf("Event: %v", event)
}

func (w *eventHandlerImpl) getMetricInfo(event *events.Envelope) (string, map[string]string, int64) {
	source := w.getSource(event)
	tags := w.getTags(event)

	return source, tags, event.GetTimestamp()
}

func (w *eventHandlerImpl) getSource(event *events.Envelope) string {
	source := event.GetIp()
	if len(source) == 0 {
		source = event.GetJob()
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

func (w *eventHandlerImpl) getTags(event *events.Envelope) map[string]string {
	tags := make(map[string]string)

	if deployment := event.GetDeployment(); len(deployment) > 0 {
		tags["deployment"] = deployment
	}

	if job := event.GetJob(); len(job) > 0 {
		tags["job"] = job
	}

	tags["foundation"] = w.foundation

	for k, v := range event.GetTags() {
		tags[k] = v
	}

	return tags
}

func (w *eventHandlerImpl) sendMetric(name string, value float64, ts int64, source string, tags map[string]string) {
	if w.debug {
		line, err := senders.MetricLine(name, value, ts, source, tags, "")
		if err != nil {
			log.Printf("[ERROR] error preparing the metric '%s': %v", name, err)
		}

		status := "dropped"
		if w.filter.Match(name, tags) {
			status = "acepted"
		}
		log.Printf("[DEBUG] [%s] metric: %s", status, line)
	}

	if w.filter.Match(name, tags) {
		err := w.sender.SendMetric(name, value, ts, source, tags)
		if err != nil {
			w.metricsSendFailure.Inc(1)
			log.Printf("[ERROR] error sending the metric '%s': %v", name, err)
		} else {
			w.numMetricsSent.Inc(1)
		}
	}
}
