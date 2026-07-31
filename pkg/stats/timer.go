package stats

import (
	"bytes"
	"fmt"
	"slices"
	"time"
)

// A Timer that can be started and stopped and accumulates the total time it
// was running (the time between Start() and Stop()).
type Timer struct {
	name     fmt.Stringer
	created  int
	start    time.Time
	duration time.Duration
}

// Start the timer
func (t *Timer) Start() *Timer {
	t.start = time.Now()
	return t
}

// Stop the timer
func (t *Timer) Stop() {
	t.duration += time.Now().Sub(t.start)
}

// ElapsedTime returns the time that passed since starting the timer
func (t *Timer) ElapsedTime() time.Duration {
	return time.Since(t.start)
}

// Duration returns the duration value of the timer in seconds
func (t *Timer) Duration() float64 {
	return t.duration.Seconds()
}

// Return a string representation of the timer
func (t *Timer) String() string {
	return fmt.Sprintf("%s: %s", t.name, t.duration)
}

// A TimerGroup represents a group of timers relevant to a single query
type TimerGroup struct {
	timers map[fmt.Stringer]*Timer
}

// NewTimerGroup returns a new TimerGroup
func NewTimerGroup() *TimerGroup {
	return &TimerGroup{
		timers: make(map[fmt.Stringer]*Timer),
	}
}

// GetTimer gets (and creates, if necessary) a Timer for a given code section
func (t *TimerGroup) GetTimer(name fmt.Stringer) *Timer {
	if timer, exists := t.timers[name]; exists {
		return timer
	}
	timer := &Timer{
		name:    name,
		created: len(t.timers),
	}
	t.timers[name] = timer
	return timer
}

// Return a string representation of a TimerGroup
func (t *TimerGroup) String() string {
	timers := make([]*Timer, 0, len(t.timers))
	for _, timer := range t.timers {
		timers = append(timers, timer)
	}

	slices.SortFunc(timers, func(a, b *Timer) int {
		return a.created - b.created
	})
	result := &bytes.Buffer{}
	for _, timer := range timers {
		fmt.Fprintf(result, "%s\n", timer)
	}
	return result.String()
}
