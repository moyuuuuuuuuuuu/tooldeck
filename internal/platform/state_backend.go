package platform

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// StateBackend isolates persistence from the control-plane model. Implementations
// commit a complete logical change atomically; SQLite only writes changed rows.
type StateBackend interface {
	Load() ([]byte, error)
	Save(State) error
	Close() error
}

type sqliteState struct {
	db       *sql.DB
	previous map[string]string
}

func openSQLite(root string) (*sqliteState, error) {
	path := filepath.Join(root, "state.db")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, query := range []string{"PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", `CREATE TABLE IF NOT EXISTS state_records (collection TEXT NOT NULL, id TEXT NOT NULL, value TEXT NOT NULL CHECK(json_valid(value)), PRIMARY KEY(collection,id))`, `CREATE TABLE IF NOT EXISTS state_meta (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL)`} {
		if _, err = db.Exec(query); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &sqliteState{db: db, previous: map[string]string{}}, nil
}
func (b *sqliteState) Close() error { return b.db.Close() }
func (b *sqliteState) Load() ([]byte, error) {
	var version int
	err := b.db.QueryRow("SELECT version FROM state_meta WHERE id=1").Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported state database version %d", version)
	}
	rows, err := b.db.Query("SELECT collection,id,value FROM state_records")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]json.RawMessage{}
	groups := map[string]map[string]json.RawMessage{}
	for rows.Next() {
		var group, id, value string
		if err = rows.Scan(&group, &id, &value); err != nil {
			return nil, err
		}
		b.previous[group+"\x00"+id] = value
		if group == "settings" {
			result[id] = json.RawMessage(value)
		} else {
			if groups[group] == nil {
				groups[group] = map[string]json.RawMessage{}
			}
			groups[group][id] = json.RawMessage(value)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for group, values := range groups {
		raw, e := json.Marshal(values)
		if e != nil {
			return nil, e
		}
		result[group] = raw
	}
	return json.Marshal(result)
}
func stateRows(state State) (map[string]string, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	if err = json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	// These objects are singleton settings, not keyed entity collections.
	singletons := map[string]bool{"donation": true, "review_required": true, "storage_quotas": true}
	out := map[string]string{}
	for field, value := range fields {
		if !singletons[field] && len(value) > 0 && value[0] == '{' {
			entries := map[string]json.RawMessage{}
			if err = json.Unmarshal(value, &entries); err != nil {
				return nil, err
			}
			for id, entry := range entries {
				out[field+"\x00"+id] = string(entry)
			}
		} else {
			out["settings\x00"+field] = string(value)
		}
	}
	return out, nil
}
func (b *sqliteState) Save(state State) error {
	next, err := stateRows(state)
	if err != nil {
		return err
	}
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range next {
		if b.previous[key] == value {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		if _, err = tx.Exec("INSERT INTO state_records(collection,id,value) VALUES(?,?,?) ON CONFLICT(collection,id) DO UPDATE SET value=excluded.value", parts[0], parts[1], value); err != nil {
			return err
		}
	}
	for key := range b.previous {
		if _, exists := next[key]; !exists {
			parts := strings.SplitN(key, "\x00", 2)
			if _, err = tx.Exec("DELETE FROM state_records WHERE collection=? AND id=?", parts[0], parts[1]); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec("INSERT INTO state_meta(id,version) VALUES(1,1) ON CONFLICT(id) DO NOTHING"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	b.previous = next
	return nil
}

func backupLegacy(root string, raw []byte) error {
	path := filepath.Join(root, "state.json.pre-sqlite.bak")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		old, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if !bytes.Equal(old, raw) {
			return errors.New("legacy state differs from migration backup; preserve both before retrying")
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	return err
}

// ExportState writes a consistent portable snapshot to a new file without
// running startup recovery or changing the live state. It never overwrites.
func ExportState(root, destination string) error {
	path, err := filepath.Abs(filepath.Join(root, "state.db"))
	if err != nil {
		return err
	}
	if _, err = os.Stat(path); err != nil {
		return err
	}
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	b := &sqliteState{db: db, previous: map[string]string{}}
	raw, err := b.Load()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	return err
}
