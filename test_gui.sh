#!/bin/bash
set -e

echo "Building..."
pkill -f blockchain || true
go build -o blockchain

# Cleanup
rm -f blockchain_gui.db node_*.key
export ADDR=8000
export DB_FILE=blockchain_gui.db
export SINGLE_NODE=true

echo "Starting Node..."
./blockchain > node_gui.log 2>&1 &
PID=$!
sleep 3

# 1. Test Node Info
echo "Testing /node/info..."
ADDRESS=$(curl -s http://localhost:8000/node/info | jq -r '.address')
if [ "$ADDRESS" == "null" ] || [ -z "$ADDRESS" ]; then
    echo "FAIL: Could not seek address from /node/info"
    exit 1
fi
echo "Node Address: $ADDRESS"

# 2. Test Consensus Start
echo "Testing /consensus/start..."
RES=$(curl -s -X POST http://localhost:8000/consensus/start)
if [[ "$RES" != *"started"* ]]; then
    echo "FAIL: Failed to start consensus"
    exit 1
fi

# 3. Wait for mining (15s for 10s tick)
echo "Waiting 15s for auto-mining..."
sleep 15

# 4. Check Block Height
HEIGHT=$(curl -s http://localhost:8000/stats | jq '.height')
echo "Height: $HEIGHT"
if [ "$HEIGHT" -gt 1 ]; then
    echo "PASS: Blocks mined automatically!"
else
    echo "FAIL: No blocks mined (Height: $HEIGHT)"
    # Check logs
    grep "Consensus" node_gui.log || true
    exit 1
fi

# 4b. Check API/Blocks
echo "Testing /api/blocks..."
BLOCKS_TYPE=$(curl -s http://localhost:8000/api/blocks | jq type)
if [ "$BLOCKS_TYPE" == "\"array\"" ]; then
    echo "PASS: /api/blocks returned an array."
else
    echo "FAIL: /api/blocks did not return an array (Got $BLOCKS_TYPE)"
    exit 1
fi

# 5. Test Consensus Stop
echo "Testing /consensus/stop..."
curl -s -X POST http://localhost:8000/consensus/stop
sleep 11
# Check height again to ensure it didn't increase
HEIGHT_2=$(curl -s http://localhost:8000/stats | jq '.height')
if [ "$HEIGHT_2" -eq "$HEIGHT" ]; then
    echo "PASS: Consensus stopped correctly."
else
    echo "FAIL: Mining continued after stop."
    exit 1
fi

echo "ALL GUI BACKEND TESTS PASSED."
kill $PID
