package router

import (
	"testing"

	"github.com/isox/internal/config"
	"github.com/isox/internal/iso8583"
)

func TestEngineEchoOn0800(t *testing.T) {
	engine := NewEngine([]config.Route{
		{
			Conditions: []config.Condition{
				{Field: "mti", Operator: "==", Value: "0800"},
			},
			Action: config.Action{Type: "echo", Fields: map[int]string{39: "00"}},
		},
		{
			Action: config.Action{Type: "forward"},
		},
	})

	msg := iso8583.NewMessage()
	msg.MTI = "0800"

	result := engine.Evaluate(msg)

	if result.Action != ActionEcho {
		t.Errorf("esperado echo, obtido %q", result.Action)
	}
	if result.Fields[39] != "00" {
		t.Errorf("DE[39]: esperado 00, obtido %q", result.Fields[39])
	}
}

func TestEngineForwardOn0100(t *testing.T) {
	engine := NewEngine([]config.Route{
		{
			Conditions: []config.Condition{
				{Field: "mti", Operator: "==", Value: "0800"},
			},
			Action: config.Action{Type: "echo"},
		},
		{
			Action: config.Action{Type: "forward"},
		},
	})

	msg := iso8583.NewMessage()
	msg.MTI = "0100"

	result := engine.Evaluate(msg)

	if result.Action != ActionForward {
		t.Errorf("esperado forward, obtido %q", result.Action)
	}
}

func TestEngineFirstMatchWins(t *testing.T) {
	engine := NewEngine([]config.Route{
		{
			Conditions: []config.Condition{
				{Field: "mti", Operator: "==", Value: "0100"},
				{Field: "de[3]", Operator: "starts_with", Value: "00"},
			},
			Action: config.Action{Type: "echo", Fields: map[int]string{39: "first"}},
		},
		{
			Conditions: []config.Condition{
				{Field: "mti", Operator: "==", Value: "0100"},
			},
			Action: config.Action{Type: "echo", Fields: map[int]string{39: "second"}},
		},
	})

	msg := iso8583.NewMessage()
	msg.MTI = "0100"
	msg.Fields[3] = "000000"

	result := engine.Evaluate(msg)

	if result.Fields[39] != "first" {
		t.Errorf("esperado first match, obtido %q", result.Fields[39])
	}
}

func TestEngineConditions(t *testing.T) {
	cases := []struct {
		name      string
		field     string
		operator  string
		value     string
		msgMTI    string
		msgFields map[int]string
		wantMatch bool
	}{
		{"== mti match", "mti", "==", "0100", "0100", nil, true},
		{"== mti no match", "mti", "==", "0200", "0100", nil, false},
		{"starts_with match", "de[2]", "starts_with", "54", "0100", map[int]string{2: "5412345678"}, true},
		{"starts_with no match", "de[2]", "starts_with", "41", "0100", map[int]string{2: "5412345678"}, false},
		{"contains match", "de[48]", "contains", "TAG01", "0100", map[int]string{48: "DATATAG01END"}, true},
		{"!= match", "mti", "!=", "0800", "0100", nil, true},
		{"!= no match", "mti", "!=", "0100", "0100", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewEngine([]config.Route{
				{
					Conditions: []config.Condition{
						{Field: tc.field, Operator: tc.operator, Value: tc.value},
					},
					Action: config.Action{Type: "echo"},
				},
			})

			msg := iso8583.NewMessage()
			msg.MTI = tc.msgMTI
			for de, v := range tc.msgFields {
				msg.Fields[de] = v
			}

			result := engine.Evaluate(msg)
			matched := result.Action == ActionEcho

			if matched != tc.wantMatch {
				t.Errorf("esperado match=%v, obtido match=%v", tc.wantMatch, matched)
			}
		})
	}
}
