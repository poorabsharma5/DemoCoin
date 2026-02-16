package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

type Transaction struct {
	Type      string // "TX" or "STAKE"
	PublicKey []byte
	Signature []byte
	To        string
	Amount    uint64
	Fee       uint64
	Nonce     uint64
	Data      string
}

type Block struct {
	Index              int
	Timestamp          string
	Transactions       []Transaction
	Validator          string // Address
	ValidatorPublicKey []byte // Added for signature verification
	Signature          []byte
	MerkleRoot         string
	StateRoot          string
	Hash               string
	PrevHash           string
}

var Blockchain []Block
var Mempool []Transaction
var P2P *P2PNode

// Node Identity
var NodePrivateKey ed25519.PrivateKey
var NodePublicKey ed25519.PublicKey

// Consensus Loop
var miningTicker *time.Ticker
var miningStop chan bool
var NodeAddress string

func loadNodeKey() error {
	port := os.Getenv("ADDR")
	if port == "" {
		port = "8080"
	}

	// Simulation Mode: Deterministic Keys for 8000/8001
	if port == "8000" || port == "8001" {
		seed := sha256.Sum256([]byte("NODE_SEED_" + port))
		NodePrivateKey = ed25519.NewKeyFromSeed(seed[:])
		NodePublicKey = NodePrivateKey.Public().(ed25519.PublicKey)
		NodeAddress = PubKeyToAddress(NodePublicKey)
		return nil
	}

	filename := "node_" + port + ".key" // Unique key file per port
	// Try to read
	data, err := os.ReadFile(filename)
	if err == nil {
		if len(data) != 64 { // Ed25519 private key is 64 bytes (32 seed + 32 pub)
			return fmt.Errorf("invalid key file size")
		}
		NodePrivateKey = ed25519.PrivateKey(data)
		NodePublicKey = NodePrivateKey.Public().(ed25519.PublicKey)
		NodeAddress = PubKeyToAddress(NodePublicKey)
		return nil
	}

	// Generate new
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		return err
	}
	NodePrivateKey = priv
	NodePublicKey = pub
	NodeAddress = PubKeyToAddress(pub)

	// Save
	return os.WriteFile(filename, priv, 0600)
}

