package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
)

type Account struct {
	Balance uint64
	Nonce   uint64
	Staked  uint64
}

var State = make(map[string]*Account)

func GetBalance(address string) uint64 {
	if acc, ok := State[address]; ok {
		return acc.Balance
	}
	return 0
}

func GetNonce(address string) uint64 {
	if acc, ok := State[address]; ok {
		return acc.Nonce
	}
	return 0
}

// ApplyTransaction applies a transaction to the given state.
// It returns an error if the transaction is invalid or the sender has insufficient funds.
func ApplyTransaction(state map[string]*Account, tx Transaction) error {
	// 1. Verify Signature
	txHash := calculateTransactionHash(tx)
	if !VerifySignature(tx.PublicKey, []byte(txHash), tx.Signature) {
		return errors.New("invalid signature")
	}

	sender := PubKeyToAddress(tx.PublicKey)
	receiver := tx.To

	// Get sender account
	senderAcc, ok := state[sender]
	if !ok {
		return fmt.Errorf("sender %s does not exist", sender)
	}

	// 2. Nonce check
	if tx.Nonce != senderAcc.Nonce {
		return fmt.Errorf("incorrect nonce: expected %d, got %d", senderAcc.Nonce, tx.Nonce)
	}

	// 3. Balance check (Amount + Fee)
	totalCost := tx.Amount + tx.Fee
	if senderAcc.Balance < totalCost {
		return fmt.Errorf("insufficient balance: have %d, want %d", senderAcc.Balance, totalCost)
	}

	// 4. State Mutation
	if tx.Type == "STAKE" {
		senderAcc.Balance -= totalCost
		senderAcc.Staked += tx.Amount
	} else {
		senderAcc.Balance -= totalCost

		// Update receiver
		receiverAcc, ok := state[receiver]
		if !ok {
			receiverAcc = &Account{Balance: 0, Nonce: 0}
			state[receiver] = receiverAcc
		}
		receiverAcc.Balance += tx.Amount
	}

	senderAcc.Nonce++

	return nil
}

// CloneState creates a deep copy of the state.
func CloneState(original map[string]*Account) map[string]*Account {
	clone := make(map[string]*Account)
	for addr, acc := range original {
		clone[addr] = &Account{
			Balance: acc.Balance,
			Nonce:   acc.Nonce,
			Staked:  acc.Staked,
		}
	}
	return clone
}

// ComputeStateRoot calculates the Merkle Root (or simple hash) of the state.
func ComputeStateRoot(state map[string]*Account) string {
	if len(state) == 0 {
		return ""
	}

	var addresses []string
	for addr := range state {
		addresses = append(addresses, addr)
	}
	sort.Strings(addresses)

	h := sha256.New()
	for _, addr := range addresses {
		acc := state[addr]
		// Hash: Address + Balance + Nonce + Staked
		record := fmt.Sprintf("%s%d%d%d", addr, acc.Balance, acc.Nonce, acc.Staked)
		h.Write([]byte(record))
	}
	return hex.EncodeToString(h.Sum(nil))
}

type Validator struct {
	Address string
	Stake   uint64
}

func GetValidators(state map[string]*Account) []Validator {
	var validators []Validator
	for addr, acc := range state {
		if acc.Staked > 0 {
			validators = append(validators, Validator{Address: addr, Stake: acc.Staked})
		}
	}
	// Sort by address for deterministic order
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	})
	return validators
}

func SelectProposer(validators []Validator, prevBlockHash string) string {
	if len(validators) == 0 {
		return ""
	}

	var totalStake uint64
	for _, v := range validators {
		totalStake += v.Stake
	}

	if totalStake == 0 {
		return ""
	}

	// Seed RNG deterministically
	// Use CRC32 of hash or just parse first 8 bytes
	seedVal := int64(0)
	if len(prevBlockHash) >= 8 {
		bytes, _ := hex.DecodeString(prevBlockHash)
		if len(bytes) >= 8 {
			seedVal = int64(bytes[0]) | int64(bytes[1])<<8 | int64(bytes[2])<<16 | int64(bytes[3])<<24 |
				int64(bytes[4])<<32 | int64(bytes[5])<<40 | int64(bytes[6])<<48 | int64(bytes[7])<<56
		}
	}

	// Use PCG for better randomness properties seeded by block hash
	r := rand.New(rand.NewPCG(uint64(seedVal), uint64(seedVal+1)))

	target := r.Uint64N(totalStake)

	var current uint64 = 0
	for _, v := range validators {
		current += v.Stake
		if current > target {
			return v.Address
		}
	}
	return validators[0].Address
}

const BlockReward = 10

// ApplyBlockReward adds base reward + fees to the validator's account.
func ApplyBlockReward(state map[string]*Account, validator string, fees uint64) {
	reward := BlockReward + fees
	if acc, ok := state[validator]; ok {
		acc.Balance += reward
	} else {
		state[validator] = &Account{Balance: reward, Nonce: 0, Staked: 0}
	}
}
