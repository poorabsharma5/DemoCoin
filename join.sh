#!/bin/bash
# Kill any existing peer process
pkill -f "blockchain_peer" || true

# Clean previous peer state for a fresh join
rm -f blockchain_peer.db

echo "Starting Peer Node on Port 8001 (P2P 6001)..."
export ADDR=8001
export P2P_PORT=6001
export DB_FILE=blockchain_peer.db
export SINGLE_NODE=true  # Match Genesis of Node A
./blockchain
