package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
)

// P2P Message Types
const (
	MsgNewTx     = "NEW_TX"
	MsgNewBlock  = "NEW_BLOCK"
	MsgGetBlocks = "GET_BLOCKS"
	MsgChain     = "CHAIN"
	MsgStatus    = "STATUS"
)

type Message struct {
	Type    string
	Payload []byte
}

type P2PNode struct {
	Host      host.Host
	Peers     map[peer.ID]bool
	TxChan    chan Transaction
	BlockChan chan struct {
		Block Block
		From  peer.ID
	}
	ChainChan chan struct {
		Chain []Block
		From  peer.ID
	}
	GetBlocksChan chan peer.ID
}

const protocolID = "/blockchain/1.0.0"
const discoveryNamespace = "myblockchain-pubsub"

func StartP2PNode(listenPort int) (*P2PNode, error) {
	// Create a new libp2p Host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.Security(noise.ID, noise.New),
	)
	if err != nil {
		return nil, err
	}

	node := &P2PNode{
		Host:   h,
		Peers:  make(map[peer.ID]bool),
		TxChan: make(chan Transaction),
		BlockChan: make(chan struct {
			Block Block
			From  peer.ID
		}),
		ChainChan: make(chan struct {
			Chain []Block
			From  peer.ID
		}),
		GetBlocksChan: make(chan peer.ID),
	}

	// Set stream handler used when a peer opens a stream
	h.SetStreamHandler(protocolID, node.handleStream)

	// Setup MDNS discovery
	if os.Getenv("DISABLE_MDNS") != "true" {
		if err := setupDiscovery(h, node); err != nil {
			return nil, err
		}
	}

	log.Printf("P2P Node started. ID: %s, Addrs: %v", h.ID(), h.Addrs())
	return node, nil
}

func (n *P2PNode) handleStream(s network.Stream) {
	log.Printf("New P2P stream from %s", s.Conn().RemotePeer())
	n.Peers[s.Conn().RemotePeer()] = true

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	go n.readData(rw, s.Conn().RemotePeer())
}

func (n *P2PNode) readData(rw *bufio.ReadWriter, sender peer.ID) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			// Stream closed or error
			return
		}

		if str == "" {
			return
		}

		var msg Message
		if err := json.Unmarshal([]byte(str), &msg); err != nil {
			log.Printf("Error decoding message: %v", err)
			continue
		}

		switch msg.Type {
		case MsgNewTx:
			var tx Transaction
			if err := json.Unmarshal(msg.Payload, &tx); err != nil {
				log.Printf("Error decoding tx: %v", err)
				continue
			}
			// Send to main loop
			n.TxChan <- tx

		case MsgNewBlock:
			var block Block
			if err := json.Unmarshal(msg.Payload, &block); err != nil {
				log.Printf("Error decoding block: %v", err)
				continue
			}
			n.BlockChan <- struct {
				Block Block
				From  peer.ID
			}{Block: block, From: sender}

		case MsgStatus:
			log.Printf("Received STATUS message")

		case MsgGetBlocks:
			// Signal that this peer wants blocks
			n.GetBlocksChan <- sender

		case MsgChain:
			var chain []Block
			if err := json.Unmarshal(msg.Payload, &chain); err != nil {
				log.Printf("Error decoding chain: %v", err)
				continue
			}
			n.ChainChan <- struct {
				Chain []Block
				From  peer.ID
			}{Chain: chain, From: sender}
		}
	}
}

func (n *P2PNode) Broadcast(msgType string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload: %v", err)
		return
	}

	msg := Message{
		Type:    msgType,
		Payload: data,
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for p := range n.Peers {
		n.SendToPeer(p, bytes)
	}
}

func (n *P2PNode) SendToPeer(p peer.ID, msgBytes []byte) {
	if p == n.Host.ID() {
		return
	}

	s, err := n.Host.NewStream(context.Background(), p, protocolID)
	if err != nil {
		log.Printf("Error opening stream to %s: %v", p, err)
		delete(n.Peers, p) // Remove peer if connection fails
		return
	}

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	rw.WriteString(string(msgBytes) + "\n")
	rw.Flush()
	s.Close()
}

func (n *P2PNode) SendMessageToPeer(peerID peer.ID, msgType string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling payload: %v", err)
		return
	}

	msg := Message{
		Type:    msgType,
		Payload: data,
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	n.SendToPeer(peerID, bytes)
}

func (n *P2PNode) ConnectToPeer(peerAddr string) error {
	addr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return err
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}
	if err := n.Host.Connect(context.Background(), *info); err != nil {
		return err
	}
	n.Peers[info.ID] = true
	log.Printf("Connected to peer: %s", info.ID)
	return nil
}

// MDNS Discovery
type discoveryNotifee struct {
	h host.Host
	n *P2PNode
}

func (d *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == d.h.ID() {
		return
	}
	log.Printf("Discovered new peer: %s", pi.ID)

	// Connect to peer
	if err := d.h.Connect(context.Background(), pi); err != nil {
		log.Printf("Connection error: %s", err)
		return
	}

	d.n.Peers[pi.ID] = true
	log.Printf("Connected to peer: %s", pi.ID)

	// Proactively request blocks from the new peer
	d.n.SendMessageToPeer(pi.ID, MsgGetBlocks, "")
}

func setupDiscovery(h host.Host, n *P2PNode) error {
	s := mdns.NewMdnsService(h, discoveryNamespace, &discoveryNotifee{h: h, n: n})
	return s.Start()
}
