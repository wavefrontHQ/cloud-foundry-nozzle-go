package nozzle

import (
	"log"
	"os"

	"github.com/cloudfoundry/sonde-go/events"
)

var logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var debug = os.Getenv("WAVEFRONT_DEBUG") == "true"

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
func NewNozzle(conf *Config) *Nozzle {
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
	for _, selectedEventType := range conf.Nozzle.SelectedEvents {
		nozzle.includedEventTypes[selectedEventType] = true
	}

	go nozzle.run()
	return nozzle
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
			if err != nil && debug {
				logger.Print("[ERROR]", err)
			}
			s.eventSerializer.BuildContainerEvent(envelope, appInfo)
		} else {
			logger.Fatal("[ERROR] APIClient is null")
		}
	}
}
