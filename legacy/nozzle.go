package legacy

import (
	"strings"

	"github.com/cloudfoundry/sonde-go/events"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

var defaultEvents = []events.Envelope_EventType{
	events.Envelope_ValueMetric,
	events.Envelope_CounterEvent,
	events.Envelope_ContainerMetric,
}

// Nozzle will read all CF events and sent it to the Forwarder
type Nozzle struct {
	EventsChannel chan *events.Envelope
	ErrorsChannel chan error
	APIClient     *APIClient

	eventSerializer    *EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
	appsInfo           map[string]*AppInfo
}

// NewNozzle create a new Nozzle
func NewNozzle(conf *common.Config) *Nozzle {
	nozzle := &Nozzle{
		eventSerializer: CreateEventHandler(conf.Wavefront),
		EventsChannel:   make(chan *events.Envelope, 1000),
		ErrorsChannel:   make(chan error),
	}

	nozzle.includedEventTypes = map[events.Envelope_EventType]bool{
		events.Envelope_HttpStartStop:   false,
		events.Envelope_LogMessage:      false,
		events.Envelope_ValueMetric:     false,
		events.Envelope_CounterEvent:    false,
		events.Envelope_Error:           false,
		events.Envelope_ContainerMetric: false,
	}

	for _, selectedEventType := range ParseSelectedEvents(conf.Nozzle.SelectedEvents) {
		nozzle.includedEventTypes[selectedEventType] = true
	}

	reporting.RegisterMetric("nozzle.queue.size", metrics.NewFunctionalGauge(nozzle.queueSize), common.GetInternalTags())

	go nozzle.run()
	return nozzle
}

func (s *Nozzle) queueSize() int64 {
	return int64(len(s.EventsChannel))
}

func (s *Nozzle) run() {
	for {
		select {
		case event := <-s.EventsChannel:
			s.handleEvent(event)
		case err := <-s.ErrorsChannel:
			s.eventSerializer.ReportError(err)
		}
	}
}

func (s *Nozzle) handleEvent(envelope *events.Envelope) {
	eventType := envelope.GetEventType()
	if !s.includedEventTypes[eventType] {
		return
	}

	switch eventType {
	case events.Envelope_ValueMetric:
		s.eventSerializer.BuildValueMetricEvent(envelope)
	case events.Envelope_CounterEvent:
		s.eventSerializer.BuildCounterEvent(envelope)
	case events.Envelope_ContainerMetric:
		appGuIG := envelope.GetContainerMetric().GetApplicationId()
		if s.APIClient != nil {
			appInfo, err := s.APIClient.GetApp(appGuIG)
			if err != nil && common.Debug {
				common.Logger.Print("[ERROR]", err)
			}
			s.eventSerializer.BuildContainerEvent(envelope, appInfo)
		} else {
			common.Logger.Fatal("[ERROR] APIClient is null")
		}
	}
}

func ParseSelectedEvents(orgEnvValue string) []events.Envelope_EventType {
	envValue := strings.Trim(orgEnvValue, "[]")
	if envValue == "" {
		return defaultEvents
	}

	selectedEvents := []events.Envelope_EventType{}
	sep := " "
	if strings.Contains(envValue, ",") {
		sep = ","
	}
	for _, envValueSplit := range strings.Split(envValue, sep) {
		envValueSlitTrimmed := strings.TrimSpace(envValueSplit)
		val, found := events.Envelope_EventType_value[envValueSlitTrimmed]
		if found {
			selectedEvents = append(selectedEvents, events.Envelope_EventType(val))
		} else {
			common.Logger.Panicf("[%s] is not a valid event type", orgEnvValue)
		}
	}

	return selectedEvents
}