func calculateTransactionHash(tx Transaction) string {
	// Hash everything EXCEPT the signature to create the component that is signed.
	record := hex.EncodeToString(tx.PublicKey) + tx.To + fmt.Sprintf("%d", tx.Amount) + fmt.Sprintf("%d", tx.Fee) + fmt.Sprintf("%d", tx.Nonce) + tx.Data
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func calculateMerkleRoot(transactions []Transaction) string {
	return CalculateMerkleRoot(transactions)
}

func calculateHash(block Block) string {
	record := fmt.Sprintf("%d", block.Index) + block.Timestamp + block.MerkleRoot + block.StateRoot + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func calculateContentHash(block Block) string {
	record := fmt.Sprintf("%d", block.Index) + block.Timestamp + block.Validator + block.MerkleRoot + block.StateRoot + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block) (Block, error) {
	var newBlock Block
	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Validator = NodeAddress
	newBlock.ValidatorPublicKey = NodePublicKey

	// PoS: Proposer Selection
	validators := GetValidators(State)
	proposer := SelectProposer(validators, oldBlock.Hash)

	if proposer != NodeAddress {
		return Block{}, fmt.Errorf("not my turn! proposer is %s", proposer)
	}

	fmt.Printf("I am the proposer! Generating block...\n")

	// Create a temporary state copy to simulate block execution
	tempState := CloneState(State)
	var validTransactions []Transaction

	// Deterministic ordering: Mempool is already ordered by arrival.
	for _, tx := range Mempool {
		if err := ApplyTransaction(tempState, tx); err == nil {
			validTransactions = append(validTransactions, tx)
		} else {
			fmt.Println("Skipping invalid transaction:", err)
		}
		if len(validTransactions) >= 10 {
			break
		}
	}

	// Block Reward
	fees := uint64(0)
	for _, tx := range validTransactions {
		fees += tx.Fee
	}
	ApplyBlockReward(tempState, NodeAddress, fees)

	newBlock.Transactions = validTransactions
	newBlock.MerkleRoot = CalculateMerkleRoot(newBlock.Transactions)
	newBlock.StateRoot = ComputeStateRoot(tempState)

	// Signing
	contentHash := calculateContentHash(newBlock)
	newBlock.Signature = SignData(NodePrivateKey, []byte(contentHash))

	newBlock.Hash = calculateHash(newBlock)

	return newBlock, nil
}

func isBlockValid(newBlock, oldBlock Block, currentState map[string]*Account) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		fmt.Println("Invalid Block Hash")
		return false
	}

	if CalculateMerkleRoot(newBlock.Transactions) != newBlock.MerkleRoot {
		fmt.Println("Invalid Merkle Root")
		return false
	}

	// PoS Validation
	validators := GetValidators(currentState)
	if len(validators) > 0 {
		expectedProposer := SelectProposer(validators, oldBlock.Hash)
		if newBlock.Validator != expectedProposer {
			fmt.Printf("Invalid Proposer: expected %s, got %s\n", expectedProposer, newBlock.Validator)
			return false
		}
	}

	// Verify Signature
	// We need to verify that newBlock.Signature signs calculateContentHash(newBlock)
	// Key: newBlock.ValidatorPublicKey
	// Check if ValidatorPublicKey matches Validator Address
	if PubKeyToAddress(newBlock.ValidatorPublicKey) != newBlock.Validator {
		fmt.Println("Validator public key does not match address")
		return false
	}

	contentHash := calculateContentHash(newBlock)
	if !VerifySignature(newBlock.ValidatorPublicKey, []byte(contentHash), newBlock.Signature) {
		fmt.Println("Invalid Block Signature")
		return false
	}

	// Atomic Execution Verification
	checkState := CloneState(currentState)
	for _, tx := range newBlock.Transactions {
		if err := ApplyTransaction(checkState, tx); err != nil {
			fmt.Println("Block validation failed: invalid tx:", err)
			return false
		}
	}

	// Apply Reward to Validator logic for checking StateRoot
	fees := uint64(0)
	for _, tx := range newBlock.Transactions {
		fees += tx.Fee
	}
	ApplyBlockReward(checkState, newBlock.Validator, fees)

	if ComputeStateRoot(checkState) != newBlock.StateRoot {
		fmt.Printf("Block validation failed: State Root mismatch. Have %s, want %s\n", ComputeStateRoot(checkState), newBlock.StateRoot)
		return false
	}

	return true
}

func run() error {
	mux := makeMuxRouter()
	httpAddr := os.Getenv("ADDR")
	if httpAddr == "" {
		httpAddr = "8080"
	}
	log.Println("Listening on ", httpAddr)
	s := &http.Server{
		Addr:           ":" + httpAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func makeMuxRouter() http.Handler {
	muxRouter := mux.NewRouter()
	muxRouter.HandleFunc("/api/blocks", handleGetBlockchain).Methods("GET")
	muxRouter.HandleFunc("/transaction", handleAddTransaction).Methods("POST")
	muxRouter.HandleFunc("/send", handleSend).Methods("POST")
	muxRouter.HandleFunc("/stake", handleStake).Methods("POST")
	muxRouter.HandleFunc("/mine", handleMineBlock).Methods("POST")
	muxRouter.HandleFunc("/mempool", handleGetMempool).Methods("GET")
	muxRouter.HandleFunc("/stats", handleGetStats).Methods("GET")
	muxRouter.HandleFunc("/testing", handleDashboard).Methods("GET")
	muxRouter.HandleFunc("/connect", handleConnectPeer).Methods("POST")
	muxRouter.HandleFunc("/balance/{address}", handleGetBalance).Methods("GET")
	muxRouter.HandleFunc("/consensus/start", handleStartConsensus).Methods("POST")
	muxRouter.HandleFunc("/consensus/stop", handleStopConsensus).Methods("POST")
	muxRouter.HandleFunc("/node/info", handleNodeInfo).Methods("GET")
	muxRouter.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/")))
	return muxRouter
}

func handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, r, http.StatusOK, map[string]string{
		"address":   NodeAddress,
		"publicKey": hex.EncodeToString(NodePublicKey),
	})
}

