package legacy

import (
	"fmt"
	"os"
	"strings"

	metrics "github.com/rcrowley/go-metrics"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/wavefront"
)

// EventHandler receive CF events and send metrics to WF
type EventHandler struct {
	wf wavefront.Wavefront

	prefix     string
	foundation string

	numValueMetricReceived     metrics.Counter
	numCounterEventReceived    metrics.Counter
	numContainerMetricReceived metrics.Counter
}

// CreateEventHandler create a new EventHandler
func CreateEventHandler(conf *config.WavefrontConfig) *EventHandler {
	wf := wavefront.NewWavefront(conf)

	internalTags := utils.GetInternalTags()
	utils.Logger.Printf("internalTags: %v", internalTags)

	numValueMetricReceived := wavefront.NewCounter("value-metric-received", internalTags)
	numCounterEventReceived := wavefront.NewCounter("counter-event-received", internalTags)
	numContainerMetricReceived := wavefront.NewCounter("container-metric-received", internalTags)

	ev := &EventHandler{
		wf:                         wf,
		prefix:                     strings.Trim(conf.Prefix, " "),
		foundation:                 strings.Trim(conf.Foundation, " "),
		numValueMetricReceived:     numValueMetricReceived,
		numCounterEventReceived:    numCounterEventReceived,
		numContainerMetricReceived: numContainerMetricReceived,
	}
	return ev
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

	w.wf.SendMetric(metricName, value, ts, source, tags)
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

	w.wf.SendMetric(metricName+".total", float64(total), ts, source, tags)
	w.wf.SendMetric(metricName+".delta", float64(delta), ts, source, tags)
}

//BuildContainerEvent parse and report metrics
func (w *EventHandler) BuildContainerEvent(event *events.Envelope, appInfo *api.AppInfo) {
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

	w.wf.SendMetric(metricName+".cpu_percentage", cpuPercentage, ts, source, tags)
	w.wf.SendMetric(metricName+".disk_bytes", float64(diskBytes), ts, source, tags)
	w.wf.SendMetric(metricName+".disk_bytes_quota", float64(diskBytesQuota), ts, source, tags)
	w.wf.SendMetric(metricName+".memory_bytes", float64(memoryBytes), ts, source, tags)
	w.wf.SendMetric(metricName+".memory_bytes_quota", float64(memoryBytesQuota), ts, source, tags)
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

//ReportError increments the error counter
func (w *EventHandler) ReportError(err error) {
	w.wf.ReportError(err)
}
