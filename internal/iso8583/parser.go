package iso8583

import "fmt"

func Parse(data []byte) (*Message, error) {
	return ParseWithDefs(data, defaultFields)
}

func ParseWithDefs(data []byte, defs map[int]fieldDef) (*Message, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	msg := NewMessage()
	msg.MTI = string(data[:4])
	pos := 4

	if len(data) < pos+8 {
		return nil, fmt.Errorf("missing primary bitmap")
	}

	var bm bitmap
	copy(bm[:8], data[pos:pos+8])
	pos += 8

	if bm.hasSecondary() {
		if len(data) < pos+8 {
			return nil, fmt.Errorf("secondary bitmap indicated but missing")
		}
		copy(bm[8:], data[pos:pos+8])
		pos += 8
	}

	for de := 2; de <= 128; de++ {
		if !bm.isSet(de) {
			continue
		}
		value, consumed, err := readField(de, data[pos:], defs)
		if err != nil {
			return nil, fmt.Errorf("reading %w", err)
		}
		msg.Fields[de] = value
		pos += consumed
	}

	return msg, nil
}