func startConsensus() {
	if miningTicker != nil {
		return // Already running
	}
	miningTicker = time.NewTicker(5 * time.Second)
	miningStop = make(chan bool)

	go func() {
		for {
			select {
			case <-miningStop:
				return
			case <-miningTicker.C:
				if len(Blockchain) == 0 {
					continue
				}
				lastBlock := Blockchain[len(Blockchain)-1]
				validators := GetValidators(State)
				if len(validators) == 0 {
					continue
				}
				expectedProposer := SelectProposer(validators, lastBlock.Hash)

				if expectedProposer == NodeAddress {
					log.Println("Consensus: It's my turn to mine!")
					// Trigger mining
					// We can reuse handleMineBlock logic but internally?
					// Or just call generateBlock and broadcast.
					// Let's call internal logic to keep consistent.
					newBlock, err := generateBlock(lastBlock)
					if err != nil {
						log.Printf("Consensus Error generating block: %v", err)
						continue
					}
					// Verify immediately (sanity check)
					if ok := isBlockValid(newBlock, lastBlock, State); !ok {
						log.Printf("Consensus: Generated block %d INVALID. StateRoot: %s", newBlock.Index, newBlock.StateRoot)
					} else {
						Blockchain = append(Blockchain, newBlock)
						for _, tx := range newBlock.Transactions {
							ApplyTransaction(State, tx)
						}
						fees := uint64(0)
						for _, tx := range newBlock.Transactions {
							fees += tx.Fee
						}
						ApplyBlockReward(State, newBlock.Validator, fees)
						SaveBlock(newBlock)
						SaveState(State)
						if P2P != nil {
							go P2P.Broadcast(MsgNewBlock, newBlock)
						}
						Mempool = []Transaction{}
						log.Printf("Consensus: Mined Block %d", newBlock.Index)
					}
				} else {
					// log.Printf("Consensus: Not my turn. Expected %s", expectedProposer)
				}
			}
		}
	}()
}

func handleStartConsensus(w http.ResponseWriter, r *http.Request) {
	startConsensus()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Consensus loop started"))
}

func handleStopConsensus(w http.ResponseWriter, r *http.Request) {
	if miningTicker != nil {
		miningTicker.Stop()
		miningTicker = nil
		miningStop <- true
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Consensus loop stopped"))
}

func handleGetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	if acc, ok := State[address]; ok {
		respondWithJSON(w, r, http.StatusOK, acc)
	} else {
		respondWithJSON(w, r, http.StatusNotFound, map[string]string{"error": "Account not found"})
	}
}

func handleGetStats(w http.ResponseWriter, r *http.Request) {
	totalSupply := uint64(0)
	for _, acc := range State {
		totalSupply += acc.Balance + acc.Staked
	}

	peers := 0
	if P2P != nil {
		peers = len(P2P.Peers)
	}

	stats := map[string]interface{}{
		"height":       len(Blockchain),
		"totalSupply":  totalSupply,
		"validators":   GetValidators(State),
		"mempoolCount": len(Mempool),
		"peers":        peers,
	}
	respondWithJSON(w, r, http.StatusOK, stats)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "dashboard.html")
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		To     string
		Amount uint64
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create Tx
	nonce := GetNonce(NodeAddress)
	// Check mempool for pending nonces?
	// Simple approach: GetNonce gets committed state. If mempool has txs, we might reuse nonce.
	// Better: `nonce = GetNonce(NodeAddress) + 1 + countInMempool(NodeAddress)`
	for _, tx := range Mempool {
		if PubKeyToAddress(tx.PublicKey) == NodeAddress {
			nonce++
		}
	}

	tx := Transaction{
		Type:      "TX",
		PublicKey: []byte(NodePublicKey),
		To:        payload.To,
		Amount:    payload.Amount,
		Fee:       1,
		Nonce:     nonce,
		Data:      "",
	}

	hash := calculateTransactionHash(tx)
	tx.Signature = SignData(NodePrivateKey, []byte(hash))

	Mempool = append(Mempool, tx)
	if P2P != nil {
		go P2P.Broadcast(MsgNewTx, tx)
	}

	respondWithJSON(w, r, http.StatusOK, tx)
}

