package schedule

import "errors"

var (
	// ErrEmptyEpoch is returned by ProgramAt when the epoch has no playlist
	// items to schedule.
	ErrEmptyEpoch = errors.New("schedule: epoch has no items")

	// ErrBeforeEpochStart is returned by ProgramAt when t precedes the
	// epoch's start — the caller picked the wrong epoch for t.
	ErrBeforeEpochStart = errors.New("schedule: t is before epoch start")

	// ErrZeroCycleLength is returned by ProgramAt when the epoch's items sum
	// to a zero or negative cycle length (e.g. all-zero cached durations).
	ErrZeroCycleLength = errors.New("schedule: epoch cycle length is zero")
)
