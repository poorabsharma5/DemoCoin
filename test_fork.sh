#!/bin/bash
set -e
export DISABLE_MDNS=true

# Build
echo "Building..."
go build -o blockchain main.go state.go crypto.go merkle.go storage.go p2p.go

# Cleanup
rm -f blockchain_*.db node_*.key
pkill -f blockchain || true

# 1. Init Genesis on Node A
echo "Step 1: Init Genesis on Node A..."
export ADDR=8000
export P2P_PORT=6000
export DB_FILE=blockchain_common.db
rm -f blockchain_common.db blockchain_A.db blockchain_B.db
./blockchain > nodeA_init.log 2>&1 &
PID_A=$!
sleep 3
# Check if node is running before kill
if ps -p $PID_A > /dev/null; then
    kill $PID_A
    sleep 1
else
    echo "Node A failed to start or already exited."
    cat nodeA_init.log
    exit 1
fi

# 2. Create two parallel universes
echo "Step 2: Creating Branch A and Branch B DBs..."
cp blockchain_common.db blockchain_A.db
cp blockchain_common.db blockchain_B.db

# 3. Mine Branch A (The "Long" Chain)
echo "Step 3: Mining Branch A (Target Chain)..."
export ADDR=8000
export P2P_PORT=6000
export DB_FILE=blockchain_A.db
./blockchain > nodeA_branch.log 2>&1 &
PID_A=$!
sleep 2
echo "Mining Block 1A..."
curl -X POST http://localhost:8000/mine
sleep 2
echo "Mining Block 2A..."
curl -X POST http://localhost:8000/mine
sleep 1
kill $PID_A
sleep 1
# Branch A Height: Genesis + 1A + 2A = 3 blocks.

# 4. Mine Branch B (The "Short/Forked" Chain)
echo "Step 4: Mining Branch B..."
export ADDR=8000 # Use Node A identity to sign valid blocks!
export P2P_PORT=6000
export DB_FILE=blockchain_B.db
./blockchain > nodeB_branch.log 2>&1 &
PID_A=$!
sleep 2
echo "Mining Block 1B..."
curl -X POST http://localhost:8000/mine
sleep 2
kill $PID_A
sleep 1
# Branch B Height: Genesis + 1B = 2 blocks.

# 5. Converge
echo "Step 5: Starting Nodes for convergence..."

# Node A (Long Chain)
export ADDR=8000
export P2P_PORT=6000
export DB_FILE=blockchain_A.db
./blockchain > nodeA_final.log 2>&1 &
PID_A=$!

# Node B (Short Chain - simulates the node on Branch B)
# It uses DB B. It has identity B (8001 key).
# BUT the blocks in DB B are signed by A. That's fine.
export ADDR=8001
export P2P_PORT=6001
export DB_FILE=blockchain_B.db
./blockchain > nodeB_final.log 2>&1 &
PID_B=$!

sleep 5

# Trigger Sync: Mine Block 3A on Node A.
# This broadcasts "NEW_BLOCK". Node B receives it.
# Node B sees Block 3A (Index 3).
# Node B's tip is Block 1B (Index 1).
# Gap detected! B requests chain.
echo "Step 6: Triggering sync by mining on Node A..."
curl -X POST http://localhost:8000/mine
sleep 5

echo "Checking Node B chain height..."
HEIGHT_B=$(curl -s http://localhost:8001/ | jq 'length')
echo "Node B Height: $HEIGHT_B"

# Expected: Genesis + 1A + 2A + 3A = Height 4.
if [ "$HEIGHT_B" -eq 4 ]; then
    echo "SUCCESS: Node B switched to longer chain (Height 4)."
else
    echo "FAILURE: Node B Height is $HEIGHT_B (Expected 4)."
    echo "--- Node A Log ---"
    tail -n 20 nodeA_final.log
    echo "--- Node B Log ---"
    tail -n 20 nodeB_final.log
fi

# Cleanup
kill $PID_A $PID_B || true
