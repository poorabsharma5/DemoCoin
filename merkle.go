package main

import (
	"crypto/sha256"
	"encoding/hex"
)

// CalculateMerkleRoot computes the Merkle Root of a list of transactions.
// It uses a recursive approach to pair up hashes until a single root is found.
func CalculateMerkleRoot(transactions []Transaction) string {
	if len(transactions) == 0 {
		return ""
	}

	var hashes []string
	for _, tx := range transactions {
		hashes = append(hashes, calculateTransactionHash(tx))
	}

	return calculateMerkleRootRecursive(hashes)
}

func calculateMerkleRootRecursive(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	if len(hashes) == 1 {
		return hashes[0]
	}

	var newLevel []string
	for i := 0; i < len(hashes); i += 2 {
		if i+1 < len(hashes) {
			newLevel = append(newLevel, hashPair(hashes[i], hashes[i+1]))
		} else {
			// Odd number of nodes, duplicate the last one
			newLevel = append(newLevel, hashPair(hashes[i], hashes[i]))
		}
	}

	return calculateMerkleRootRecursive(newLevel)
}

func hashPair(left, right string) string {
	h := sha256.New()
	h.Write([]byte(left + right))
	return hex.EncodeToString(h.Sum(nil))
}
