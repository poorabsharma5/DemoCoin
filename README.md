# Crypto Project

A research blockchain implementation in Go, featuring:
- PoW and PoS consensus mechanisms.
- P2P networking with `libp2p`.
- Persistent storage with `bbolt`.
- REST API.

## Prerequisites (macOS)

1.  **Install Go**:
    If you haven't installed Go, use Homebrew:
    ```bash
    brew install go
    ```
    Verify installation:
    ```bash
    go version
    ```

2.  **Clone the Repository**:
    ```bash
    git clone <repository-url>
    cd <repository-folder>
    ```

## Installation

Install the project dependencies using Go modules:

```bash
go mod tidy
```

This will automatically download and install all required packages listed in `go.mod`.

### Manual Dependency Installation

If you prefer installing dependencies manually:

```bash
# HTTP Router
go get github.com/gorilla/mux

# Environment Variables
go get github.com/joho/godotenv

# Persistent DB
go get go.etcd.io/bbolt

# P2P Networking
go get github.com/libp2p/go-libp2p

# Debugging / Pretty Print
go get github.com/davecgh/go-spew/spew
```

## Running the Project

To run a single node:

```bash
go run main.go state.go crypto.go merkle.go storage.go p2p.go
```

To run the Proof of Stake simulation (3 nodes):

```bash
./run_pos_simulation.sh
```
