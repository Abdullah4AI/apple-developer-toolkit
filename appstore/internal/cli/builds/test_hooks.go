package builds

import "time"

// SetWaitClockForTesting replaces the clock builds wait uses for elapsed-time
// reporting. The wait deadline itself still uses the real clock. It returns a
// restore function to reset the previous clock.
func SetWaitClockForTesting(fn func() time.Time) func() {
	previous := buildsWaitNow
	if fn == nil {
		buildsWaitNow = time.Now
	} else {
		buildsWaitNow = fn
	}
	return func() {
		buildsWaitNow = previous
	}
}
