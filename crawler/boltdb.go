package crawler

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/gocolly/colly/v2/storage"
	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("colly")

type BoltDBStorage struct {
	DBPath string
	db     *bolt.DB
	mu     sync.RWMutex
}

// Init initializes the BoltDB database
func (s *BoltDBStorage) Init() error {
	dbDir := filepath.Dir(s.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for BoltDB: %w", err)
	}

	db, err := bolt.Open(s.DBPath, 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to open BoltDB: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	s.db = db
	return nil
}

// Visited implements storage.Storage interface
func (s *BoltDBStorage) Visited(requestID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		key := []byte(fmt.Sprintf("v:%d", requestID))
		return b.Put(key, []byte("1"))
	})
}

// IsVisited implements storage.Storage interface
func (s *BoltDBStorage) IsVisited(requestID uint64) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var visited bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		key := []byte(fmt.Sprintf("v:%d", requestID))
		v := b.Get(key)
		visited = v != nil
		return nil
	})
	return visited, err
}

// Cookies implements storage.Storage interface
func (s *BoltDBStorage) Cookies(u *url.URL) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cookies string
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		key := []byte(fmt.Sprintf("c:%s", u))
		v := b.Get(key)
		if v != nil {
			cookies = string(v)
		}
		return nil
	})
	return cookies
}

// SetCookies implements storage.Storage interface
func (s *BoltDBStorage) SetCookies(u *url.URL, cookies string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		key := []byte(fmt.Sprintf("c:%s", u))
		return b.Put(key, []byte(cookies))
	})
}

// Clear removes all data from storage
func (s *BoltDBStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketName); err != nil {
			return err
		}
		_, err := tx.CreateBucket(bucketName)
		return err
	})
}

// Close closes the BoltDB database
func (s *BoltDBStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Ensure BoltDBStorage implements storage.Storage interface
var _ storage.Storage = (*BoltDBStorage)(nil)
