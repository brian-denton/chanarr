// Package store persists chanarr's state in SQLite: Channels, Epochs (with
// their cached PlaylistItems and durations), the stable tuner DeviceID, and
// the optional Plex connection (server URL + token). See spec.md §10 — exact
// schema/migrations are implementation detail, not a spec-level decision.
package store

import "chanarr/internal/schedule"

// Store is the persistence boundary the rest of chanarr depends on. TODO:
// implement against database/sql + a SQLite driver.
type Store interface {
	Channels() ([]schedule.Channel, error)
	SaveChannel(schedule.Channel) (schedule.Channel, error)
	DeleteChannel(id int64) error

	CurrentEpoch(channelID int64) (schedule.Epoch, error)
	SaveEpoch(schedule.Epoch) (schedule.Epoch, error)

	DeviceID() (string, error)

	PlexConnection() (serverURL, token string, connected bool, err error)
	SavePlexConnection(serverURL, token string) error
}

// Open opens (creating if needed) the SQLite database at path. TODO: implement.
func Open(path string) (Store, error) {
	return nil, errNotImplemented
}
