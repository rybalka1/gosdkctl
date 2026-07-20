package version

import (
	"strconv"
	"strings"
)

func IsGoVersionDir(name string) bool {
	if !strings.HasPrefix(name, "go") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(name, "go"), ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func Compare(left, right string) int {
	lv := parse(left)
	rv := parse(right)

	maxLen := len(lv)
	if len(rv) > maxLen {
		maxLen = len(rv)
	}

	for i := 0; i < maxLen; i++ {
		li := component(lv, i)
		ri := component(rv, i)
		switch {
		case li < ri:
			return -1
		case li > ri:
			return 1
		}
	}

	return 0
}

func parse(name string) []int {
	raw := strings.TrimPrefix(name, "go")
	parts := strings.Split(raw, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, value)
	}
	return out
}

func component(parts []int, idx int) int {
	if idx >= len(parts) {
		return 0
	}
	return parts[idx]
}
