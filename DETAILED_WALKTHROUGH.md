# 🌐 The Ultimate DemoCoin Blockchain Walkthrough

Welcome to **DemoCoin**, a fully functional, educational blockchain implementation written in Go. This project demonstrates the core principles of decentralized ledgers, from cryptographic identity to peer-to-peer networking and consensus algorithms.

This guide is designed to be **extremely detailed**, breaking down every single file, feature, and line of logic so you can understand exactly how a blockchain operates under the hood.

---

## 🌟 Key Features

*   **Proof of Stake (PoS)**: Energy-efficient consensus where validators are chosen based on their stake (coins deposited) rather than computational power.
*   **P2P Networking**: Nodes automatically discover each other over LAN using mDNS and sync their blockchains instantly.
*   **Cryptographic Security**: Uses **Ed25519** for digital signatures, ensuring only the owner of a wallet can spend their funds.
*   **State Management**: Tracks account balances and nonces (to prevent replay attacks) using a state machine approach.
*   **Persistence**: Saves the blockchain and state to a local `bbolt` database (`blockchain.db`), so data survives restarts.
*   **Web Dashboard**: A clean, modern UI for sending coins, viewing blocks, and managing staking.
*   **Mining Center**: A dedicated UI to enable/disable validation and stake coins.

---

## 🛠️ Requirements & Setup

Before you start, ensure your environment is ready.

