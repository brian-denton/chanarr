// Package store persists chanarr's state in SQLite: Channels, Epochs (with
// their cached PlaylistItems and durations), the stable tuner DeviceID, and
// the optional Plex connection (server URL + token). See spec.md §10 — exact
// schema/migrations are implementation detail, not a spec-level decision.
//
// Uses modernc.org/sqlite (a pure-Go driver, no CGO) so chanarr stays a
// single static binary (spec.md §1) — a CGO-based driver would break cross
// compilation.
package store

import "chanarr/internal/schedule"

// Store is the persistence boundary the rest of chanarr depends on.
type Store interface {
	Channels() ([]schedule.Channel, error)
	// Channel returns a single channel by ID, or ErrNotFound.
	Channel(id int64) (schedule.Channel, error)
	// SaveChannel inserts a new channel when ch.ID == 0, or updates the
	// existing row otherwise. Returns ErrNotFound if ch.ID is set but no
	// such channel exists.
	SaveChannel(ch schedule.Channel) (schedule.Channel, error)
	DeleteChannel(id int64) error

	// CurrentEpoch returns the most recently stamped epoch for a channel —
	// the one a rolling guide horizon or a fresh tune-in should use.
	// Returns ErrNotFound if the channel has no epoch yet.
	CurrentEpoch(channelID int64) (schedule.Epoch, error)
	// SaveEpoch always inserts a new, immutable epoch row (docs/adr/0001)
	// — it never updates an existing one.
	SaveEpoch(epoch schedule.Epoch) (schedule.Epoch, error)

	// DeviceID returns chanarr's stable HDHomeRun DeviceID (internal/tuner),
	// generating and persisting one on first call.
	DeviceID() (string, error)

	// PlexClientIdentifier returns the stable identifier chanarr presents
	// to plex.tv's PIN-link flow (internal/plexlink), generating and
	// persisting one on first call. Deliberately distinct from DeviceID —
	// they identify chanarr to two different Plex systems (the local PMS's
	// tuner discovery vs. plex.tv's account-linking identity).
	PlexClientIdentifier() (string, error)

	// PlexConnection reports the optional Plex connection (docs/adr/0002).
	// connected is false, with empty strings, until SavePlexConnection has
	// been called at least once — never an error on its own.
	PlexConnection() (serverURL, token string, connected bool, err error)
	SavePlexConnection(serverURL, token string) error

	// ShareCredentials returns the stored login for a network share
	// (internal/netfs), or empty strings with no error when none is
	// stored — an unauthenticated (guest) attempt is the correct
	// fallback, not a failure.
	ShareCredentials(protocol, host, share string) (username, password string, err error)
	// SaveShareCredentials stores (or replaces) the login for a share.
	SaveShareCredentials(protocol, host, share, username, password string) error

	Close() error
}

// Open opens (creating and migrating if needed) the SQLite database at
// path. Use ":memory:" for an ephemeral database (tests).
func Open(path string) (Store, error) {
	return openSQLite(path)
}