func handleStake(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Amount uint64
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	nonce := GetNonce(NodeAddress)
	for _, tx := range Mempool {
		if PubKeyToAddress(tx.PublicKey) == NodeAddress {
			nonce++
		}
	}

	tx := Transaction{
		Type:      "STAKE",
		PublicKey: []byte(NodePublicKey),
		To:        "", // Stake doesn't go to anyone
		Amount:    payload.Amount,
		Fee:       1,
		Nonce:     nonce,
		Data:      "staking",
	}

	hash := calculateTransactionHash(tx)
	tx.Signature = SignData(NodePrivateKey, []byte(hash))

	Mempool = append(Mempool, tx)
	if P2P != nil {
		go P2P.Broadcast(MsgNewTx, tx)
	}

	respondWithJSON(w, r, http.StatusOK, tx)
}

func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Blockchain, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func handleAddTransaction(w http.ResponseWriter, r *http.Request) {
	var tx Transaction

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&tx); err != nil {
		respondWithJSON(w, r, http.StatusBadRequest, map[string]string{"error": "Invalid transaction format"})
		return
	}
	defer r.Body.Close()

	// Add transaction to mempool
	Mempool = append(Mempool, tx)

	// Broadcast NEW_TX
	if P2P != nil {
		go P2P.Broadcast(MsgNewTx, tx)
	}

	respondWithJSON(w, r, http.StatusCreated, map[string]string{
		"message": "Transaction added to mempool",
		"from":    PubKeyToAddress(tx.PublicKey),
		"to":      tx.To,
		"amount":  fmt.Sprintf("%d", tx.Amount),
	})
}

