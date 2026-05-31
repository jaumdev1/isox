package router

import (
	"github.com/isox/internal/config"
	"github.com/isox/internal/iso8583"
)

type ActionType string

const (
	ActionForward ActionType = "forward"
	ActionEcho    ActionType = "echo"
)

type Result struct {
	Action    ActionType
	Fields    map[int]string
	Upstreams []config.UpstreamRef
}

type Engine struct {
	routes []config.Route
}

func NewEngine(routes []config.Route) *Engine {
	return &Engine{routes: routes}
}

func (e *Engine) Evaluate(msg *iso8583.Message) Result {
	for _, route := range e.routes {
		if matchAll(msg, route.Conditions) {
			return Result{
				Action:    ActionType(route.Action.Type),
				Fields:    route.Action.Fields,
				Upstreams: route.Action.Upstreams,
			}
		}
	}
	return Result{Action: ActionForward}
}
