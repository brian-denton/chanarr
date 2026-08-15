package store

import "errors"

// ErrNotFound is returned when a lookup by ID (or an update/delete
// targeting one) finds no matching row.
var ErrNotFound = errors.New("store: not found")
