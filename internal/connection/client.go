package connection

import (
	"log"
	"net"
	"time"

	"github.com/isox/internal/config"
	"github.com/isox/internal/framing"
)

type Client struct {
	cfg      config.Downstream
	framer   framing.Framer
	outbound chan []byte
	onRead   func([]byte)
	done     chan struct{}
}

func NewClient(cfg config.Downstream, onRead func([]byte)) (*Client, error) {
	f, err := framing.New(cfg.LengthEncoding, cfg.LengthHeader)
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg:      cfg,
		framer:   f,
		onRead:   onRead,
		outbound: make(chan []byte, 512),
		done:     make(chan struct{}),
	}, nil
}

func (c *Client) Start() {
	go c.run()
}

func (c *Client) Send(data []byte) {
	c.outbound <- data
}

func (c *Client) Stop() {
	close(c.done)
}

func (c *Client) run() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		conn, err := net.DialTimeout("tcp", c.cfg.Addr, 10*time.Second)
		if err != nil {
			log.Printf("connection to MIP failed (%s): %v — retrying in %s", c.cfg.Addr, err, c.cfg.ReconnectInterval)
			time.Sleep(c.cfg.ReconnectInterval)
			continue
		}

		log.Printf("connected to MIP at %s", c.cfg.Addr)

		readerDone := make(chan struct{})
		go c.reader(conn, readerDone)
		c.writer(conn, readerDone)

		log.Printf("MIP connection lost — reconnecting in %s", c.cfg.ReconnectInterval)
		conn.Close()
		time.Sleep(c.cfg.ReconnectInterval)
	}
}

func (c *Client) reader(conn net.Conn, done chan<- struct{}) {
	defer close(done)
	for {
		data, err := c.framer.Read(conn)
		if err != nil {
			log.Printf("error reading from MIP: %v", err)
			return
		}
		c.onRead(data)
	}
}

func (c *Client) writer(conn net.Conn, readerDone <-chan struct{}) {
	for {
		select {
		case <-c.done:
			return
		case <-readerDone:
			return
		case data := <-c.outbound:
			if err := c.framer.Write(conn, data); err != nil {
				log.Printf("error writing to MIP: %v", err)
				return
			}
		}
	}
}
