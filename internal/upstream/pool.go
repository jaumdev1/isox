package upstream

import (
	"fmt"
	"math/rand"

	"github.com/isox/internal/config"
)

type Pool struct {
	clients map[string]*Client
}

func NewPool(upstreams map[string]config.Upstream) *Pool {
	clients := make(map[string]*Client, len(upstreams))
	for name, cfg := range upstreams {
		clients[name] = NewClient(cfg)
	}
	return &Pool{clients: clients}
}

// Select picks an upstream client using weighted random selection.
// With a single upstream the weight is ignored and it is always selected.
func (p *Pool) Select(refs []config.UpstreamRef) (*Client, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("no upstreams defined for route")
	}

	if len(refs) == 1 {
		return p.get(refs[0].Name)
	}

	total := 0
	for _, r := range refs {
		total += r.Weight
	}

	n := rand.Intn(total)
	cumulative := 0
	for _, r := range refs {
		cumulative += r.Weight
		if n < cumulative {
			return p.get(r.Name)
		}
	}

	return p.get(refs[len(refs)-1].Name)
}

func (p *Pool) get(name string) (*Client, error) {
	c, ok := p.clients[name]
	if !ok {
		return nil, fmt.Errorf("upstream %q not found", name)
	}
	return c, nil
}
