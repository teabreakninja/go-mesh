package meshtastic

import (
	"fmt"
	"sync"
	"go-mesh/internal/utils"
)

// SimpleNodeInfo holds basic node information for name resolution
type SimpleNodeInfo struct {
	ID        string
	LongName  string
	ShortName string
}

// NodeDB manages a database of known mesh nodes for name resolution
type NodeDB struct {
	mu    sync.RWMutex
	nodes map[uint32]*SimpleNodeInfo // Map node ID to SimpleNodeInfo
}

// NewNodeDB creates a new node database
func NewNodeDB() *NodeDB {
	return &NodeDB{
		nodes: make(map[uint32]*SimpleNodeInfo),
	}
}

// AddOrUpdateUserInfo adds or updates user information for a node
func (db *NodeDB) AddOrUpdateUserInfo(nodeID uint32, id, longName, shortName string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	existing, exists := db.nodes[nodeID]
	if exists {
		existing.ID = id
		existing.LongName = longName
		existing.ShortName = shortName
	} else {
		// Create new SimpleNodeInfo with user data
		db.nodes[nodeID] = &SimpleNodeInfo{
			ID:        id,
			LongName:  longName,
			ShortName: shortName,
		}
	}
}

// GetNodeName returns the friendly name for a node ID
// Returns long name if available, otherwise short name, otherwise hex ID
func (db *NodeDB) GetNodeName(nodeID uint32) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	node, exists := db.nodes[nodeID]
	if !exists {
		// Return hex format for unknown nodes
		return fmt.Sprintf("!%08x", nodeID)
	}

	if node.LongName != "" {
		return utils.SanitizeForTerminal(node.LongName)
	}
	if node.ShortName != "" {
		return utils.SanitizeForTerminal(node.ShortName)
	}
	if node.ID != "" {
		return node.ID
	}

	// Fallback to hex
	return fmt.Sprintf("!%08x", nodeID)
}

// GetNodeShortName returns the short name for a node ID
// Returns short name if available, otherwise abbreviated long name, otherwise hex ID
func (db *NodeDB) GetNodeShortName(nodeID uint32) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	node, exists := db.nodes[nodeID]
	if !exists {
		return fmt.Sprintf("!%08x", nodeID)
	}

	if node.ShortName != "" {
		return utils.SanitizeForTerminal(node.ShortName)
	}
	if node.LongName != "" {
		// Sanitize and abbreviate long name to 8 characters max
		sanitized := utils.SanitizeForTerminal(node.LongName)
		if len(sanitized) <= 8 {
			return sanitized
		}
		return sanitized[:8]
	}

	// Fallback to hex
	return fmt.Sprintf("!%08x", nodeID)
}

// GetNodeCount returns the number of nodes in the database
func (db *NodeDB) GetNodeCount() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.nodes)
}

// GetAllNodes returns all nodes as a map of nodeID -> SimpleNodeInfo
func (db *NodeDB) GetAllNodes() map[uint32]*SimpleNodeInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	nodes := make(map[uint32]*SimpleNodeInfo)
	for k, v := range db.nodes {
		nodes[k] = v
	}
	return nodes
}
