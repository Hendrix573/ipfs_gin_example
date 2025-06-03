package storage

import (
	"errors"

	badger "github.com/dgraph-io/badger/v4"
)

// Store defines the interface for block storage
type Store interface {
	Put(cid []byte, data []byte) error
	Get(cid []byte) ([]byte, error)
	Close() error
}

// BadgerStore is a BadgerDB implementation of the Store interface
type BadgerStore struct {
	db *badger.DB
}

// NewBadgerStore creates a new BadgerStore
func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	// opts.Logger = nil // Optional: Disable badger logs for cleaner output

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerStore{db: db}, nil
}

// Put stores a block in BadgerDB
func (s *BadgerStore) Put(cid []byte, data []byte) error {
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(cid, data)
	})
	return err
}

// Get retrieves a block from BadgerDB
func (s *BadgerStore) Get(cid []byte) ([]byte, error) {
	var data []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(cid)
		if err != nil {
			return err // badger.ErrKeyNotFound
		}
		data, err = item.ValueCopy(nil)
		return err
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, errors.New("block not found")
	}
	return data, err
}

// Close closes the BadgerDB
func (s *BadgerStore) Close() error {
	return s.db.Close()
}
