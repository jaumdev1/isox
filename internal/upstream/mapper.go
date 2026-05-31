package upstream

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isox/internal/config"
	"github.com/isox/internal/iso8583"
)

func toJSON(msg *iso8583.Message, mappings []config.FieldMapping) ([]byte, error) {
	body := make(map[string]interface{})
	for _, m := range mappings {
		var value string
		if m.DE == 0 {
			value = msg.MTI
		} else {
			value = msg.Fields[m.DE]
		}
		setPath(body, m.Path, value)
	}
	return json.Marshal(body)
}

func fromJSON(data []byte, original *iso8583.Message, mappings []config.FieldMapping) (*iso8583.Message, error) {
	var body map[string]interface{}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("authorizer response is not valid JSON: %w", err)
	}

	resp := iso8583.NewMessage()
	resp.MTI = responseMTI(original.MTI)
	for de, v := range original.Fields {
		resp.Fields[de] = v
	}

	for _, m := range mappings {
		val := getPath(body, m.Path)
		if val == "" {
			continue
		}
		if m.DE == 0 {
			resp.MTI = val
		} else {
			resp.Fields[m.DE] = val
		}
	}

	return resp, nil
}

// responseMTI converts a request MTI to its response counterpart (0100 → 0110).
func responseMTI(mti string) string {
	if len(mti) != 4 {
		return mti
	}
	return mti[:2] + "1" + mti[3:]
}

func setPath(m map[string]interface{}, path string, value string) {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) == 1 {
		m[path] = value
		return
	}
	sub, ok := m[parts[0]].(map[string]interface{})
	if !ok {
		sub = make(map[string]interface{})
		m[parts[0]] = sub
	}
	setPath(sub, parts[1], value)
}

func getPath(m map[string]interface{}, path string) string {
	parts := strings.SplitN(path, ".", 2)
	val, ok := m[parts[0]]
	if !ok {
		return ""
	}
	if len(parts) == 1 {
		return fmt.Sprintf("%v", val)
	}
	sub, ok := val.(map[string]interface{})
	if !ok {
		return ""
	}
	return getPath(sub, parts[1])
}
