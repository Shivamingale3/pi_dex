package parser

import (
	"time"
)

func entryTimestamp(entry map[string]any) time.Time {
	micros, ok := entry["__REALTIME_TIMESTAMP"]
	if ok {
		switch v := micros.(type) {
		case string:
			if us, err := parseInt64(v); err == nil {
				return time.UnixMicro(us)
			}
		case int64:
			return time.UnixMicro(v)
		case float64:
			return time.UnixMicro(int64(v))
		}
	}
	return time.Now()
}

func entryMessage(entry map[string]any) string {
	msg, ok := entry["MESSAGE"]
	if !ok {
		return ""
	}
	return messageToString(msg)
}

func messageToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case []any:
		var result string
		for i, item := range val {
			if i > 0 {
				result += "\n"
			}
			result += messageToString(item)
		}
		return result
	}
	return ""
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}
