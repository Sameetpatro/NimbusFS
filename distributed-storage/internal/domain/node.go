package domain

import "time"

// NodeStatus represents the lifecycle state of a storage node.
// using iota + stringer makes log output human readable
type NodeStatus int

const (
	// NodeStatusUnknown is the zero value before we've heard from a node at all
	NodeStatusUnknown NodeStatus = iota
	// NodeStatusAlive means heartbeat received within threshold, safe for new placements
	NodeStatusAlive
	// NodeStatusSuspect means missed one heartbeat interval, don't place new chunks yet
	NodeStatusSuspect
	// NodeStatusDead means missed threshold, triggers re-replication of chunks it held
	NodeStatusDead
)

// String implements fmt.Stringer so slog and metrics labels show "Alive" not "1"
func (s NodeStatus) String() string {
	switch s {
	case NodeStatusAlive:
		return "alive"
	case NodeStatusSuspect:
		return "suspect"
	case NodeStatusDead:
		return "dead"
	default:
		return "unknown"
	}
}

// StorageNode is the master's view of a storage node.
// capacity fields let master make intelligent placement decisions
type StorageNode struct {
	// stable id assigned at registration, survives restarts even when ip changes
	NodeID string
	// host:port for grpc connections, resolved at runtime not baked into chunk metadata
	Address string
	// current liveness from heartbeat monitor, drives placement and re-replication
	Status NodeStatus
	// updated on every heartbeat receipt so we can compute silence duration
	LastHeartbeat time.Time
	// bytes total on disk, reported by node so master avoids overfilling volumes
	TotalSpace int64
	// bytes used, updated with each chunk stored for capacity-aware replica picking
	UsedSpace int64
	// number of chunks held, for load balancing across otherwise equal-capacity nodes
	ChunkCount int
}
