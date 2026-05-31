package config

import "time"

type Config struct {
	Global     Global
	Downstream Downstream
	Upstreams  map[string]Upstream
	Routes     []Route
}

type Global struct {
	Workers     int
	LogLevel    string
	LogFile     string
	MetricsPort int
}

type Downstream struct {
	Addr              string
	LengthHeader      int
	LengthEncoding    string
	ReconnectInterval time.Duration
	Heartbeat         HeartbeatConfig
}

type HeartbeatConfig struct {
	Interval time.Duration
	Timeout  time.Duration
	MTI      string
	Fields   map[int]string
}

type Upstream struct {
	URL             string
	TimeoutMs       time.Duration
	Mapping         []FieldMapping
	ResponseMapping []FieldMapping
}

type FieldMapping struct {
	DE   int
	Path string
}

type Route struct {
	Conditions []Condition
	Action     Action
}

type Condition struct {
	Field    string
	Operator string
	Value    string
}

type Action struct {
	Type      string
	Fields    map[int]string
	Upstreams []UpstreamRef
}

type UpstreamRef struct {
	Name   string
	Weight int
}
