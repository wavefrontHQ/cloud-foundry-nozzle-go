package nozzle

import (
	"log"
	"os"

	metrics "github.com/rcrowley/go-metrics"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

var logger = log.New(os.Stdout, "[WAVEFRONT] ", 0)
var debug = os.Getenv("WAVEFRONT_DEBUG") == "true"

// Nozzle will read all CF events and sent it to the Forwarder
type Nozzle interface {
	SetAPIClient(*APIClient)
}

type forwardingNozzle struct {
	eventSerializer    EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
	eventsChannel      <-chan *events.Envelope
	errorsChannel      <-chan error
	appsInfo           map[string]*AppInfo
	fetcher            *APIClient
}

// NewNozzle create a new Nozzle
func NewNozzle(eventSerializer EventHandler, selectedEventTypes []events.Envelope_EventType, eventsChannel <-chan *events.Envelope, errors <-chan error) Nozzle {
	nozzle := &forwardingNozzle{
		eventSerializer: eventSerializer,
		eventsChannel:   eventsChannel,
		errorsChannel:   errors,
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

	reporting.RegisterMetric("nozzle.queue.size", metrics.NewFunctionalGauge(nozzle.queueSize), GetInternalTags())

	go nozzle.run()
	return nozzle
}

func (s *forwardingNozzle) SetAPIClient(api *APIClient) {
	s.fetcher = api
}

func (s *forwardingNozzle) queueSize() int64 {
	return int64(len(s.eventsChannel))
}

func (s *forwardingNozzle) run() {
	for {
		select {
		case event := <-s.eventsChannel:
			s.handleEvent(event)
		case err := <-s.errorsChannel:
			s.eventSerializer.ReportError(err)
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
		if s.fetcher != nil {
			appInfo, err := s.fetcher.GetApp(appGuIG)
			if err != nil && debug {
				logger.Print("[ERROR]", err)
			}
			s.eventSerializer.BuildContainerEvent(envelope, appInfo)
		} else {
			logger.Println("***********")
		}
	}
}
