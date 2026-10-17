package ratelimit

import (
	"strconv"
	"time"
)

// RetryAfterSeconds formats d as the value of a Retry-After header: whole
// seconds, rounded up, with a minimum of one. A rejected client told to retry
// after 0 seconds would retry immediately and be rejected again.
func RetryAfterSeconds(d time.Duration) string {
	seconds := (d + time.Second - 1) / time.Second
	if seconds < 1 {
		seconds = 1
	}

	return strconv.FormatInt(int64(seconds), 10)
}
