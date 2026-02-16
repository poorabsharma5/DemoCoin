#!/bin/bash

# Kill any existing nodes
pkill -f "go run main.go" || true
rm -f blockchain_*.db node_*.key

# --- Configuration ---
# Node A (Genesis Leader)
PORT_A=8080
P2P_A=6000
DB_A="blockchain_a.db"

# Node B
PORT_B=8081
P2P_B=6001
DB_B="blockchain_b.db"

# Node C
PORT_C=8082
P2P_C=6002
DB_C="blockchain_c.db"

echo "Starting Node A (Genesis)..."
export ADDR=$PORT_A
export P2P_PORT=$P2P_A
export DB_FILE=$DB_A
go run main.go state.go crypto.go merkle.go storage.go p2p.go > node_a.log 2>&1 &
PID_A=$!
sleep 5

echo "Starting Node B..."
export ADDR=$PORT_B
export P2P_PORT=$P2P_B
export DB_FILE=$DB_B
go run main.go state.go crypto.go merkle.go storage.go p2p.go > node_b.log 2>&1 &
PID_B=$!
sleep 5

echo "Starting Node C..."
export ADDR=$PORT_C
export P2P_PORT=$P2P_C
export DB_FILE=$DB_C
go run main.go state.go crypto.go merkle.go storage.go p2p.go > node_c.log 2>&1 &
PID_C=$!
sleep 5

echo "Keys generated. Node Address setup complete."
# We need to know addresses to send funds.
# They are printed in logs "Node Identity: ..."
# Let's extract them.

ADDR_A=$(grep "Node Identity:" node_a.log | head -1 | awk '{print $3}')
ADDR_B=$(grep "Node Identity:" node_b.log | head -1 | awk '{print $3}')
ADDR_C=$(grep "Node Identity:" node_c.log | head -1 | awk '{print $3}')

echo "Node A: $ADDR_A"
echo "Node B: $ADDR_B"
echo "Node C: $ADDR_C"

if [ -z "$ADDR_A" ] || [ -z "$ADDR_B" ] || [ -z "$ADDR_C" ]; then
    echo "Failed to get addresses. Check logs."
    kill $PID_A $PID_B $PID_C
    exit 1
fi

# --- Funding ---
echo "Funding Node B (70 coins)..."
curl -s -X POST -H "Content-Type: application/json" -d "{\"To\":\"$ADDR_B\", \"Amount\":70}" http://localhost:$PORT_A/send > /dev/null
sleep 2

echo "Funding Node C (20 coins)..."
curl -s -X POST -H "Content-Type: application/json" -d "{\"To\":\"$ADDR_C\", \"Amount\":20}" http://localhost:$PORT_A/send > /dev/null
sleep 2

# Mine block to confirm transfers (A is currently only miner/validator since stakes are 0? No, genesis stake is 0? 
# Wait, Genesis account in `main.go` has `Staked: 0`.
# If `GetValidators` returns empty, `SelectProposer` returns "".
# `generateBlock` checks `len(validators) > 0`.
# If 0, it allows mining (bootstrap).
# So A can mine.
echo "Mining block to confirm transfers..."
curl -s -X POST http://localhost:$PORT_A/mine > /dev/null
sleep 2

# --- Staking ---
echo "Node A Staking 100..."
curl -s -X POST -H "Content-Type: application/json" -d '{"Amount":100}' http://localhost:$PORT_A/stake > /dev/null
sleep 1

echo "Node B Staking 50..."
curl -s -X POST -H "Content-Type: application/json" -d '{"Amount":50}' http://localhost:$PORT_B/stake > /dev/null
sleep 1

echo "Node C Staking 10..."
curl -s -X POST -H "Content-Type: application/json" -d '{"Amount":10}' http://localhost:$PORT_C/stake > /dev/null
sleep 1

echo "Mining block to confirm stakes..."
# Still bootstrapping logic or A is the only one with funds/stake?
# Actually, stakes are in mempool. A mines them.
curl -s -X POST http://localhost:$PORT_A/mine > /dev/null
sleep 5

# Now Validator set should be updated.
# A: 100, B: 50, C: 10. Total 160.
# Frequencies: A~62%, B~31%, C~6%.

echo "--- Starting Mining Simulation (Press Ctrl+C to stop) ---"
echo "Observing proposer selection..."

for i in {1..20}; do
    echo "Round $i:"
    # Try all nodes. Only correct proposer will succeed.
    # We suppress error output (400 Bad Request if not turn)
    RES_A=$(curl -s -X POST http://localhost:$PORT_A/mine)
    RES_B=$(curl -s -X POST http://localhost:$PORT_B/mine)
    RES_C=$(curl -s -X POST http://localhost:$PORT_C/mine)
    
    # Check who mined
    if [[ $RES_A == *"Index"* ]]; then echo "  Node A mined block!"; fi
    if [[ $RES_B == *"Index"* ]]; then echo "  Node B mined block!"; fi
    if [[ $RES_C == *"Index"* ]]; then echo "  Node C mined block!"; fi
    
    sleep 2
done

echo "Simulation complete. Killing nodes..."
kill $PID_A $PID_B $PID_C
