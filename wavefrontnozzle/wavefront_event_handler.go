package wavefrontnozzle

import (
	"errors"
	"log"
	"os"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/config"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
)

type WavefrontEventHandler struct {
	sender     senders.Sender
	logger     *log.Logger
	prefix     string
	foundation string
}

func CreateWavefrontEventHandler(conf *config.WaveFrontConfig) *WavefrontEventHandler {
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

	return &WavefrontEventHandler{sender: sender, logger: logger, prefix: conf.Prefix, foundation: conf.Foundation}
}

func (w *WavefrontEventHandler) BuildHttpStartStopEvent(event *events.Envelope) {
	// genericSerializer(event)
}

func (w *WavefrontEventHandler) BuildLogMessageEvent(event *events.Envelope) {
	// genericSerializer(event)
}

func (w *WavefrontEventHandler) BuildValueMetricEvent(event *events.Envelope) {
	// >>> Events: origin:"DopplerServer" eventType:ValueMetric timestamp:1544661565385165422 deployment:"cf" job:"doppler" index:"3eba5e5c-069c-4f06-a3d6-ca7faa8df2db" ip:"10.202.6.14" valueMetric:<name:"grpcManager.subscriptions" value:2 unit:"subscriptions" >
	// MetricName: "<origin>.<name>.<unit>"
	metricName := w.prefix
	metricName += "." + event.GetOrigin()
	metricName += "." + event.GetValueMetric().GetName()
	metricName += "." + event.GetValueMetric().GetUnit()
	source, tags, ts := w.getMetricInfo(event)

	value := event.GetValueMetric().GetValue()

	w.sender.SendMetric(metricName, value, ts, source, tags)

	w.genericSerializer(event)
}

func (w *WavefrontEventHandler) BuildCounterEvent(event *events.Envelope) {
	metricName := w.prefix
	metricName += "." + event.GetOrigin()
	metricName += "." + event.GetCounterEvent().GetName()
	source, tags, ts := w.getMetricInfo(event)

	total := event.GetCounterEvent().GetTotal()
	delta := event.GetCounterEvent().GetDelta()

	w.sender.SendMetric(metricName+".total", float64(total), ts, source, tags)
	w.sender.SendMetric(metricName+".delta", float64(delta), ts, source, tags)
}

func (w *WavefrontEventHandler) BuildErrorEvent(event *events.Envelope) {
	// genericSerializer(event)
}

func (w *WavefrontEventHandler) BuildContainerEvent(event *events.Envelope, appInfo *api.AppInfo) {
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

	w.sender.SendMetric(metricName+".cpu_percentage", cpuPercentage, ts, source, tags)
	w.sender.SendMetric(metricName+".disk_bytes", float64(diskBytes), ts, source, tags)
	w.sender.SendMetric(metricName+".disk_bytes_quota", float64(diskBytesQuota), ts, source, tags)
	w.sender.SendMetric(metricName+".memory_bytes", float64(memoryBytes), ts, source, tags)
	w.sender.SendMetric(metricName+".memory_bytes_quota", float64(memoryBytesQuota), ts, source, tags)
}

func (w *WavefrontEventHandler) genericSerializer(event *events.Envelope) {
	w.logger.Printf("Event: %v", event)
}

func (w *WavefrontEventHandler) getMetricInfo(event *events.Envelope) (string, map[string]string, int64) {
	source := w.getSource(event)
	tags := w.getTags(event)

	return source, tags, event.GetTimestamp()
}

func (w *WavefrontEventHandler) getSource(event *events.Envelope) string {
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

func (w *WavefrontEventHandler) getTags(event *events.Envelope) map[string]string {
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
