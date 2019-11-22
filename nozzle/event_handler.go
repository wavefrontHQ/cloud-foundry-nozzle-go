package nozzle

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	metrics "github.com/rcrowley/go-metrics"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
)

var trace = os.Getenv("WAVEFRONT_TRACE") == "true"

// EventHandler receive CF events and send metrics to WF
type EventHandler struct {
	sender     senders.Sender
	reporter   reporting.WavefrontMetricsReporter
	prefix     string
	foundation string
	filter     Filter

	numMetricsSent             metrics.Counter
	metricsSendFailure         metrics.Counter
	metricsFiltered            metrics.Counter
	numValueMetricReceived     metrics.Counter
	numCounterEventReceived    metrics.Counter
	numContainerMetricReceived metrics.Counter
	handleErrorMetric          metrics.Counter
}

// CreateEventHandler create a new EventHandler
func CreateEventHandler(conf *WavefrontConfig) *EventHandler {
	var sender senders.Sender
	var err error

	if len(conf.ProxyAddr) == 0 {
		conf.ProxyAddr = os.Getenv("PROXY_CONN_HOST")
	}

	if len(conf.URL) > 0 && len(conf.Token) > 0 {
		logger.Printf("Direct connetion to Wavefront: %s", conf.URL)
		directCfg := &senders.DirectConfiguration{
			Server:               strings.Trim(conf.URL, " "),
			Token:                strings.Trim(conf.Token, " "),
			BatchSize:            conf.BatchSize,
			MaxBufferSize:        conf.MaxBufferSize,
			FlushIntervalSeconds: conf.FlushInterval,
		}
		sender, err = senders.NewDirectSender(directCfg)
		if err != nil {
			logger.Fatal(err)
		}
	} else if len(conf.ProxyAddr) > 0 && conf.ProxyPort > 0 {
		logger.Printf("Connecting to Wavefront proxy: '%s:%d'", conf.ProxyAddr, conf.ProxyPort)
		proxyCfg := &senders.ProxyConfiguration{
			Host:                 strings.Trim(conf.ProxyAddr, " "),
			MetricsPort:          conf.ProxyPort,
			FlushIntervalSeconds: conf.FlushInterval,
		}
		sender, err = senders.NewProxySender(proxyCfg)
		if err != nil {
			logger.Fatal(err)
		}
	} else {
		logger.Printf("Direct configuration: %s", conf.URL)
		logger.Printf("Proxy configuration: '%s:%d'", conf.ProxyAddr, conf.ProxyPort)
		logger.Fatal(errors.New("No Wavefront configuration detected"))
	}

	reporter := reporting.NewReporter(
		sender,
		application.New("pcf-nozzle", "internal-metrics"),
		reporting.Prefix("wavefront-firehose-nozzle.app"),
	)

	internalTags := GetInternalTags()
	logger.Printf("internalTags: %v", internalTags)

	numMetricsSent := newCounter("total-metrics-sent", internalTags)
	metricsSendFailure := newCounter("metrics-send-failure", internalTags)
	metricsFiltered := newCounter("metrics-filtered", internalTags)
	numValueMetricReceived := newCounter("value-metric-received", internalTags)
	numCounterEventReceived := newCounter("counter-event-received", internalTags)
	numContainerMetricReceived := newCounter("container-metric-received", internalTags)
	handleErrorMetric := newCounter("firehose-connection-error", internalTags)

	ev := &EventHandler{
		sender:                     sender,
		reporter:                   reporter,
		prefix:                     strings.Trim(conf.Prefix, " "),
		foundation:                 strings.Trim(conf.Foundation, " "),
		filter:                     NewGlobFilter(conf.Filters),
		numMetricsSent:             numMetricsSent,
		metricsSendFailure:         metricsSendFailure,
		metricsFiltered:            metricsFiltered,
		numValueMetricReceived:     numValueMetricReceived,
		numCounterEventReceived:    numCounterEventReceived,
		numContainerMetricReceived: numContainerMetricReceived,
		handleErrorMetric:          handleErrorMetric,
	}
	ev.startHealthReport()
	return ev
}

func newCounter(name string, tags map[string]string) metrics.Counter {
	return reporting.GetOrRegisterMetric(name, metrics.NewCounter(), tags).(metrics.Counter)
}

//BuildValueMetricEvent parse and report metrics
func (w *EventHandler) BuildValueMetricEvent(event *events.Envelope) {
	w.numValueMetricReceived.Inc(1)

	metricName := w.prefix
	metricName += "." + event.GetOrigin()
	metricName += "." + event.GetValueMetric().GetName()
	metricName += "." + event.GetValueMetric().GetUnit()
	source, tags, ts := w.getMetricInfo(event)

	value := event.GetValueMetric().GetValue()

	w.sendMetric(metricName, value, ts, source, tags)
}

//BuildCounterEvent parse and report metrics
func (w *EventHandler) BuildCounterEvent(event *events.Envelope) {
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

//BuildContainerEvent parse and report metrics
func (w *EventHandler) BuildContainerEvent(event *events.Envelope, appInfo *AppInfo) {
	w.numContainerMetricReceived.Inc(1)

	metricName := w.prefix + ".container." + event.GetOrigin()
	source, tags, ts := w.getMetricInfo(event)

	tags["applicationId"] = event.GetContainerMetric().GetApplicationId()
	tags["instanceIndex"] = fmt.Sprintf("%d", event.GetContainerMetric().GetInstanceIndex())
	if appInfo != nil {
		tags["applicationName"] = appInfo.Name
		tags["space"] = appInfo.Space
		tags["org"] = appInfo.Org
	}

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

func (w *EventHandler) getMetricInfo(event *events.Envelope) (string, map[string]string, int64) {
	source := w.getSource(event)
	tags := w.getTags(event)

	return source, tags, event.GetTimestamp()
}

func (w *EventHandler) getSource(event *events.Envelope) string {
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

func (w *EventHandler) getTags(event *events.Envelope) map[string]string {
	tags := make(map[string]string)

	if deployment := event.GetDeployment(); len(deployment) > 0 {
		tags["deployment"] = deployment
	}

	if job := event.GetJob(); len(job) > 0 {
		tags["job"] = job
	}

	tags["foundation"] = w.foundation

	for k, v := range event.GetTags() {
		if len(k) > 0 && len(v) > 0 {
			tags[k] = v
		}
	}
	return tags
}

func (w *EventHandler) sendMetric(name string, value float64, ts int64, source string, tags map[string]string) {
	if trace {
		line, err := senders.MetricLine(name, value, ts, source, tags, "")
		if err != nil {
			logger.Printf("[ERROR] error preparing the metric '%s': %v", name, err)
		}

		status := "filtered"
		if w.filter.Match(name, tags) {
			status = "accepted"
		}
		logger.Printf("[DEBUG] [%s] metric: %s", status, line)
	}

	if w.filter.Match(name, tags) {
		err := w.sender.SendMetric(name, value, ts, source, tags)
		if err != nil {
			w.metricsSendFailure.Inc(1)
			if debug {
				logger.Printf("[ERROR] error sending the metric '%s': %v", name, err)
			}
		} else {
			w.numMetricsSent.Inc(1)
		}
	} else {
		w.metricsFiltered.Inc(1)
	}
}

//ReportError increments the error counter
func (w *EventHandler) ReportError(err error) {
	w.handleErrorMetric.Inc(1)
}

func (w *EventHandler) startHealthReport() {
	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			logger.Printf("total metrics sent: %d  filtered: %d  failures: %d", w.numMetricsSent.Count(), w.metricsFiltered.Count(), w.metricsSendFailure.Count())
		}
	}()
}
