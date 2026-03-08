package otp

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

const (
	codeLength  = 6
	codeTTL     = 5 * time.Minute
	maxAttempts = 5
)

type entry struct {
	code      string
	expiresAt time.Time
	attempts  int
}

type Store struct {
	mu    sync.Mutex
	codes map[string]*entry // key: phone number
}

func NewStore() *Store {
	s := &Store{codes: make(map[string]*entry)}
	// Cleanup goroutine
	go func() {
		for range time.Tick(10 * time.Minute) {
			s.cleanup()
		}
	}()
	return s
}

// Generate creates a new 6-digit code for the phone number and returns it
func (s *Store) Generate(phone string) (string, error) {
	code, err := randomDigits(codeLength)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.codes[phone] = &entry{
		code:      code,
		expiresAt: time.Now().Add(codeTTL),
	}
	s.mu.Unlock()

	return code, nil
}

// Verify checks the code. Returns true if valid, false otherwise.
func (s *Store) Verify(phone, code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.codes[phone]
	if !ok {
		return false
	}

	if time.Now().After(e.expiresAt) {
		delete(s.codes, phone)
		return false
	}

	e.attempts++
	if e.attempts > maxAttempts {
		delete(s.codes, phone)
		return false
	}

	if e.code != code {
		return false
	}

	// Valid — consume it
	delete(s.codes, phone)
	return true
}

func (s *Store) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for phone, e := range s.codes {
		if now.After(e.expiresAt) {
			delete(s.codes, phone)
		}
	}
}

func randomDigits(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	result := make([]byte, n)
	for i := range b {
		result[i] = '0' + (b[i] % 10)
	}
	return fmt.Sprintf("%s", result), nil
}
