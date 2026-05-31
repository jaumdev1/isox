package pipeline

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/isox/internal/config"
	"github.com/isox/internal/connection"
	"github.com/isox/internal/iso8583"
	"github.com/isox/internal/router"
	"github.com/isox/internal/upstream"
)

type Pipeline struct {
	cfg      *config.Config
	inbound  chan *iso8583.Message
	outbound chan []byte
	engine   atomic.Pointer[router.Engine]
	pool     atomic.Pointer[upstream.Pool]
	conn     *connection.Client
	hb       *connection.Heartbeat
	ctx      context.Context
	cancel   context.CancelFunc
}

func New(cfg *config.Config) (*Pipeline, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pipeline{
		cfg:      cfg,
		inbound:  make(chan *iso8583.Message, 512),
		outbound: make(chan []byte, 512),
		ctx:      ctx,
		cancel:   cancel,
	}

	p.engine.Store(router.NewEngine(cfg.Routes))
	p.pool.Store(upstream.NewPool(cfg.Upstreams))

	conn, err := connection.NewClient(cfg.Downstream, p.onFrame)
	if err != nil {
		cancel()
		return nil, err
	}
	p.conn = conn

	p.hb = connection.NewHeartbeat(cfg.Downstream.Heartbeat, func(data []byte) {
		p.outbound <- data
	})

	return p, nil
}

func (p *Pipeline) Start() {
	p.conn.Start()
	p.hb.Start()
	for i := 0; i < p.cfg.Global.Workers; i++ {
		go p.worker()
	}
	go p.writer()
}

func (p *Pipeline) Stop() {
	p.cancel()
	p.hb.Stop()
	p.conn.Stop()
}

// Reload swaps the routing engine and upstream pool atomically.
// In-flight requests finish with the old config; new requests use the new one.
func (p *Pipeline) Reload(cfg *config.Config) {
	p.engine.Store(router.NewEngine(cfg.Routes))
	p.pool.Store(upstream.NewPool(cfg.Upstreams))
	log.Printf("config reloaded — routes: %d, upstreams: %d", len(cfg.Routes), len(cfg.Upstreams))
}

func (p *Pipeline) onFrame(raw []byte) {
	msg, err := iso8583.Parse(raw)
	if err != nil {
		log.Printf("error parsing ISO 8583 message: %v", err)
		return
	}
	p.inbound <- msg
}

func (p *Pipeline) worker() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case msg := <-p.inbound:
			p.process(msg)
		}
	}
}

func (p *Pipeline) process(msg *iso8583.Message) {
	// load atomically — both reads see the same generation
	engine := p.engine.Load()
	pool := p.pool.Load()

	result := engine.Evaluate(msg)

	var resp *iso8583.Message

	switch result.Action {
	case router.ActionEcho:
		resp = buildEcho(msg, result.Fields)

	case router.ActionForward:
		client, err := pool.Select(result.Upstreams)
		if err != nil {
			log.Printf("upstream selection error: %v", err)
			return
		}
		resp, err = client.Send(p.ctx, msg)
		if err != nil {
			log.Printf("error calling upstream: %v", err)
			return
		}
	}

	data, err := iso8583.Serialize(resp)
	if err != nil {
		log.Printf("error serializing response: %v", err)
		return
	}

	p.outbound <- data
}

func (p *Pipeline) writer() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case data := <-p.outbound:
			p.conn.Send(data)
		}
	}
}

func buildEcho(msg *iso8583.Message, extraFields map[int]string) *iso8583.Message {
	resp := iso8583.NewMessage()
	resp.MTI = echoMTI(msg.MTI)
	for de, v := range msg.Fields {
		resp.Fields[de] = v
	}
	for de, v := range extraFields {
		resp.Fields[de] = v
	}
	return resp
}

func echoMTI(mti string) string {
	if len(mti) != 4 {
		return mti
	}
	return mti[:2] + "1" + mti[3:]
}