func handleMineBlock(w http.ResponseWriter, r *http.Request) {
	if len(Blockchain) == 0 {
		respondWithJSON(w, r, http.StatusBadRequest, map[string]string{"error": "Blockchain not initialized"})
		return
	}

	newBlock, err := generateBlock(Blockchain[len(Blockchain)-1])
	if err != nil {
		respondWithJSON(w, r, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if isBlockValid(newBlock, Blockchain[len(Blockchain)-1], State) {
		Blockchain = append(Blockchain, newBlock)
		spew.Dump(Blockchain)

		// Commit state changes
		for _, tx := range newBlock.Transactions {
			err := ApplyTransaction(State, tx)
			if err != nil {
				log.Printf("CRITICAL: Failed to apply valid transaction to state: %v", err)
			}
		}
		// Apply Block Reward
		fees := uint64(0)
		for _, tx := range newBlock.Transactions {
			fees += tx.Fee
		}
		ApplyBlockReward(State, newBlock.Validator, fees)

		// Persistence
		if err := SaveBlock(newBlock); err != nil {
			log.Printf("ERROR: Failed to save block to DB: %v", err)
		}
		if err := SaveState(State); err != nil {
			log.Printf("ERROR: Failed to save state to DB: %v", err)
		}

		// Broadcast NEW_BLOCK
		if P2P != nil {
			go P2P.Broadcast(MsgNewBlock, newBlock)
		}

		// Clear Mempool
		Mempool = []Transaction{}
	}

	respondWithJSON(w, r, http.StatusCreated, newBlock)
}

type ConnectRequest struct {
	Peer string `json:"peer"`
}

func handleConnectPeer(w http.ResponseWriter, r *http.Request) {
	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if P2P == nil {
		http.Error(w, "P2P not started", http.StatusBadRequest)
		return
	}

	if err := P2P.ConnectToPeer(req.Peer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Connected"))
}

func handleGetMempool(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Mempool, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.WriteHeader(code)
	w.Write(response)
}

// P2P Handlers

func handleP2PMessages() {
	for {
		select {
		case tx := <-P2P.TxChan:
			// Basic duplication check
			found := false
			for _, t := range Mempool {
				if calculateTransactionHash(t) == calculateTransactionHash(tx) {
					found = true
					break
				}
			}
			if !found {
				log.Printf("Received new tx from P2P: Amount %d", tx.Amount)
				Mempool = append(Mempool, tx)
			}

		case blockData := <-P2P.BlockChan:
			block := blockData.Block
			sender := blockData.From
			log.Printf("Received new block from P2P: Index %d", block.Index)

			if len(Blockchain) == 0 {
				continue
			}
			lastBlock := Blockchain[len(Blockchain)-1]

			// Case 1: Next block in sequence
			if block.Index == lastBlock.Index+1 {
				if isBlockValid(block, lastBlock, State) {
					Blockchain = append(Blockchain, block)

					// Update State
					for _, tx := range block.Transactions {
						ApplyTransaction(State, tx)
					}
					// Apply Reward
					fees := uint64(0)
					for _, tx := range block.Transactions {
						fees += tx.Fee
					}
					ApplyBlockReward(State, block.Validator, fees)

					// Persist
					SaveBlock(block)
					SaveState(State)

					// Clear mempool
					Mempool = []Transaction{}
					log.Println("Appended new block from P2P")
				} else {
					log.Println("Received invalid block from P2P")
				}
			} else if block.Index > lastBlock.Index {
				// Case 2: Block is ahead -> Possible Fork or we are behind.
				log.Printf("Detected higher block (My Height: %d, Peer Height: %d). Requesting full chain from %s...", len(Blockchain), block.Index+1, sender)
				P2P.SendMessageToPeer(sender, MsgGetBlocks, "")
			}

		case sender := <-P2P.GetBlocksChan:
			log.Printf("Peer %s requested blocks. Sending chain...", sender)
			P2P.SendMessageToPeer(sender, MsgChain, Blockchain)

		case chainData := <-P2P.ChainChan:
			newChain := chainData.Chain
			sender := chainData.From
			log.Printf("Received chain from %s (Length: %d)", sender, len(newChain))

			if len(newChain) > len(Blockchain) {
				log.Println("Received longer chain. Verifying...")
				if verifyChain(newChain) {
					log.Println("Longer chain is valid. Switching...")
					replaceChain(newChain)
				} else {
					log.Println("Received longer chain but it is INVALID.")
				}
			} else {
				log.Println("Received chain is not longer. Ignoring.")
			}
		}
	}
}

func verifyChain(chain []Block) bool {
	// 1. Check Genesis (Optional: match our genesis? strictly yes for this test)
	// But simply checking consistency is enough for now.
	if len(chain) == 0 {
		return false
	}

	// 2. Validate links
	// Clone state to verify from scratch
	// Ideally we load Genesis state.
	// For now, let's assume Genesis (Index 0) is valid if it matches ours, OR just trust 0 is valid and verify transitions.
	// Better: Validate from Block 1 using Block 0 as base.

	// Helper to reconstruct state from scratch

	// Seed genesis state (Hardcoded for now / same as main init)
	// actually we should load genesis from chain[0].
	// For simulation, let's assume we start from a clean slate + Genesis transactions?
	// But our Genesis has STATE populated manually in `main.go`.
	// We need to replicate that.
	// Or, just verify `isBlockValid` for i=1..N.

	// Hack: Re-initialize specific genesis state for verification?
	// Real impl: Store Genesis Block in DB.

	// Fast verification:
	// To verify properly, we need to roll the state forward
	tempState := make(map[string]*Account)
	// Seed with Genesis from chain[0]
	gen := chain[0]
	tempState[gen.Validator] = &Account{Balance: 1000000, Nonce: 0, Staked: 100}

	for i := 1; i < len(chain); i++ {
		if !isBlockValid(chain[i], chain[i-1], tempState) {
			log.Printf("Chain verification failed at index %d", i)
			return false
		}
		// Roll state forward for next iteration
		for _, tx := range chain[i].Transactions {
			ApplyTransaction(tempState, tx)
		}
		fees := uint64(0)
		for _, tx := range chain[i].Transactions {
			fees += tx.Fee
		}
		ApplyBlockReward(tempState, chain[i].Validator, fees)
	}
	return true
}

func replaceChain(newBlocks []Block) {
	log.Println("Replacing chain...")
	Blockchain = newBlocks

	// Rebuild State
	newState := make(map[string]*Account)

	// Seed Genesis matching main() logic
	// We use the Validator of the genesis block
	genesisBlock := newBlocks[0]
	genesisAddr := genesisBlock.Validator

	newState[genesisAddr] = &Account{Balance: 1000000, Nonce: 0, Staked: 100}

	// If there were other addresses in genesis simulation, we'd add them here.
	// But in our current setup with SINGLE_NODE=true, only A exists with stake.

	for i, block := range newBlocks {
		// Apply transactions from this block
		for _, tx := range block.Transactions {
			if err := ApplyTransaction(newState, tx); err != nil {
				log.Printf("Error replaying tx in replaceChain: %v", err)
			}
		}

		// Apply Block Reward
		if i > 0 { // Skip Genesis reward (pre-funded)
			fees := uint64(0)
			for _, tx := range block.Transactions {
				fees += tx.Fee
			}
			ApplyBlockReward(newState, block.Validator, fees)
		}

		// Persist each block to DB
		SaveBlock(block)
	}

	State = newState
	SaveState(State)
	log.Printf("Chain replaced. New Height: %d, StateRoot: %s", len(Blockchain), ComputeStateRoot(State))
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	dbFile := os.Getenv("DB_FILE")
	if dbFile == "" {
		dbFile = "blockchain.db"
	}

	// Load Node Identity
	if err := loadNodeKey(); err != nil {
		log.Fatal("Failed to load node key:", err)
	}
	fmt.Printf("Node Identity: %s\n", NodeAddress)

	// Initialize DB
	if err := InitDB(dbFile); err != nil {
		log.Fatal("Failed to init DB:", err)
	}
	defer CloseDB()

	// Load Blockchain and State
	Blockchain, err = LoadBlockchain()
	if err != nil {
		log.Fatal("Failed to load blockchain:", err)
	}

	// If empty, init genesis
	if len(Blockchain) == 0 {
		go func() {
			t := time.Now()

			// Simulation: Hardcode stakes for Node A (8000) and Node B (8001)
			genKey := func(portStr string) (string, ed25519.PublicKey) {
				seed := sha256.Sum256([]byte("NODE_SEED_" + portStr))
				priv := ed25519.NewKeyFromSeed(seed[:])
				pub := priv.Public().(ed25519.PublicKey)
				return PubKeyToAddress(pub), pub
			}

			addrA, _ := genKey("8000")
			addrB, _ := genKey("8001")

			State[addrA] = &Account{Balance: 1000000, Nonce: 0, Staked: 100}
			if os.Getenv("SINGLE_NODE") != "true" {
				State[addrB] = &Account{Balance: 1000000, Nonce: 0, Staked: 100}
			}

			genesisStateRoot := ComputeStateRoot(State)

			// Genesis Validator is Node A (8000)
			// But note: if running as B, NodePublicKey will be B's key.
			// But Genesis block signature MUST be valid for Validator field.
			// So if Validator is A, Signature must be signed by A.
			// If running as B, we can't sign as A!
			// WE MUST SIGN AS OURSELVES if we create Genesis.
			// BUT Genesis needs to be SAME for both.
			// If B creates Genesis, it signs with B.
			// If A creates Genesis, it signs with A.
			// RESULT: Different Hash!
			// FIX: Only Node A creates Genesis (in test_fork.sh).
			// Node B just loads it.
			// So Validator field should be A.
			// And A signs it.

			genesisBlock := Block{
				Index:              0,
				Timestamp:          t.String(),
				Transactions:       []Transaction{},
				Validator:          addrA,
				ValidatorPublicKey: NodePublicKey, // If running as A (8000), this is A's key.
				MerkleRoot:         "",
				StateRoot:          genesisStateRoot,
				Hash:               "",
				PrevHash:           "",
			}
			genesisBlock.Hash = calculateHash(genesisBlock)
			spew.Dump(genesisBlock)
			Blockchain = append(Blockchain, genesisBlock)

			// Save Genesis
			SaveBlock(genesisBlock)
			SaveState(State)

			fmt.Printf("Genesis state initialized with dual simulation stake.\n")
			fmt.Printf("Address A: %s\n", addrA)
			fmt.Printf("Address B: %s\n", addrB)
			fmt.Printf("Private Key (local): %s\n", hex.EncodeToString(NodePrivateKey))
			fmt.Printf("Public Key (local): %s\n", hex.EncodeToString(NodePublicKey))
		}()
	} else {
		loadedState, err := LoadState()
		if err != nil {
			log.Fatal("Failed to load state:", err)
		}
		State = loadedState
		log.Println("Blockchain and State loaded from DB.")
		log.Printf("Height: %d\n", len(Blockchain))
	}

	// Start P2P Node
	p2pPortStr := os.Getenv("P2P_PORT")
	if p2pPortStr != "" {
		p2pPort, _ := strconv.Atoi(p2pPortStr)
		P2P, err = StartP2PNode(p2pPort)
		if err != nil {
			log.Fatal("Failed to start P2P node:", err)
		}
		go handleP2PMessages()
	}

	log.Fatal(run())

}
