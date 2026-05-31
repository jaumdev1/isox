package connection

import (
	"log"
	"time"

	"github.com/isox/internal/config"
	"github.com/isox/internal/iso8583"
)

type Heartbeat struct {
	cfg  config.HeartbeatConfig
	send func([]byte)
	done chan struct{}
}

func NewHeartbeat(cfg config.HeartbeatConfig, send func([]byte)) *Heartbeat {
	return &Heartbeat{
		cfg:  cfg,
		send: send,
		done: make(chan struct{}),
	}
}

func (h *Heartbeat) Start() {
	go h.run()
}

func (h *Heartbeat) Stop() {
	close(h.done)
}

func (h *Heartbeat) run() {
	ticker := time.NewTicker(h.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.done:
			return
		case <-ticker.C:
			data, err := h.build()
			if err != nil {
				log.Printf("heartbeat: error building %s: %v", h.cfg.MTI, err)
				continue
			}
			h.send(data)
		}
	}
}

func (h *Heartbeat) build() ([]byte, error) {
	msg := iso8583.NewMessage()
	msg.MTI = h.cfg.MTI
	for de, v := range h.cfg.Fields {
		msg.Fields[de] = v
	}
	return iso8583.Serialize(msg)
}
