package ratelimit

import (
	"time"

	"github.com/andrewbytecoder/nmq/pkg/clock"
)

// Note: This file is inspired by:
// https://github.com/prashantv/go-bench/blob/master/ratelimit

// Limiter is used to rate-limit some process, possibly across goroutines.
// The process is expected to call Take() before every iteration, which
// may block to throttle the goroutine.
type Limiter interface {
	// Take should block to make sure that the RPS is met
	Take() time.Time
}

// Clock is the minimum necessary interface to instantiate a rate limiter with
// a clock or mock clock, compatible with clocks created using
// github.com/andres-erbsen/clock.
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

// config configures a limiter.
type config struct {
	clock Clock
	slack int
	per   time.Duration
}

// New returns a limiter that will limit to the given RPS.
func New(rate int, opts ...Option) Limiter {
	return newAtomicInt64Based(rate, opts...)
}

// buildConfig combines defaults with options
func buildConfig(opts []Option) config {
	c := config{
		clock: clock.New(),
		slack: 10,
		per:   time.Second,
	}

	for _, opt := range opts {
		opt.apply(&c)
	}
	return c
}

// Option configures a Limiter.
type Option interface {
	apply(*config)
}

type clockOption struct {
	clock Clock
}

func (o clockOption) apply(c *config) {
	c.clock = o.clock
}

// WithClock returns an option for ratelimit.New that provides an alternate
// Clock implementation, typically a mock Clock for testing.
func WithClock(clock Clock) Option {
	return clockOption{clock: clock}
}

type slackOption int

func (o slackOption) apply(c *config) {
	c.slack = int(o)
}

// WithoutSlack configures the limiter to be strict and not to accumulate
// previously "unspent" requests for future bursts of traffic.
var WithoutSlack Option = slackOption(0)

// WithSlack configures custom slack.
// slack allows the limiter to accumulate "unspent" requests
// for future bursts of traffic.
func WithSlack(slack int) Option {
	return slackOption(slack)
}

type perOption time.Duration

func (o perOption) apply(c *config) {
	c.per = time.Duration(o)
}

// Per allows  configuring limits for different time windows.
//
// The default window is one second, so New(100) produces a one hundred per second
// (100Hz) rate limiter.
// New(2, Per(60*time.Second)) creates a 2 per  minute rate limiter.
func Per(per time.Duration) Option {
	return perOption(per)
}

type unlimited struct {
}

// NewUnlimited returns a RateLimiter that is not limited
func NewUnlimited() Limiter {
	return unlimited{}
}

func (unlimited) Take() time.Time {
	return time.Now()
}
