package store

import (
	"os"
	"time"
)

// TimestampStore abstracts persistence for the timestamp of the last playlist check.
type TimestampStore interface {
	Get() (time.Time, error)
	Put(time.Time) error
}

// FileStore stores the timestamp in a plain text file using RFC3339 format.
type FileStore struct {
	Path string
}

func (f FileStore) Get() (time.Time, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// no timestamp yet – treat as zero time
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func (f FileStore) Put(t time.Time) error {
	return os.WriteFile(f.Path, []byte(t.Format(time.RFC3339)), 0o644)
}
