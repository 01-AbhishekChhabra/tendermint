package eventmeter

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/netmon/Godeps/_workspace/src/github.com/gorilla/websocket"
	"github.com/tendermint/netmon/Godeps/_workspace/src/github.com/rcrowley/go-metrics"
	. "github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/go-common"
	"github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/go-events"
	client "github.com/tendermint/netmon/Godeps/_workspace/src/github.com/tendermint/go-rpc/client"
)

//------------------------------------------------------
// Generic system to subscribe to events and record their frequency
//------------------------------------------------------

//------------------------------------------------------
// Function types

// Closure to enable side effects from receiving an event
type EventCallbackFunc func(em *EventMetric, data events.EventData)

// Get the eventID and data out of the raw json received over the go-rpc websocket
type EventUnmarshalFunc func(b json.RawMessage) (string, events.EventData, error)

// Closure to enable side effects from receiving a pong
type LatencyCallbackFunc func(latency float64)

//------------------------------------------------------
// Meter for a particular event

// Metrics for a given event
type EventMetric struct {
	ID          string    `json:"id"`
	Started     time.Time `json:"start_time"`
	LastHeard   time.Time `json:"last_heard"`
	MinDuration int64     `json:"min_duration"`
	MaxDuration int64     `json:"max_duration"`

	// tracks event count and rate
	meter metrics.Meter

	// filled in from the Meter
	Count    int64   `json:"count"`
	Rate1    float64 `json:"rate_1" wire:"unsafe"`
	Rate5    float64 `json:"rate_5" wire:"unsafe"`
	Rate15   float64 `json:"rate_15" wire:"unsafe"`
	RateMean float64 `json:"rate_mean" wire:"unsafe"`

	// so the event can have effects in the event-meter's consumer.
	// runs in a go routine
	callback EventCallbackFunc
}

func (metric *EventMetric) Copy() *EventMetric {
	metric2 := *metric
	metric2.meter = metric.meter.Snapshot()
	return &metric2
}

// called on GetMetric
func (metric *EventMetric) fillMetric() *EventMetric {
	metric.Count = metric.meter.Count()
	metric.Rate1 = metric.meter.Rate1()
	metric.Rate5 = metric.meter.Rate5()
	metric.Rate15 = metric.meter.Rate15()
	metric.RateMean = metric.meter.RateMean()
	return metric
}

//------------------------------------------------------
// Websocket client and event meter for many events

// Each node gets an event meter to track events for that node
type EventMeter struct {
	QuitService // inherits from the wsc

	wsc *client.WSClient

	mtx    sync.Mutex
	events map[string]*EventMetric

	// to record ws latency
	timer        metrics.Timer
	lastPing     time.Time
	receivedPong bool
	callback     LatencyCallbackFunc

	unmarshalEvent EventUnmarshalFunc
}

func NewEventMeter(addr string, unmarshalEvent EventUnmarshalFunc) *EventMeter {
	em := &EventMeter{
		wsc:            client.NewWSClient(addr),
		events:         make(map[string]*EventMetric),
		timer:          metrics.NewTimer(),
		receivedPong:   true,
		unmarshalEvent: unmarshalEvent,
	}
	em.QuitService = em.wsc.QuitService
	return em
}

func (em *EventMeter) Start() error {
	if err := em.wsc.OnStart(); err != nil {
		return err
	}

	em.wsc.Conn.SetPongHandler(func(m string) error {
		// NOTE: https://github.com/gorilla/websocket/issues/97
		em.mtx.Lock()
		defer em.mtx.Unlock()
		em.receivedPong = true
		em.timer.UpdateSince(em.lastPing)
		if em.callback != nil {
			go em.callback(em.timer.Mean())
		}
		return nil
	})
	go em.receiveRoutine()
	return nil
}

func (em *EventMeter) Stop() {
	em.wsc.OnStop()
}

func (em *EventMeter) Subscribe(eventID string, cb EventCallbackFunc) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	if _, ok := em.events[eventID]; ok {
		return fmt.Errorf("subscribtion already exists")
	}
	if err := em.wsc.Subscribe(eventID); err != nil {
		return err
	}

	metric := &EventMetric{
		ID:          eventID,
		Started:     time.Now(),
		MinDuration: 1 << 62,
		meter:       metrics.NewMeter(),
		callback:    cb,
	}
	em.events[eventID] = metric
	return nil
}

func (em *EventMeter) Unsubscribe(eventID string) error {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	if err := em.wsc.Unsubscribe(eventID); err != nil {
		return err
	}
	// XXX: should we persist or save this info first?
	delete(em.events, eventID)
	return nil
}

// Fill in the latest data for an event and return a copy
func (em *EventMeter) GetMetric(eventID string) (*EventMetric, error) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	metric, ok := em.events[eventID]
	if !ok {
		return nil, fmt.Errorf("Unknown event %s", eventID)
	}
	return metric.fillMetric().Copy(), nil
}

// Return the average latency over the websocket
func (em *EventMeter) Latency() float64 {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	return em.timer.Mean()
}

func (em *EventMeter) RegisterLatencyCallback(f LatencyCallbackFunc) {
	em.mtx.Lock()
	defer em.mtx.Unlock()
	em.callback = f
}

//------------------------------------------------------

func (em *EventMeter) receiveRoutine() {
	pingTicker := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-pingTicker.C:
			if err := em.pingForLatency(); err != nil {
				log.Error("Failed to write ping message on websocket", err)
				em.Stop()
				return
			}
		case r := <-em.wsc.ResultsCh:
			eventID, data, err := em.unmarshalEvent(r)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			em.updateMetric(eventID, data)
		case <-em.Quit:
			break
		}

	}
}

func (em *EventMeter) pingForLatency() error {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	// ping to record latency
	if !em.receivedPong {
		// XXX: why is the pong taking so long? should we stop the conn?
		return nil
	}

	em.lastPing = time.Now()
	em.receivedPong = false
	err := em.wsc.Conn.WriteMessage(websocket.PingMessage, []byte{})
	if err != nil {
		return err
	}
	return nil
}

func (em *EventMeter) updateMetric(eventID string, data events.EventData) {
	em.mtx.Lock()
	defer em.mtx.Unlock()

	metric, ok := em.events[eventID]
	if !ok {
		// we already unsubscribed, or got an unexpected event
		return
	}

	last := metric.LastHeard
	metric.LastHeard = time.Now()
	metric.meter.Mark(1)
	dur := int64(metric.LastHeard.Sub(last))
	if dur < metric.MinDuration {
		metric.MinDuration = dur
	}
	if !last.IsZero() && dur > metric.MaxDuration {
		metric.MaxDuration = dur
	}

	if metric.callback != nil {
		go metric.callback(metric.Copy(), data)
	}
}
