package main

import (
	"testing"
)

func resetState() {
	State = make(map[string]*Account)
}

func TestApplyTransaction_Valid(t *testing.T) {
	resetState()

	priv, pub, _ := GenerateKeyPair()
	sender := PubKeyToAddress(pub)
	receiver := "bob"

	// Setup sender with funds
	State[sender] = &Account{Balance: 100, Nonce: 0}

	tx := Transaction{
		PublicKey: pub,
		To:        receiver,
		Amount:    50,
		Fee:       10,
		Nonce:     0,
	}

	// Sign
	hash := calculateTransactionHash(tx)
	tx.Signature = SignData(priv, []byte(hash))

	err := ApplyTransaction(State, tx)
	if err != nil {
		t.Fatalf("Expected valid transaction to apply, got error: %v", err)
	}

	if State[sender].Balance != 40 { // 100 - 50 - 10 = 40
		t.Errorf("Expected sender balance 40, got %d", State[sender].Balance)
	}
	if State[sender].Nonce != 1 {
		t.Errorf("Expected sender nonce 1, got %d", State[sender].Nonce)
	}
	if State[receiver].Balance != 50 {
		t.Errorf("Expected receiver balance 50, got %d", State[receiver].Balance)
	}
}

func TestApplyTransaction_InsufficientBalance(t *testing.T) {
	resetState()
	priv, pub, _ := GenerateKeyPair()
	sender := PubKeyToAddress(pub)
	State[sender] = &Account{Balance: 50, Nonce: 0}

	tx := Transaction{
		PublicKey: pub,
		To:        "bob",
		Amount:    45,
		Fee:       10, // Total 55 > 50
		Nonce:     0,
	}
	tx.Signature = SignData(priv, []byte(calculateTransactionHash(tx)))

	err := ApplyTransaction(State, tx)
	if err == nil {
		t.Fatal("Expected error for insufficient balance, got nil")
	}
}

func TestApplyTransaction_WrongNonce(t *testing.T) {
	resetState()
	priv, pub, _ := GenerateKeyPair()
	sender := PubKeyToAddress(pub)
	State[sender] = &Account{Balance: 100, Nonce: 5}

	tx := Transaction{
		PublicKey: pub,
		To:        "bob",
		Amount:    10,
		Fee:       0,
		Nonce:     0, // Expected 5
	}
	tx.Signature = SignData(priv, []byte(calculateTransactionHash(tx)))

	err := ApplyTransaction(State, tx)
	if err == nil {
		t.Fatal("Expected error for wrong nonce, got nil")
	}
}

func TestApplyTransaction_DoubleSpend(t *testing.T) {
	resetState()
	priv, pub, _ := GenerateKeyPair()
	sender := PubKeyToAddress(pub)
	State[sender] = &Account{Balance: 100, Nonce: 0}

	tx := Transaction{
		PublicKey: pub,
		To:        "bob",
		Amount:    100,
		Fee:       0,
		Nonce:     0,
	}
	tx.Signature = SignData(priv, []byte(calculateTransactionHash(tx)))

	// First spend OK
	if err := ApplyTransaction(State, tx); err != nil {
		t.Fatalf("First tx failed: %v", err)
	}

	// Second spend fails (nonce mismatch AND balance 0, but nonce checked first usually)
	// If we retry with next nonce but 0 balance:
	tx2 := Transaction{
		PublicKey: pub,
		To:        "charlie",
		Amount:    10,
		Fee:       0,
		Nonce:     1,
	}
	tx2.Signature = SignData(priv, []byte(calculateTransactionHash(tx2)))

	if err := ApplyTransaction(State, tx2); err == nil {
		t.Fatal("Expected error for double spend (insufficient balance), got nil")
	}
}

func TestApplyTransaction_InvalidSignature(t *testing.T) {
	resetState()
	_, pub, _ := GenerateKeyPair()
	// No balance needed for this check if logic checks signature first

	tx := Transaction{
		PublicKey: pub,
		To:        "bob",
		Amount:    10,
		Fee:       0,
		Nonce:     0,
		Signature: []byte("invalid_signature"),
	}

	err := ApplyTransaction(State, tx)
	if err == nil {
		t.Fatal("Expected error for invalid signature, got nil")
	}
	if err.Error() != "invalid signature" {
		t.Fatalf("Expected 'invalid signature' error, got: %v", err)
	}
}

func TestStateRoot(t *testing.T) {
	resetState()
	// Check empty root
	root1 := ComputeStateRoot(State)
	if root1 != "" {
		t.Errorf("Expected empty root, got %s", root1)
	}

	// Add account
	State["alice"] = &Account{Balance: 100, Nonce: 0}
	root2 := ComputeStateRoot(State)
	if root2 == "" || root2 == root1 {
		t.Error("Root should change after state update")
	}

	// Change balance
	State["alice"].Balance = 50
	root3 := ComputeStateRoot(State)
	if root3 == root2 {
		t.Error("Root should change after balance update")
	}
}
