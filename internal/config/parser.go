package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config: %w", err)
	}
	defer f.Close()

	p := &parser{
		scanner: bufio.NewScanner(f),
		cfg: &Config{
			Upstreams: make(map[string]Upstream),
		},
	}

	p.cfg.Global.Workers = 8
	p.cfg.Global.LogLevel = "info"
	p.cfg.Global.MetricsPort = 9090
	p.cfg.Downstream.LengthHeader = 4
	p.cfg.Downstream.LengthEncoding = "bcd"
	p.cfg.Downstream.ReconnectInterval = 5 * time.Second
	p.cfg.Downstream.Heartbeat.Interval = 30 * time.Second
	p.cfg.Downstream.Heartbeat.Timeout = 5 * time.Second
	p.cfg.Downstream.Heartbeat.MTI = "0800"

	if err := p.parse(); err != nil {
		return nil, err
	}

	return p.cfg, nil
}

type parser struct {
	scanner *bufio.Scanner
	cfg     *Config
	line    int
}

func (p *parser) parse() error {
	for p.scanner.Scan() {
		p.line++
		line := strings.TrimSpace(p.scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "global":
			if err := p.parseBlock(p.parseGlobal); err != nil {
				return err
			}
		case "downstream":
			if err := p.parseBlock(p.parseDownstream); err != nil {
				return err
			}
		case "upstream":
			// upstream <name> {
			name := ""
			if len(parts) >= 2 {
				name = parts[1]
			}
			u := Upstream{TimeoutMs: 20 * time.Second}
			if err := p.parseBlock(func(l string) error {
				return p.parseUpstreamBlock(l, &u)
			}); err != nil {
				return err
			}
			p.cfg.Upstreams[name] = u
		case "route":
			if err := p.parseBlock(p.parseRoutes); err != nil {
				return err
			}
		}
	}
	return p.scanner.Err()
}

