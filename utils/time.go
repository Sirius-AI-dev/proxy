package utils

import "time"

func ParseDuration(s string, defaultValue time.Duration) time.Duration {
	duration, err := time.ParseDuration(s)
	if err != nil {
		return defaultValue
	}
	return duration
}
