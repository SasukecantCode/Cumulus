package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type StateManager interface {
	GenerateState() (string, error)
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
	state, err := randomState(32)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.states[state] = time.Now().Add(m.ttl)
	m.mu.Unlock()

	return state, nil
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
