package server

import (
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"sync"

	"sg-emulator/internal/scalegraph"
)

// Registry manages VirtualApps and provides lookup capabilities.
type Registry struct {
	mu     sync.RWMutex
	byID   map[scalegraph.ScalegraphId]*VirtualApp
	logger *slog.Logger
}

// NewRegistry creates a new empty Registry
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		byID:   make(map[scalegraph.ScalegraphId]*VirtualApp),
		logger: logger,
	}
}

// Register adds a VirtualApp to the registry
func (r *Registry) Register(vapp *VirtualApp) error {
	r.logger.Debug("Registering virtual app", "id", vapp.ID())
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[vapp.ID()]; exists {
		r.logger.Warn("Virtual app already registered", "id", vapp.ID())
		return fmt.Errorf("virtual app with ID %s already registered", vapp.ID())
	}

	r.byID[vapp.ID()] = vapp
	r.logger.Info("Virtual app registered", "id", vapp.ID(), "total_apps", len(r.byID))
	return nil
}

// Unregister removes a VirtualApp from the registry
func (r *Registry) Unregister(id scalegraph.ScalegraphId) {
	r.logger.Debug("Unregistering virtual app", "id", id)
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.byID, id)
	r.logger.Info("Virtual app unregistered", "id", id, "remaining_apps", len(r.byID))
}

// GetByID returns a VirtualApp by its ScalegraphId
func (r *Registry) GetByID(id scalegraph.ScalegraphId) (*VirtualApp, bool) {
	r.logger.Debug("Looking up virtual app", "id", id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	vapp, exists := r.byID[id]
	if !exists {
		r.logger.Debug("Virtual app not found", "id", id)
	}
	return vapp, exists
}

// List returns all registered VirtualApps
func (r *Registry) List() []*VirtualApp {
	r.mu.RLock()
	defer r.mu.RUnlock()

	apps := make([]*VirtualApp, 0, len(r.byID))
	for _, vapp := range r.byID {
		apps = append(apps, vapp)
	}
	return apps
}

// Count returns the number of registered VirtualApps
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// GetKClosest returns the k VirtualApps whose IDs are closest to the target ID
// using XOR distance metric (Kademlia-style routing).
// The XOR distance is calculated bitwise between the target ID and each VirtualApp ID.
// Nodes with smaller XOR distances are considered closer.
func (r *Registry) GetKClosest(targetID scalegraph.ScalegraphId, k int) ([]*VirtualApp, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If k is 0 or negative, return error
	if k <= 0 {
		return nil, fmt.Errorf("k must be positive")
	}

	// Collect all apps with their XOR distances
	type appDistance struct {
		app      *VirtualApp
		distance *big.Int
	}

	distances := make([]appDistance, 0, len(r.byID))
	for _, vapp := range r.byID {
		distance := xorDistance(targetID, vapp.ID())
		distances = append(distances, appDistance{
			app:      vapp,
			distance: distance,
		})
	}

	// Sort by XOR distance (ascending)
	sort.Slice(distances, func(i, j int) bool {
		return distances[i].distance.Cmp(distances[j].distance) < 0
	})

	// Return up to k closest apps
	result := make([]*VirtualApp, 0, k)
	for i := 0; i < len(distances) && i < k; i++ {
		result = append(result, distances[i].app)
	}

	return result, nil
}

// xorDistance calculates the XOR distance between two ScalegraphIds.
// The distance is computed by XORing the two IDs byte-by-byte and
// interpreting the result as a big integer for comparison.
func xorDistance(a, b scalegraph.ScalegraphId) *big.Int {
	// XOR each byte of the two IDs
	xor := make([]byte, 20)
	for i := 0; i < 20; i++ {
		xor[i] = a[i] ^ b[i]
	}

	// Convert XOR result to big.Int for distance comparison
	distance := new(big.Int)
	distance.SetBytes(xor)
	return distance
}
