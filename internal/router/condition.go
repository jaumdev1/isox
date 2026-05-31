package router

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/isox/internal/config"
	"github.com/isox/internal/iso8583"
)

func matchAll(msg *iso8583.Message, conditions []config.Condition) bool {
	for _, c := range conditions {
		if !match(msg, c) {
			return false
		}
	}
	return true
}

func match(msg *iso8583.Message, c config.Condition) bool {
	value := fieldValue(msg, c.Field)

	switch c.Operator {
	case "==":
		return value == c.Value
	case "!=":
		return value != c.Value
	case "starts_with":
		return strings.HasPrefix(value, c.Value)
	case "ends_with":
		return strings.HasSuffix(value, c.Value)
	case "contains":
		return strings.Contains(value, c.Value)
	case "regex":
		matched, err := regexp.MatchString(c.Value, value)
		return err == nil && matched
	}
	return false
}

func fieldValue(msg *iso8583.Message, field string) string {
	if field == "mti" {
		return msg.MTI
	}
	if strings.HasPrefix(field, "de[") {
		s := strings.TrimSuffix(strings.TrimPrefix(field, "de["), "]")
		de, err := strconv.Atoi(s)
		if err != nil {
			return ""
		}
		return msg.Fields[de]
	}
	return ""
}
