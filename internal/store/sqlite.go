package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"chanarr/internal/schedule"
)

const schema = `
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	number       TEXT NOT NULL,
	name         TEXT NOT NULL,
	folder       TEXT NOT NULL,
	shuffle      INTEGER NOT NULL DEFAULT 0,
	shuffle_seed INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS epochs (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	channel_id      INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
	start_unix_nano INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS epoch_items (
	epoch_id    INTEGER NOT NULL REFERENCES epochs(id) ON DELETE CASCADE,
	position    INTEGER NOT NULL,
	path        TEXT NOT NULL,
	duration_ns INTEGER NOT NULL,
	PRIMARY KEY (epoch_id, position)
);
`

type sqliteStore struct {
	db *sql.DB
}

func openSQLite(path string) (*sqliteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// SQLite serializes writes regardless; pooling multiple connections
	// through database/sql just invites "database is locked" errors (and,
	// for ":memory:" DSNs, silently fragments state across connections
	// that each get their own private database). One connection is the
	// standard-practice fix for both.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: enable foreign keys: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate schema: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) Close() error { return s.db.Close() }

func (s *sqliteStore) Channels() ([]schedule.Channel, error) {
	rows, err := s.db.Query(`SELECT id, number, name, folder, shuffle, shuffle_seed FROM channels ORDER BY number`)
	if err != nil {
		return nil, fmt.Errorf("store: query channels: %w", err)
	}
	defer rows.Close()

	var channels []schedule.Channel
	for rows.Next() {
		var ch schedule.Channel
		if err := rows.Scan(&ch.ID, &ch.Number, &ch.Name, &ch.Folder, &ch.Shuffle, &ch.ShuffleSeed); err != nil {
			return nil, fmt.Errorf("store: scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

func (s *sqliteStore) SaveChannel(ch schedule.Channel) (schedule.Channel, error) {
	if ch.ID == 0 {
		res, err := s.db.Exec(
			`INSERT INTO channels (number, name, folder, shuffle, shuffle_seed) VALUES (?, ?, ?, ?, ?)`,
			ch.Number, ch.Name, ch.Folder, ch.Shuffle, ch.ShuffleSeed,
		)
		if err != nil {
			return schedule.Channel{}, fmt.Errorf("store: insert channel: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return schedule.Channel{}, fmt.Errorf("store: insert channel: %w", err)
		}
		ch.ID = id
		return ch, nil
	}

	res, err := s.db.Exec(
		`UPDATE channels SET number = ?, name = ?, folder = ?, shuffle = ?, shuffle_seed = ? WHERE id = ?`,
		ch.Number, ch.Name, ch.Folder, ch.Shuffle, ch.ShuffleSeed, ch.ID,
	)
	if err != nil {
		return schedule.Channel{}, fmt.Errorf("store: update channel: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return schedule.Channel{}, fmt.Errorf("store: update channel: %w", err)
	} else if n == 0 {
		return schedule.Channel{}, ErrNotFound
	}
	return ch, nil
}

func (s *sqliteStore) DeleteChannel(id int64) error {
	res, err := s.db.Exec(`DELETE FROM channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete channel: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("store: delete channel: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *sqliteStore) CurrentEpoch(channelID int64) (schedule.Epoch, error) {
	var epochID int64
	var startNano int64
	err := s.db.QueryRow(
		`SELECT id, start_unix_nano FROM epochs WHERE channel_id = ? ORDER BY start_unix_nano DESC, id DESC LIMIT 1`,
		channelID,
	).Scan(&epochID, &startNano)
	if errors.Is(err, sql.ErrNoRows) {
		return schedule.Epoch{}, ErrNotFound
	}
	if err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: query current epoch: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT path, duration_ns FROM epoch_items WHERE epoch_id = ? ORDER BY position ASC`,
		epochID,
	)
	if err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: query epoch items: %w", err)
	}
	defer rows.Close()

	var items []schedule.PlaylistItem
	for rows.Next() {
		var path string
		var durationNs int64
		if err := rows.Scan(&path, &durationNs); err != nil {
			return schedule.Epoch{}, fmt.Errorf("store: scan epoch item: %w", err)
		}
		items = append(items, schedule.PlaylistItem{Path: path, Duration: time.Duration(durationNs)})
	}
	if err := rows.Err(); err != nil {
		return schedule.Epoch{}, err
	}

	return schedule.Epoch{
		ID:        epochID,
		ChannelID: channelID,
		Start:     time.Unix(0, startNano).UTC(),
		Items:     items,
	}, nil
}

func (s *sqliteStore) SaveEpoch(epoch schedule.Epoch) (schedule.Epoch, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: save epoch: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO epochs (channel_id, start_unix_nano) VALUES (?, ?)`,
		epoch.ChannelID, epoch.Start.UTC().UnixNano(),
	)
	if err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: insert epoch: %w", err)
	}
	epochID, err := res.LastInsertId()
	if err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: insert epoch: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO epoch_items (epoch_id, position, path, duration_ns) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: insert epoch items: %w", err)
	}
	defer stmt.Close()

	for i, item := range epoch.Items {
		if _, err := stmt.Exec(epochID, i, item.Path, int64(item.Duration)); err != nil {
			return schedule.Epoch{}, fmt.Errorf("store: insert epoch item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return schedule.Epoch{}, fmt.Errorf("store: save epoch: %w", err)
	}

	epoch.ID = epochID
	return epoch, nil
}

func (s *sqliteStore) setting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: read setting %s: %w", key, err)
	}
	return value, true, nil
}

func (s *sqliteStore) setSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("store: write setting %s: %w", key, err)
	}
	return nil
}

const settingDeviceID = "device_id"

func (s *sqliteStore) DeviceID() (string, error) {
	if id, ok, err := s.setting(settingDeviceID); err != nil {
		return "", err
	} else if ok {
		return id, nil
	}

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("store: generate device id: %w", err)
	}
	id := "chanarr-" + hex.EncodeToString(buf)

	// The single open connection (SetMaxOpenConns(1) in openSQLite) fully
	// serializes concurrent callers, so this get-or-create needs no extra
	// locking of its own.
	if err := s.setSetting(settingDeviceID, id); err != nil {
		return "", err
	}
	final, _, err := s.setting(settingDeviceID)
	if err != nil {
		return "", err
	}
	return final, nil
}

const (
	settingPlexServerURL = "plex_server_url"
	settingPlexToken     = "plex_token"
)

func (s *sqliteStore) PlexConnection() (serverURL, token string, connected bool, err error) {
	serverURL, _, err = s.setting(settingPlexServerURL)
	if err != nil {
		return "", "", false, err
	}
	token, _, err = s.setting(settingPlexToken)
	if err != nil {
		return "", "", false, err
	}
	return serverURL, token, serverURL != "" && token != "", nil
}

func (s *sqliteStore) SavePlexConnection(serverURL, token string) error {
	if err := s.setSetting(settingPlexServerURL, serverURL); err != nil {
		return err
	}
	return s.setSetting(settingPlexToken, token)
}
