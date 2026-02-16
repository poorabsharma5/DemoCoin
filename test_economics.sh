#!/bin/bash
set -e

# Build
echo "Building..."
go build -o blockchain

# Cleanup
rm -f blockchain_econ.db node_*.key
pkill -f blockchain || true
sleep 1
export ADDR=8000
export SINGLE_NODE=true
export DB_FILE=blockchain_econ.db

# Start Node
echo "Starting Node..."
./blockchain > node_econ.log 2>&1 &
PID=$!
sleep 3

# Get Validator Address
VAL_ADDR=$(curl -s http://localhost:8000/stats | grep -o '"Address": "[^"]*"' | head -n 1 | cut -d '"' -f 4)
echo "Validator: $VAL_ADDR"

# Initial State
INIT_BAL=$(curl -s http://localhost:8000/balance/$VAL_ADDR | jq '.Balance')
INIT_SUPPLY=$(curl -s http://localhost:8000/stats | jq '.totalSupply')

echo "Initial Balance: $INIT_BAL"
echo "Initial Supply: $INIT_SUPPLY"

# Create Recipient
RECIPIENT="dest_address_placeholder"

# Send Transaction (Amount=100, Fee=1 is hardcoded in main.go)
echo "Sending Transaction..."
curl -X POST -d "{\"To\":\"$RECIPIENT\", \"Amount\":100}" http://localhost:8000/send
sleep 1

# Mine Block
echo "Mining Block..."
curl -X POST http://localhost:8000/mine
sleep 2

# Final State
FINAL_BAL=$(curl -s http://localhost:8000/balance/$VAL_ADDR | jq '.Balance')
FINAL_SUPPLY=$(curl -s http://localhost:8000/stats | jq '.totalSupply')

echo "Final Balance: $FINAL_BAL"
echo "Final Supply: $FINAL_SUPPLY"

# Verification
# 1. Supply should increase by Block Reward (10) ONLY. Fees are internal transfer.
EXPECTED_SUPPLY=$(($INIT_SUPPLY + 10))
if [ "$FINAL_SUPPLY" -eq "$EXPECTED_SUPPLY" ]; then
    echo "Supply Check: PASS ($FINAL_SUPPLY)"
else
    echo "Supply Check: FAIL (Expected $EXPECTED_SUPPLY, Got $FINAL_SUPPLY)"
    exit 1
fi

# 2. Balance Check
# Val sent 100. Val paid 1 fee. Val received 10 reward. Val received 1 fee.
# Net: -100 - 1 + 10 + 1 = -90.
EXPECTED_BAL=$(($INIT_BAL - 90))
if [ "$FINAL_BAL" -eq "$EXPECTED_BAL" ]; then
    echo "Fee/Reward Check: PASS ($FINAL_BAL)"
else
    echo "Fee/Reward Check: FAIL (Expected $EXPECTED_BAL, Got $FINAL_BAL)"
    exit 1
fi

echo "SUCCESS: Economic Rules Verified."
kill $PID
