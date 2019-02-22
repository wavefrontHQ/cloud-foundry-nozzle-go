package nozzle

import (
	"errors"
	"log"

	"github.com/cloudfoundry/sonde-go/events"
)

// Nozzle will read all CF events and sent it to the Forwarder
type Nozzle interface {
	Run() error
}

type forwardingNozzle struct {
	eventSerializer    EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
	eventsChannel      <-chan *events.Envelope
	errorsChannel      <-chan error
	logger             *log.Logger
	appsInfo           map[string]*AppInfo
	fetcher            *APIClient
}

// NewNozzle create a new Nozzle
func NewNozzle(fetcher *APIClient, eventSerializer EventHandler, selectedEventTypes []events.Envelope_EventType, eventsChannel <-chan *events.Envelope, errors <-chan error, logger *log.Logger) Nozzle {
	nozzle := &forwardingNozzle{
		eventSerializer: eventSerializer,
		eventsChannel:   eventsChannel,
		errorsChannel:   errors,
		logger:          logger,
		fetcher:         fetcher,
	}

	nozzle.includedEventTypes = map[events.Envelope_EventType]bool{
		events.Envelope_HttpStartStop:   false,
		events.Envelope_LogMessage:      false,
		events.Envelope_ValueMetric:     false,
		events.Envelope_CounterEvent:    false,
		events.Envelope_Error:           false,
		events.Envelope_ContainerMetric: false,
	}
	for _, selectedEventType := range selectedEventTypes {
		nozzle.includedEventTypes[selectedEventType] = true
	}

	return nozzle
}

func (s *forwardingNozzle) Run() error {
	for {
		select {
		case event, ok := <-s.eventsChannel:
			if !ok {
				return errors.New("eventsChannel channel closed")
			}
			s.handleEvent(event)
		case err, ok := <-s.errorsChannel:
			if !ok {
				return errors.New("errorsChannel closed")
			}
			s.handleError(err)
		}
	}
}

func (s *forwardingNozzle) handleEvent(envelope *events.Envelope) {
	eventType := envelope.GetEventType()
	if !s.includedEventTypes[eventType] {
		return
	}

	switch eventType {
	case events.Envelope_HttpStartStop:
		s.eventSerializer.BuildHTTPStartStopEvent(envelope)
	case events.Envelope_LogMessage:
		s.eventSerializer.BuildLogMessageEvent(envelope)
	case events.Envelope_ValueMetric:
		s.eventSerializer.BuildValueMetricEvent(envelope)
	case events.Envelope_CounterEvent:
		s.eventSerializer.BuildCounterEvent(envelope)
	case events.Envelope_Error:
		s.eventSerializer.BuildErrorEvent(envelope)
	case events.Envelope_ContainerMetric:
		appGuIG := envelope.GetContainerMetric().GetApplicationId()
		appInfo := s.fetcher.GetApp(appGuIG)
		s.eventSerializer.BuildContainerEvent(envelope, appInfo)
	}
}

func (s *forwardingNozzle) handleError(err error) {
	s.logger.Printf("Error from firehose - %v", err)
}
