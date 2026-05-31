package iso8583

import (
	"bytes"
	"fmt"
	"sort"
)

func Serialize(msg *Message) ([]byte, error) {
	return SerializeWithDefs(msg, defaultFields)
}

func SerializeWithDefs(msg *Message, defs map[int]fieldDef) ([]byte, error) {
	if len(msg.MTI) != 4 {
		return nil, fmt.Errorf("invalid MTI: %q", msg.MTI)
	}

	des := make([]int, 0, len(msg.Fields))
	for de := range msg.Fields {
		des = append(des, de)
	}
	sort.Ints(des)

	var bm bitmap
	hasSecondary := false
	for _, de := range des {
		if de > 64 {
			hasSecondary = true
		}
		bm.set(de)
	}
	if hasSecondary {
		bm.set(1)
	}

	var buf bytes.Buffer
	buf.WriteString(msg.MTI)
	buf.Write(bm.bytes(hasSecondary))

	for _, de := range des {
		b, err := writeField(de, msg.Fields[de], defs)
		if err != nil {
			return nil, err
		}
		buf.Write(b)
	}

	return buf.Bytes(), nil
}
