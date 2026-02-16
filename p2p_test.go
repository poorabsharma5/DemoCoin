package main

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestP2PBroadcast(t *testing.T) {
	// Start Node A
	nodeA, err := StartP2PNode(0) // 0 lets OS choose port
	if err != nil {
		t.Fatalf("Failed to start Node A: %v", err)
	}
	defer nodeA.Host.Close()

	// Start Node B
	nodeB, err := StartP2PNode(0)
	if err != nil {
		t.Fatalf("Failed to start Node B: %v", err)
	}
	defer nodeB.Host.Close()

	// Connect A to B manually (skip MDNS for test speed/reliability)
	// We need Node B's address.
	// Host.Addrs() returns /ip4/127.0.0.1/tcp/xxxxx
	// We need to construct peer.AddrInfo

	infoB := peer.AddrInfo{
		ID:    nodeB.Host.ID(),
		Addrs: nodeB.Host.Addrs(),
	}

	if err := nodeA.Host.Connect(context.Background(), infoB); err != nil {
		t.Fatalf("Failed to connect A to B: %v", err)
	}

	// Wait for stream handler setup (implicit in Connect? No, handleStream is set)
	// Wait a bit for connection to stabilize?
	time.Sleep(1 * time.Second)

	// Add B to A's peer map manually if logic depends on it (MDNS usually does this)
	// In p2p.go, we add to map in handleStream (incoming) or discovery (outgoing).
	// Since we connected A->B, A is initiator. B receives stream?
	// Our Broadcast logic opens a stream to everyone in `Peers` map.
	// We need to populate Peers map.
	nodeA.Peers[nodeB.Host.ID()] = true
	nodeB.Peers[nodeA.Host.ID()] = true

	// Send Message from A to B
	tx := Transaction{Data: "Test P2P"}
	go nodeA.Broadcast(MsgNewTx, tx)

	// Listen on B
	select {
	case receivedTx := <-nodeB.TxChan:
		if receivedTx.Data != "Test P2P" {
			t.Errorf("Expected 'Test P2P', got '%s'", receivedTx.Data)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestP2PBlockBroadcast(t *testing.T) {
	// Start Node A
	nodeA, err := StartP2PNode(0)
	if err != nil {
		t.Fatalf("Failed to start Node A: %v", err)
	}
	defer nodeA.Host.Close()

	// Start Node B
	nodeB, err := StartP2PNode(0)
	if err != nil {
		t.Fatalf("Failed to start Node B: %v", err)
	}
	defer nodeB.Host.Close()

	infoB := peer.AddrInfo{
		ID:    nodeB.Host.ID(),
		Addrs: nodeB.Host.Addrs(),
	}

	if err := nodeA.Host.Connect(context.Background(), infoB); err != nil {
		t.Fatalf("Failed to connect A to B: %v", err)
	}

	nodeA.Peers[nodeB.Host.ID()] = true
	nodeB.Peers[nodeA.Host.ID()] = true

	// Broadcast Block
	block := Block{Index: 1, Hash: "blockhash"}
	go nodeA.Broadcast(MsgNewBlock, block)

	select {
	case receivedData := <-nodeB.BlockChan:
		if receivedData.Block.Hash != "blockhash" {
			t.Errorf("Expected 'blockhash', got '%s'", receivedData.Block.Hash)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for block")
	}
}
