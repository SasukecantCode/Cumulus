package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StateManager interface {
	GenerateState() (string, error)
	GenerateStateWithContext(workspaceID uint64, uiPort string) (string, error)
	ValidateState(state string) error
}

type InMemoryStateManager struct {
	mu     sync.Mutex
	states map[string]time.Time
	ttl    time.Duration
}

func NewInMemoryStateManager(ttl time.Duration) *InMemoryStateManager {
	return &InMemoryStateManager{
		states: make(map[string]time.Time),
		ttl:    ttl,
	}
}

func (m *InMemoryStateManager) GenerateState() (string, error) {
	return m.GenerateStateWithContext(0, "3000")
}

func (m *InMemoryStateManager) GenerateStateWithContext(workspaceID uint64, uiPort string) (string, error) {
	randToken, err := randomState(16)
	if err != nil {
		return "", err
	}

	if uiPort == "" {
		uiPort = "3000"
	}

	state := fmt.Sprintf("ws_%d_ui_%s_%s", workspaceID, uiPort, randToken)

	m.mu.Lock()
	now := time.Now()
	for k, expiry := range m.states {
		if now.After(expiry) {
			delete(m.states, k)
		}
	}
	m.states[state] = now.Add(m.ttl)
	m.mu.Unlock()

	return state, nil
}

func ParseStateContext(state string) (workspaceID uint64, uiPort string) {
	if !strings.HasPrefix(state, "ws_") {
		return 0, "3000"
	}

	parts := strings.Split(state, "_")
	// Expected format: ["ws", "<workspaceID>", "ui", "<uiPort>", "<randToken>"]
	if len(parts) >= 5 && parts[0] == "ws" && parts[2] == "ui" {
		if id, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
			workspaceID = id
		}
		uiPort = parts[3]
		return workspaceID, uiPort
	}

	return 0, "3000"
}

func (m *InMemoryStateManager) ValidateState(state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	expiry, exists := m.states[state]
	if !exists {
		return errors.New("invalid or expired state")
	}
	if time.Now().After(expiry) {
		delete(m.states, state)
		return errors.New("invalid or expired state")
	}

	delete(m.states, state)
	return nil
}

func randomState(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
