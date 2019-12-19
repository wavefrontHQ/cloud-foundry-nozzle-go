package legacy

import (
	"strings"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/api"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/config"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
)

var defaultEvents = []events.Envelope_EventType{
	events.Envelope_ValueMetric,
	events.Envelope_CounterEvent,
	events.Envelope_ContainerMetric,
}

// Nozzle will read all CF events and sent it to the Forwarder
type Nozzle struct {
	eventsChannel chan *events.Envelope
	errorsChannel chan error
	APIClient     *api.APIClient

	eventSerializer    *EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
	appsInfo           map[string]*api.AppInfo
}

// NewNozzle create a new Nozzle
func NewNozzle(conf *config.Config, eventsChannel chan *events.Envelope, errorsChannel chan error) *Nozzle {
	nozzle := &Nozzle{
		eventSerializer: CreateEventHandler(conf.Wavefront),
		eventsChannel:   eventsChannel,
		errorsChannel:   errorsChannel,
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

	go nozzle.run()
	return nozzle
}

func (s *Nozzle) run() {
	for {
		select {
		case event := <-s.eventsChannel:
			s.handleEvent(event)
		case err := <-s.errorsChannel:
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
			appInfo := s.APIClient.GetApp(appGuIG)
			s.eventSerializer.BuildContainerEvent(envelope, appInfo)
		} else {
			utils.Logger.Fatal("[ERROR] APIClient is null")
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
			utils.Logger.Panicf("[%s] is not a valid event type", orgEnvValue)
		}
	}

	return selectedEvents
}
