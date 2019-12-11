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
	eventsChannel chan *events.Envelope
	errorsChannel chan error
	APIClient     *APIClient

	eventSerializer    *EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
	appsInfo           map[string]*AppInfo
}

// NewNozzle create a new Nozzle
func NewNozzle(conf *Config, eventsChannel chan *events.Envelope, errorsChannel chan error) *Nozzle {
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
	for _, selectedEventType := range conf.Nozzle.SelectedEvents {
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
		// appGuIG := envelope.GetContainerMetric().GetApplicationId()
		// if s.APIClient != nil {
		// 	appInfo, err := s.APIClient.GetApp(appGuIG)
		// 	if err != nil && debug {
		// 		logger.Print("[ERROR]", err)
		// 	}
		s.eventSerializer.BuildContainerEvent(envelope, nil)
		// } else {
		// 	logger.Fatal("[ERROR] APIClient is null")
		// }
	}
}
