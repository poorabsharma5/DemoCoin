package main

import (
	"encoding/json"
	"fmt"

	"go.etcd.io/bbolt"
)

const (
	blocksBucket = "blocks"
	stateBucket  = "state"
)

var DB *bbolt.DB

// InitDB initializes the BoltDB database and creates necessary buckets.
func InitDB(dbFile string) error {
	var err error
	DB, err = bbolt.Open(dbFile, 0600, nil)
	if err != nil {
		return err
	}

	return DB.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(blocksBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(stateBucket))
		return err
	})
}

// CloseDB closes the database connection.
func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}

// SaveBlock persists a block to the database.
func SaveBlock(block Block) error {
	return DB.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encoded, err := json.Marshal(block)
		if err != nil {
			return err
		}
		// Key: Block Index (as formatted string for ordering? Or Hash?)
		// Usually Hash is key. But for simple retrieval/loading ordered list, let's use Index or just append.
		// Since we load entire chain on startup in this simple model, let's just use Index.
		// Note: bbolt keys are byte slices. Lexicographical order.
		// Using BigEndian uint64 would be better, but fmt.Sprintf("%d") sorts 1, 10, 2.
		// Let's use Hash as key and store "LastHash" to traverse back?
		// Or since we just reload everything into memory, we can just iterate.
		// Let's save by Hash for standard lookup, AND maybe Index if needed.
		// For loading all: we can iterate the bucket.
		// Let's key by Index (padded) to allow easy ordered iteration?
		// "000001", "000002".
		key := fmt.Sprintf("%08d", block.Index)
		return b.Put([]byte(key), encoded)
	})
}

// LoadBlockchain loads all blocks from the database.
func LoadBlockchain() ([]Block, error) {
	var blockchain []Block
	err := DB.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var block Block
			err := json.Unmarshal(v, &block)
			if err != nil {
				return err
			}
			blockchain = append(blockchain, block)
		}
		return nil
	})
	return blockchain, err
}

// SaveState persists the state to the database.
// This overwrites existing state entries or adds new ones.
// Ideally usage: call this with the modified accounts or the whole state map?
// Since we have atomic blocks, we can just iterate the whole map and save it.
// Optimization: Only save dirtied accounts. But here, save all for simplicity or pass map.
func SaveState(state map[string]*Account) error {
	return DB.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		for addr, account := range state {
			encoded, err := json.Marshal(account)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(addr), encoded); err != nil {
				return err
			}
		}
		return nil
	})
}

// LoadState loads the entire state from the database.
func LoadState() (map[string]*Account, error) {
	state := make(map[string]*Account)
	err := DB.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(stateBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var account Account
			err := json.Unmarshal(v, &account)
			if err != nil {
				return err
			}
			state[string(k)] = &account
		}
		return nil
	})
	return state, err
}
