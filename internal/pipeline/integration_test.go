package pipeline

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isox/internal/config"
	"github.com/isox/internal/framing"
	"github.com/isox/internal/iso8583"
)

func makeAuthorizer(t *testing.T, label string, counter *atomic.Int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		body, _ := io.ReadAll(r.Body)

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("%s: invalid JSON: %v", label, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		inner, _ := req["body"].(map[string]interface{})
		if inner["mti"] != "0100" {
			t.Errorf("%s: expected MTI 0100, got %v", label, inner["mti"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"body": map[string]interface{}{
				"response_code": "00",
				"auth_code":     label,
			},
		})
	}))
}

func makeConfig(stableURL, canaryURL string, stableWeight, canaryWeight int, mipAddr string) *config.Config {
	mapping := []config.FieldMapping{
		{DE: 0, Path: "body.mti"},
		{DE: 2, Path: "body.pan"},
		{DE: 11, Path: "body.stan"},
		{DE: 41, Path: "body.terminal_id"},
	}
	responseMapping := []config.FieldMapping{
		{DE: 39, Path: "body.response_code"},
		{DE: 38, Path: "body.auth_code"},
	}

	upstreams := map[string]config.Upstream{
		"stable": {URL: stableURL, TimeoutMs: 5 * time.Second, Mapping: mapping, ResponseMapping: responseMapping},
	}
	upstreamRefs := []config.UpstreamRef{
		{Name: "stable", Weight: stableWeight},
	}

	if canaryURL != "" {
		upstreams["canary"] = config.Upstream{URL: canaryURL, TimeoutMs: 5 * time.Second, Mapping: mapping, ResponseMapping: responseMapping}
		upstreamRefs = append(upstreamRefs, config.UpstreamRef{Name: "canary", Weight: canaryWeight})
	}

	return &config.Config{
		Global: config.Global{Workers: 4},
		Downstream: config.Downstream{
			Addr:              mipAddr,
			LengthHeader:      4,
			LengthEncoding:    "bcd",
			ReconnectInterval: 1 * time.Second,
			Heartbeat:         config.HeartbeatConfig{Interval: 1 * time.Hour, Timeout: 5 * time.Second, MTI: "0800"},
		},
		Upstreams: upstreams,
		Routes: []config.Route{
			{
				Conditions: []config.Condition{{Field: "mti", Operator: "==", Value: "0800"}},
				Action:     config.Action{Type: "echo", Fields: map[int]string{39: "00"}},
			},
			{
				Action: config.Action{Type: "forward", Upstreams: upstreamRefs},
			},
		},
	}
}

func sendAndReceive(t *testing.T, conn net.Conn, framer interface {
	Write(w interface{ Write([]byte) (int, error) }, data []byte) error
	Read(r interface{ Read([]byte) (int, error) }) ([]byte, error)
}, msg *iso8583.Message) *iso8583.Message {
	t.Helper()
	data, err := iso8583.Serialize(msg)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if err := framer.Write(conn, data); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.(*net.TCPConn).SetReadDeadline(time.Now().Add(5 * time.Second))
	respData, err := framer.Read(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp, err := iso8583.Parse(respData)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return resp
}

func TestIntegration(t *testing.T) {
	var counter atomic.Int64
	auth := makeAuthorizer(t, "stable", &counter)
	defer auth.Close()

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	cfg := makeConfig(auth.URL, "", 100, 0, ln.Addr().String())
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	p.Start()
	defer p.Stop()

	conn, _ := ln.Accept()
	defer conn.Close()

	f, _ := framing.New("bcd", 4)

	req := iso8583.NewMessage()
	req.MTI = "0100"
	req.Fields[2] = "5412345678901234"
	req.Fields[3] = "000000"
	req.Fields[4] = "000000010000"
	req.Fields[11] = "000042"
	req.Fields[41] = "TERM0001"
	req.Fields[42] = "MERCHANT000001 "
	req.Fields[49] = "986"

	data, _ := iso8583.Serialize(req)
	f.Write(conn, data)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	respData, err := f.Read(conn)
	if err != nil {
		t.Fatalf("expected 0110: %v", err)
	}
	resp, _ := iso8583.Parse(respData)

	if resp.MTI != "0110" {
		t.Errorf("expected MTI 0110, got %q", resp.MTI)
	}
	if resp.Fields[39] != "00" {
		t.Errorf("expected DE[39]=00, got %q", resp.Fields[39])
	}
}

func TestIntegrationHeartbeat(t *testing.T) {
	var authCalled atomic.Int64
	auth := makeAuthorizer(t, "stable", &authCalled)
	defer auth.Close()

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	cfg := makeConfig(auth.URL, "", 100, 0, ln.Addr().String())
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	p.Start()
	defer p.Stop()

	conn, _ := ln.Accept()
	defer conn.Close()

	f, _ := framing.New("bcd", 4)

	hb := iso8583.NewMessage()
	hb.MTI = "0800"
	hb.Fields[70] = "301"
	data, _ := iso8583.Serialize(hb)
	f.Write(conn, data)

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	respData, err := f.Read(conn)
	if err != nil {
		t.Fatalf("expected 0810: %v", err)
	}

	resp, _ := iso8583.Parse(respData)
	if resp.MTI != "0810" {
		t.Errorf("expected MTI 0810, got %q", resp.MTI)
	}
	if authCalled.Load() != 0 {
		t.Error("authorizer should not be called for 0800")
	}
	fmt.Printf("DE[39]=%s\n", resp.Fields[39])
}

func TestCanaryDistribution(t *testing.T) {
	var stableCount, canaryCount atomic.Int64

	stable := makeAuthorizer(t, "stable", &stableCount)
	defer stable.Close()
	canary := makeAuthorizer(t, "canary", &canaryCount)
	defer canary.Close()

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	// 80% stable, 20% canary
	cfg := makeConfig(stable.URL, canary.URL, 80, 20, ln.Addr().String())
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("new pipeline: %v", err)
	}
	p.Start()
	defer p.Stop()

	conn, _ := ln.Accept()
	defer conn.Close()

	f, _ := framing.New("bcd", 4)
	total := 200

	for i := 0; i < total; i++ {
		msg := iso8583.NewMessage()
		msg.MTI = "0100"
		msg.Fields[2] = "5412345678901234"
		msg.Fields[3] = "000000"
		msg.Fields[4] = "000000010000"
		msg.Fields[11] = fmt.Sprintf("%06d", i+1)
		msg.Fields[41] = "TERM0001"
		msg.Fields[42] = "MERCHANT000001 "
		msg.Fields[49] = "986"

		data, _ := iso8583.Serialize(msg)
		f.Write(conn, data)

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		f.Read(conn)
	}

	sc := stableCount.Load()
	cc := canaryCount.Load()
	total64 := int64(total)

	t.Logf("stable: %d/%d (%.1f%%), canary: %d/%d (%.1f%%)",
		sc, total, float64(sc)/float64(total)*100,
		cc, total, float64(cc)/float64(total)*100,
	)

	// tolerância de ±10% — com 200 requests a distribuição deve convergir
	if sc < total64*65/100 || sc > total64*95/100 {
		t.Errorf("stable hit rate out of range: %d/%d", sc, total)
	}
	if cc < total64*5/100 || cc > total64*35/100 {
		t.Errorf("canary hit rate out of range: %d/%d", cc, total)
	}
}
