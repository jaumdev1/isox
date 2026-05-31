package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/isox/internal/config"
	"github.com/isox/internal/iso8583"
)

type Client struct {
	cfg    config.Upstream
	client *http.Client
}

func NewClient(cfg config.Upstream) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.TimeoutMs},
	}
}

func (c *Client) Send(ctx context.Context, msg *iso8583.Message) (*iso8583.Message, error) {
	body, err := toJSON(msg, c.cfg.Mapping)
	if err != nil {
		return nil, fmt.Errorf("mapping message to JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return buildTimeout(msg), nil
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return buildTimeout(msg), nil
	}

	isoResp, err := fromJSON(data, msg, c.cfg.ResponseMapping)
	if err != nil {
		return buildTimeout(msg), nil
	}

	return isoResp, nil
}

// buildTimeout returns a response with DE39=68 (response timeout).
func buildTimeout(original *iso8583.Message) *iso8583.Message {
	resp := iso8583.NewMessage()
	resp.MTI = responseMTI(original.MTI)
	for _, de := range []int{2, 3, 4, 11, 12, 13, 41, 42} {
		if v, ok := original.Fields[de]; ok {
			resp.Fields[de] = v
		}
	}
	resp.Fields[39] = "68"
	return resp
}
