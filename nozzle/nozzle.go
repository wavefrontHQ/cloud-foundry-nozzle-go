package nozzle

import (
	"errors"
	"log"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/api"
)

type Nozzle interface {
	Run() error
}

type ForwardingNozzle struct {
	eventSerializer    EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
	eventsChannel      <-chan *events.Envelope
	errorsChannel      <-chan error
	logger             *log.Logger
	appsInfo           map[string]*api.AppInfo
	fetcher            *api.ApiClient
}

type EventHandler interface {
	BuildHttpStartStopEvent(event *events.Envelope)
	BuildLogMessageEvent(event *events.Envelope)
	BuildValueMetricEvent(event *events.Envelope)
	BuildCounterEvent(event *events.Envelope)
	BuildErrorEvent(event *events.Envelope)
	BuildContainerEvent(event *events.Envelope, appInfo *api.AppInfo)
}

func NewForwarder(fetcher *api.ApiClient, eventSerializer EventHandler, selectedEventTypes []events.Envelope_EventType, eventsChannel <-chan *events.Envelope, errors <-chan error, logger *log.Logger) Nozzle {
	nozzle := &ForwardingNozzle{
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

func (s *ForwardingNozzle) Run() error {
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

func (s *ForwardingNozzle) handleEvent(envelope *events.Envelope) {
	eventType := envelope.GetEventType()
	if !s.includedEventTypes[eventType] {
		return
	}

	switch eventType {
	case events.Envelope_HttpStartStop:
		s.eventSerializer.BuildHttpStartStopEvent(envelope)
	case events.Envelope_LogMessage:
		s.eventSerializer.BuildLogMessageEvent(envelope)
	case events.Envelope_ValueMetric:
		s.eventSerializer.BuildValueMetricEvent(envelope)
	case events.Envelope_CounterEvent:
		s.eventSerializer.BuildCounterEvent(envelope)
	case events.Envelope_Error:
		s.eventSerializer.BuildErrorEvent(envelope)
	case events.Envelope_ContainerMetric:
		appGuId := envelope.GetContainerMetric().GetApplicationId()
		appInfo, ok := s.appsInfo[appGuId]
		if !ok {
			s.appsInfo = s.fetcher.ListApps()
			appInfo = s.appsInfo[appGuId]
		}
		s.eventSerializer.BuildContainerEvent(envelope, appInfo)
	}
}

func (s *ForwardingNozzle) handleError(err error) {
	s.logger.Printf("Error from firehose - %v", err)
}
