package nozzle

import (
	"os"
	"strings"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	metrics "github.com/rcrowley/go-metrics"

	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

var trace = os.Getenv("WAVEFRONT_TRACE") == "true"

// EventHandler receive CF events and send metrics to WF
type EventHandler struct {
	wf common.Wavefront

	prefix     string
	foundation string

	numGaugeMetricReceived  metrics.Counter
	numCounterEventReceived metrics.Counter
}

// CreateEventHandler create a new EventHandler
func CreateEventHandler(conf *common.WavefrontConfig) *EventHandler {
	wf := common.NewWavefront(conf)

	internalTags := common.GetInternalTags()
	common.Logger.Printf("internalTags: %v", internalTags)

	numGaugeMetricReceived := newCounter("value-metric-received", internalTags)
	numCounterEventReceived := newCounter("counter-event-received", internalTags)

	ev := &EventHandler{
		wf:                      wf,
		prefix:                  strings.Trim(conf.Prefix, " "),
		foundation:              strings.Trim(conf.Foundation, " "),
		numGaugeMetricReceived:  numGaugeMetricReceived,
		numCounterEventReceived: numCounterEventReceived,
	}
	return ev
}

func newCounter(name string, tags map[string]string) metrics.Counter {
	return reporting.GetOrRegisterMetric(name, metrics.NewCounter(), tags).(metrics.Counter)
}

// //BuildValueMetricEvent parse and report metrics
// func (w *EventHandler) BuildValueMetricEvent(event *loggregator_v2.Envelope) {
// 	w.numValueMetricReceived.Inc(1)

// 	metricName := w.prefix
// 	metricName += "." + event.GetOrigin()
// 	metricName += "." + event.GetValueMetric().GetName()
// 	metricName += "." + event.GetValueMetric().GetUnit()
// 	source, tags, ts := w.getMetricInfo(event)

// 	value := event.GetValueMetric().GetValue()

// 	w.sendMetric(metricName, value, ts, source, tags)
// }

//BuildCounterEvent parse and report metrics
func (w *EventHandler) BuildCounterEvent(event *loggregator_v2.Envelope) {
	w.numCounterEventReceived.Inc(1)

	metricName := w.prefix
	metricName += "." + event.GetTags()["origin"]
	metricName += "." + event.GetCounter().GetName()

	// common.Logger.Println("metricName:", metricName)

	source, tags, ts := w.getMetricInfo(event)

	total := event.GetCounter().GetTotal()
	delta := event.GetCounter().GetDelta()

	w.wf.SendMetric(metricName+".total", float64(total), ts, source, tags)
	w.wf.SendMetric(metricName+".delta", float64(delta), ts, source, tags)
}

//BuildGaugeEvent parse and report metrics
func (w *EventHandler) BuildGaugeEvent(event *loggregator_v2.Envelope) {
	w.numGaugeMetricReceived.Inc(1)

	for name, metric := range event.GetGauge().GetMetrics() {
		metricName := w.prefix
		if event.GetTags()["origin"] == "rep" {
			metricName += ".container"
		}
		metricName += "." + event.GetTags()["origin"]
		metricName += "." + name

		common.Logger.Println("metricName:", metricName)

		source, tags, ts := w.getMetricInfo(event)

		w.wf.SendMetric(metricName+"."+metric.GetUnit(), metric.Value, ts, source, tags)
	}
}

//BuildContainerEvent parse and report metrics
// func (w *EventHandler) BuildContainerEvent(event *loggregator_v2.Envelope, appInfo *AppInfo) {
// 	w.numContainerMetricReceived.Inc(1)

// 	metricName := w.prefix + ".container." + event.GetOrigin()
// 	source, tags, ts := w.getMetricInfo(event)

// 	tags["applicationId"] = event.GetContainerMetric().GetApplicationId()
// 	tags["instanceIndex"] = fmt.Sprintf("%d", event.GetContainerMetric().GetInstanceIndex())
// 	if appInfo != nil {
// 		tags["applicationName"] = appInfo.Name
// 		tags["space"] = appInfo.Space
// 		tags["org"] = appInfo.Org
// 	}

// 	cpuPercentage := event.GetContainerMetric().GetCpuPercentage()
// 	diskBytes := event.GetContainerMetric().GetDiskBytes()
// 	diskBytesQuota := event.GetContainerMetric().GetDiskBytesQuota()
// 	memoryBytes := event.GetContainerMetric().GetMemoryBytes()
// 	memoryBytesQuota := event.GetContainerMetric().GetMemoryBytesQuota()

// 	w.sendMetric(metricName+".cpu_percentage", cpuPercentage, ts, source, tags)
// 	w.sendMetric(metricName+".disk_bytes", float64(diskBytes), ts, source, tags)
// 	w.sendMetric(metricName+".disk_bytes_quota", float64(diskBytesQuota), ts, source, tags)
// 	w.sendMetric(metricName+".memory_bytes", float64(memoryBytes), ts, source, tags)
// 	w.sendMetric(metricName+".memory_bytes_quota", float64(memoryBytesQuota), ts, source, tags)
// }

func (w *EventHandler) getMetricInfo(event *loggregator_v2.Envelope) (string, map[string]string, int64) {
	source := w.getSource(event)
	tags := w.getTags(event)

	return source, tags, event.GetTimestamp()
}

func (w *EventHandler) getSource(event *loggregator_v2.Envelope) string {
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

func (w *EventHandler) getTags(event *loggregator_v2.Envelope) map[string]string {
	tags := make(map[string]string)

	if deployment := event.GetTags()["deployment"]; len(deployment) > 0 {
		tags["deployment"] = deployment
	}

	if job := event.GetTags()["job"]; len(job) > 0 {
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

//ReportError increments the error counter
func (w *EventHandler) ReportError(err error) {
	w.wf.ReportError(err)
}
