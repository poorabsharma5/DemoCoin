package main

import (
	"testing"
)

func TestCalculateMerkleRoot(t *testing.T) {
	// Case 0: Empty
	if root := CalculateMerkleRoot([]Transaction{}); root != "" {
		t.Errorf("Expected empty root, got %s", root)
	}

	// Case 1: Single Transaction
	tx1 := Transaction{Data: "tx1"}
	root1 := CalculateMerkleRoot([]Transaction{tx1})
	hash1 := calculateTransactionHash(tx1)
	if root1 != hash1 {
		t.Errorf("Expected root %s, got %s", hash1, root1)
	}

	// Case 2: Two Transactions
	tx2 := Transaction{Data: "tx2"}
	root2 := CalculateMerkleRoot([]Transaction{tx1, tx2})
	hash2 := calculateTransactionHash(tx2)
	expectedRoot2 := hashPair(hash1, hash2)
	if root2 != expectedRoot2 {
		t.Errorf("Expected root %s, got %s", expectedRoot2, root2)
	}

	// Case 3: Three Transactions (Odd number, should duplicate last)
	tx3 := Transaction{Data: "tx3"}
	root3 := CalculateMerkleRoot([]Transaction{tx1, tx2, tx3})
	hash3 := calculateTransactionHash(tx3)

	// L1: H1, H2, H3
	// L2: H(H1+H2), H(H3+H3)
	// Root: H(L2_0 + L2_1)

	h12 := hashPair(hash1, hash2)
	h33 := hashPair(hash3, hash3)
	expectedRoot3 := hashPair(h12, h33)

	if root3 != expectedRoot3 {
		t.Errorf("Expected root %s, got %s", expectedRoot3, root3)
	}
}
