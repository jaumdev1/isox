package iso8583

import (
	"fmt"
	"strconv"
)

type fieldType int

const (
	fixed  fieldType = iota
	llvar
	lllvar
)

type fieldDef struct {
	kind   fieldType
	length int
}

var defaultFields = map[int]fieldDef{
	1:   {fixed, 8},
	2:   {llvar, 19},
	3:   {fixed, 6},
	4:   {fixed, 12},
	7:   {fixed, 10},
	11:  {fixed, 6},
	12:  {fixed, 6},
	13:  {fixed, 4},
	14:  {fixed, 4},
	18:  {fixed, 4},
	22:  {fixed, 3},
	25:  {fixed, 2},
	32:  {llvar, 11},
	35:  {llvar, 37},
	37:  {fixed, 12},
	38:  {fixed, 6},
	39:  {fixed, 2},
	41:  {fixed, 8},
	42:  {fixed, 15},
	43:  {fixed, 40},
	48:  {lllvar, 999},
	49:  {fixed, 3},
	52:  {fixed, 8},
	55:  {lllvar, 999},
	60:  {lllvar, 999},
	61:  {lllvar, 999},
	70:  {fixed, 3},
	90:  {fixed, 42},
	102: {llvar, 28},
	103: {llvar, 28},
}

func readField(de int, data []byte, defs map[int]fieldDef) (string, int, error) {
	def, ok := defs[de]
	if !ok {
		return "", 0, fmt.Errorf("field DE[%d] not defined", de)
	}

	switch def.kind {
	case fixed:
		if len(data) < def.length {
			return "", 0, fmt.Errorf("DE[%d]: not enough data, expected %d bytes, got %d", de, def.length, len(data))
		}
		return string(data[:def.length]), def.length, nil

	case llvar:
		if len(data) < 2 {
			return "", 0, fmt.Errorf("DE[%d]: missing LLVAR prefix", de)
		}
		length, err := strconv.Atoi(string(data[:2]))
		if err != nil {
			return "", 0, fmt.Errorf("DE[%d]: invalid LLVAR prefix: %w", de, err)
		}
		if len(data) < 2+length {
			return "", 0, fmt.Errorf("DE[%d]: not enough data for LLVAR", de)
		}
		return string(data[2 : 2+length]), 2 + length, nil

	case lllvar:
		if len(data) < 3 {
			return "", 0, fmt.Errorf("DE[%d]: missing LLLVAR prefix", de)
		}
		length, err := strconv.Atoi(string(data[:3]))
		if err != nil {
			return "", 0, fmt.Errorf("DE[%d]: invalid LLLVAR prefix: %w", de, err)
		}
		if len(data) < 3+length {
			return "", 0, fmt.Errorf("DE[%d]: not enough data for LLLVAR", de)
		}
		return string(data[3 : 3+length]), 3 + length, nil
	}

	return "", 0, fmt.Errorf("DE[%d]: unknown field type", de)
}

func writeField(de int, value string, defs map[int]fieldDef) ([]byte, error) {
	def, ok := defs[de]
	if !ok {
		return nil, fmt.Errorf("field DE[%d] not defined", de)
	}

	switch def.kind {
	case fixed:
		if len(value) != def.length {
			return nil, fmt.Errorf("DE[%d]: expected length %d, got %d", de, def.length, len(value))
		}
		return []byte(value), nil

	case llvar:
		return []byte(fmt.Sprintf("%02d", len(value)) + value), nil

	case lllvar:
		return []byte(fmt.Sprintf("%03d", len(value)) + value), nil
	}

	return nil, fmt.Errorf("DE[%d]: unknown field type", de)
}
