package nozzle

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/cloudfoundry/sonde-go/events"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/common"
	"github.com/wavefronthq/go-metrics-wavefront/reporting"
)

// Nozzle will read all CF events and sent it to the Forwarder
type Nozzle struct {
	EventsChannel chan *loggregator_v2.Envelope
	ErrorsChannel chan error

	eventSerializer    *EventHandler
	includedEventTypes map[events.Envelope_EventType]bool
}

// NewNozzle create a new Nozzle
func NewNozzle(conf *common.Config) *Nozzle {
	nozzle := &Nozzle{
		eventSerializer: CreateEventHandler(conf.Wavefront),
		EventsChannel:   make(chan *loggregator_v2.Envelope, 1000),
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

func (s *Nozzle) handleEvent(envelope *loggregator_v2.Envelope) {
	switch envelope.GetMessage().(type) {
	case *loggregator_v2.Envelope_Counter:
		s.eventSerializer.BuildCounterEvent(envelope)
	case *loggregator_v2.Envelope_Gauge:
		s.eventSerializer.BuildGaugeEvent(envelope)
	default:
		// common.Logger.Printf("---> %v\n", envelope)
		// common.Logger.Printf("---> %v\n", envelope.GetMessage())
		// common.Logger.Panicf("---> %v\n", reflect.TypeOf(envelope.GetMessage()))
	}
}