func (p *parser) parseBlock(fn func(string) error) error {
	for p.scanner.Scan() {
		p.line++
		line := strings.TrimSpace(p.scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "}" {
			return nil
		}
		if err := fn(line); err != nil {
			return fmt.Errorf("line %d: %w", p.line, err)
		}
	}
	return nil
}

func (p *parser) parseGlobal(line string) error {
	key, val, err := directive(line)
	if err != nil {
		return err
	}
	switch key {
	case "workers":
		p.cfg.Global.Workers, err = strconv.Atoi(val)
	case "log_level":
		p.cfg.Global.LogLevel = val
	case "log_file":
		p.cfg.Global.LogFile = val
	case "metrics_port":
		p.cfg.Global.MetricsPort, err = strconv.Atoi(val)
	}
	return err
}

func (p *parser) parseDownstream(line string) error {
	key, val, err := directive(line)
	if err != nil {
		if strings.HasPrefix(line, "heartbeat") {
			return p.parseBlock(p.parseHeartbeat)
		}
		return err
	}
	switch key {
	case "addr":
		p.cfg.Downstream.Addr = val
	case "length_header":
		p.cfg.Downstream.LengthHeader, err = strconv.Atoi(val)
	case "length_encoding":
		p.cfg.Downstream.LengthEncoding = val
	case "reconnect_interval_ms":
		var ms int
		ms, err = strconv.Atoi(val)
		p.cfg.Downstream.ReconnectInterval = time.Duration(ms) * time.Millisecond
	}
	return err
}

func (p *parser) parseHeartbeat(line string) error {
	key, val, err := directive(line)
	if err != nil {
		return err
	}
	switch key {
	case "interval_ms":
		ms, e := strconv.Atoi(val)
		if e != nil {
			return e
		}
		p.cfg.Downstream.Heartbeat.Interval = time.Duration(ms) * time.Millisecond
	case "timeout_ms":
		ms, e := strconv.Atoi(val)
		if e != nil {
			return e
		}
		p.cfg.Downstream.Heartbeat.Timeout = time.Duration(ms) * time.Millisecond
	case "mti":
		p.cfg.Downstream.Heartbeat.MTI = val
	}
	return err
}

func (p *parser) parseUpstreamBlock(line string, u *Upstream) error {
	key, val, err := directive(line)
	if err != nil {
		if strings.HasPrefix(line, "request_mapping") {
			return p.parseBlock(func(l string) error {
				m, e := parseMapping(l)
				if e != nil {
					return e
				}
				u.Mapping = append(u.Mapping, m)
				return nil
			})
		}
		if strings.HasPrefix(line, "response_mapping") {
			return p.parseBlock(func(l string) error {
				m, e := parseMapping(l)
				if e != nil {
					return e
				}
				u.ResponseMapping = append(u.ResponseMapping, m)
				return nil
			})
		}
		return err
	}
	switch key {
	case "url":
		u.URL = val
	case "timeout_ms":
		var ms int
		ms, err = strconv.Atoi(val)
		u.TimeoutMs = time.Duration(ms) * time.Millisecond
	}
	return err
}

func (p *parser) parseRoutes(line string) error {
	if strings.HasPrefix(line, "match ") || line == "default" {
		route, err := p.parseRoute(line)
		if err != nil {
			return err
		}
		p.cfg.Routes = append(p.cfg.Routes, route)
	}
	return nil
}

func (p *parser) parseRoute(header string) (Route, error) {
	route := Route{}

	if header == "default" {
		return route, p.parseBlock(func(line string) error {
			return p.parseAction(line, &route)
		})
	}

	header = strings.TrimPrefix(header, "match ")
	header = strings.TrimSuffix(strings.TrimSuffix(header, " {"), "{")

	for _, part := range splitConditions(header) {
		c, err := parseCondition(strings.TrimSpace(part))
		if err != nil {
			return route, err
		}
		route.Conditions = append(route.Conditions, c)
	}

	return route, p.parseBlock(func(line string) error {
		return p.parseAction(line, &route)
	})
}

func (p *parser) parseAction(line string, route *Route) error {
	// upstream <name> weight <N>;
	if strings.HasPrefix(line, "upstream ") {
		return p.parseUpstreamRef(line, route)
	}

	key, val, err := directive(line)
	if err != nil {
		return err
	}
	switch key {
	case "action":
		route.Action.Type = val
	default:
		if strings.HasPrefix(key, "de[") {
			if route.Action.Fields == nil {
				route.Action.Fields = make(map[int]string)
			}
			de, e := parseDE(key)
			if e != nil {
				return e
			}
			route.Action.Fields[de] = val
		}
	}
	return nil
}

// parseUpstreamRef parses: upstream <name> weight <N>;
func (p *parser) parseUpstreamRef(line string, route *Route) error {
	line = strings.TrimSuffix(line, ";")
	parts := strings.Fields(line)

	// parts: ["upstream", "name"] or ["upstream", "name", "weight", "90"]
	if len(parts) < 2 {
		return fmt.Errorf("invalid upstream directive: %q", line)
	}

	ref := UpstreamRef{
		Name:   parts[1],
		Weight: 100, // default: single upstream gets all traffic
	}

	if len(parts) == 4 && parts[2] == "weight" {
		w, err := strconv.Atoi(parts[3])
		if err != nil {
			return fmt.Errorf("invalid weight %q: %w", parts[3], err)
		}
		ref.Weight = w
	}

	route.Action.Type = "forward"
	route.Action.Upstreams = append(route.Action.Upstreams, ref)
	return nil
}

func directive(line string) (key, val string, err error) {
	line = strings.TrimSuffix(line, ";")
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid directive: %q", line)
	}
	return strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), `"`), nil
}

func splitConditions(s string) []string {
	return strings.Split(s, " and ")
}

func parseCondition(s string) (Condition, error) {
	for _, op := range []string{"starts_with", "contains", "regex", "!=", "=="} {
		idx := strings.Index(s, op)
		if idx < 0 {
			continue
		}
		return Condition{
			Field:    strings.TrimSpace(s[:idx]),
			Operator: op,
			Value:    strings.Trim(strings.TrimSpace(s[idx+len(op):]), `"`),
		}, nil
	}
	return Condition{}, fmt.Errorf("invalid condition: %q", s)
}

func parseMapping(line string) (FieldMapping, error) {
	line = strings.TrimSuffix(line, ";")
	parts := strings.SplitN(line, "->", 2)
	if len(parts) != 2 {
		return FieldMapping{}, fmt.Errorf("invalid mapping: %q", line)
	}
	left := strings.TrimSpace(parts[0])
	path := strings.TrimSpace(parts[1])

	if left == "mti" {
		return FieldMapping{DE: 0, Path: path}, nil
	}

	de, err := parseDE(left)
	if err != nil {
		return FieldMapping{}, err
	}
	return FieldMapping{DE: de, Path: path}, nil
}

func parseDE(s string) (int, error) {
	s = strings.TrimPrefix(s, "de[")
	s = strings.TrimSuffix(s, "]")
	return strconv.Atoi(s)
}