### Prerequisites
1.  **Go (Golang)**: Version 1.22 or higher. [Download here](https://go.dev/dl/).
2.  **Terminal**: Any standard terminal (Terminal.app, iTerm2, VSCode Terminal).

### Installation
1.  **Clone/Download** the project folder.
2.  **Install Dependencies**: Run this command to fetch all required Go libraries (libp2p, bbolt, etc.):
    ```bash
    go mod tidy
    ```

---

## 📂 Deep Dive: The Codebase

Here is a detailed breakdown of every file in the project and its specific role.

### 1. `main.go` - The Core Engine
This is the entry point of the application. It ties everything together: networking, database, web server, and consensus.

*   **`main()`**: The captain of the ship.
    *   Loads your identity (private key) from `node.key`.
    *   Initializes the database connection.
    *   Starts the P2P node listener.
    *   Starts the HTTP API server on port 8000 (or `ADDR` env var).
*   **`Consensus Loop (startConsensus)`**:
    *   Runs every 5 seconds.
    *   Checks if you are a **Validator** (have staked coins).
    *   Calculates if it is *your turn* to propose the next block using `SelectProposer`.
    *   If selected, it calls `handleMineBlock` to create, sign, and broadcast a new block.
*   **`isBlockValid()`**: The security checkpoint.
    *   Ensures the block is correctly linked to the previous one (Hash pointers).
    *   Verifies the validator's signature (Proof of Authenticity).
    *   Re-executes every transaction to ensure no rules were broken (e.g., spending more than you have).
    *   Verifies the **State Root** matches the result of the transactions.

### 2. `state.go` - The Ledger (Account Book)
Blockchains are state machines. This file tracks the current state of the world (who owns what).

*   **`Account` Struct**: 
    *   `Balance`: How many DemoCoins you have locally available.
    *   `Staked`: How many coins you have locked up to become a validator.
    *   `Nonce`: A counter that increments with every transaction. This prevents someone from sending the same "Pay 10 coins" transaction twice.
*   **`ApplyTransaction()`**:
    *   Deducts balance from Sender.
    *   Adds balance to Receiver.
    *   Or, if `Type == "STAKE"`, moves coins from `Balance` to `Staked`.
    *   **CRITICAL**: If any check fails (e.g., low balance, wrong signature), the entire transaction is rejected.
*   **`ComputeStateRoot()`**:
    *   Takes the entire `State` map and hashes it into a single string. This "fingerprint" is included in every block, so all nodes agree on the exact account balances at that point in time.

### 3. `p2p.go` - The Networking Layer
This file allows nodes to talk to each other without a central server.

*   **`StartP2PNode()`**:
    *   Opens a listening port (default 6000) for incoming connections.
    *   Uses **Noise** encryption to secure communication.
*   **`setupDiscovery()` / mDNS**:
    *   Broadcasting "I am here" on your local WiFi network.
    *   When a new peer is found, it automatically connects and exchanges messages.
*   **Message Types**:
    *   `NEW_TX`: "I just sent money, add this to your mempool."
    *   `NEW_BLOCK`: "I just found a block! Add it to your chain."
    *   `GET_BLOCKS`: "I am new here, please send me the whole blockchain history."
    *   `CHAIN`: The response containing the full list of blocks.

### 4. `crypto.go` - Cryptography & Identity
Handles the math that makes the blockchain secure.

*   **`GenerateKeyPair()`**: Creates an Ed25519 Private/Public key pair.
    *   **Private Key**: Your secret password. Never shared. Used to sign transactions.
    *   **Public Key**: Your identity/wallet address. Shared with everyone.
*   **`SignData()` / `VerifySignature()`**:
    *   Ensures that only the owner of the private key can authorized a transaction.
    *   If you change even one byte of the transaction data, the signature becomes invalid.

### 5. `merkle.go` - Data Integrity
*   **`CalculateMerkleRoot()`**:
    *   Takes all transactions in a block and hashes them together in a tree structure.
    *   Produces a single "Root Hash".
    *   If a hacker tries to change a transaction inside an old block, the Merkle Root changes, which changes the Block Hash, which breaks the chain.

### 6. `storage.go` - Database Persistence
*   **`InitDB()`**: Opens `blockchain.db` using **bbolt** (a fast key-value store).
*   **`SaveBlock()`**: Writes every confirmed block to disk.
*   **`SaveState()`**: Snapshots the current account balances to disk.
*   **`LoadBlockchain()`**: Reads the history from disk when you restart the app.

---

## 💻 Running the Project

### 1. Starting the Main Node
Run the initialization script. This creates your identity, database, and starts the server.
```bash
./run.sh
```
*   **API**: `http://localhost:8000`
*   **P2P Port**: `6000`

### 2. Using the Dashboard
Open your browser to `http://localhost:8000`.
*   **Home Tab**: View your Address, Balance, and recent Blocks.
*   **Mining Tab**:
    1.  **Stake Coins**: Enter "10" and click Stake.
    2.  **Enable Mining**: Click the green button.
    3.  Wait! Every 5 seconds, if you are selected, you will mine a block and earn rewards.

### 3. Adding a Peer (P2P Demo)
To simulate a second person joining the network on the same computer:
```bash
./join.sh
```
*   **API**: `http://localhost:8001`
*   **P2P Port**: `6001`
*   **Syncing**: Notice that as soon as it starts, it downloads all blocks from Node A and shows the exact same height.

---

## 📊 Developer Tools & API

We have included several tools for debugging and demonstration.

### Admin Stats API
`GET /stats`
Returns a JSON summary of the node's internal state.
*   **Example Response**:
    ```json
    {
      "height": 42,
      "peers": ["12D3Koo..."],
      "mempoolCount": 0,
      "totalSupply": 1000420,
      "validators": [
        { "Address": "e540...", "Stake": 100 }
      ]
    }
    ```

### Testing Dashboard
While `index.html` is the user-facing wallet, you can view raw JSON data by visiting:
*   `/api/blocks`: Full list of blocks.
*   `/node/info`: Your public key and address.
*   `/mempool`: List of pending transactions.

---

## 🔮 Future Improvements?
If you wanted to take this further, here is what you could add:
1.  **Merkle Proofs**: Allow light clients to verify transactions without downloading the full chain.
2.  **Smart Contracts**: Add a virtual machine (like EVM) to `ApplyTransaction` to run code.
3.  **Global Discovery**: Use a DHT (Distributed Hash Table) so nodes can find each other across the internet, not just LAN.

Enjoy building with DemoCoin! 🚀
