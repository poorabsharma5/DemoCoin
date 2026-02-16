package main

import (
	"os"
	"testing"
	"time"
)

func TestStorage(t *testing.T) {
	// Setup temp DB
	testDB := "test_blockchain.db"
	os.Remove(testDB)

	// Init DB logic manually to point to test file (or hack InitDB to accept filename)
	// Since InitDB uses const filename, we can try to change it or just open manually in test and assume Save/Load use global DB.
	// But `storage.go` references global DB.
	// Let's modify `InitDB` to take filename in `storage.go`? Or for now just swap `dbFile` IF it was var. It is const.
	// We should update `storage.go`.

	// Better: Update InitDB to take filename.
	// Wait, cannot modify storage.go in this same call.
	// I will just rely on `storage.go` using `dbFile` which is `blockchain.db`.
	// I will backup existing `blockchain.db` if it exists.

	originalDB := "blockchain.db"
	backupDB := "blockchain.db.bak"
	if _, err := os.Stat(originalDB); err == nil {
		os.Rename(originalDB, backupDB)
		defer os.Rename(backupDB, originalDB)
	}
	defer os.Remove(originalDB)

	if err := InitDB(testDB); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer CloseDB()

	// 1. Save Block
	block := Block{
		Index:     1,
		Timestamp: time.Now().String(),
		Hash:      "test_hash",
	}
	if err := SaveBlock(block); err != nil {
		t.Fatalf("Failed to save block: %v", err)
	}

	// 2. Load Chain
	chain, err := LoadBlockchain()
	if err != nil {
		t.Fatalf("Failed to load chain: %v", err)
	}
	if len(chain) == 0 {
		t.Fatal("Chain is empty")
	}
	if chain[0].Hash != block.Hash {
		t.Errorf("Expected hash %s, got %s", block.Hash, chain[0].Hash)
	}

	// 3. Save State
	// Using map directly
	stateMap := make(map[string]*Account)
	stateMap["alice"] = &Account{Balance: 100, Nonce: 5}
	if err := SaveState(stateMap); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// 4. Load State
	loadedState, err := LoadState()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	if loadedState["alice"].Balance != 100 {
		t.Errorf("Expected balance 100, got %d", loadedState["alice"].Balance)
	}
}
