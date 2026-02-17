#!/bin/bash
set -e

# Kill any existing blockchain process
pkill -f blockchain || true

echo "Building Blockchain Node..."
go build -o blockchain

echo "Starting Node on Port 8000..."
export ADDR=8000
export P2P_PORT=6000
./blockchain
