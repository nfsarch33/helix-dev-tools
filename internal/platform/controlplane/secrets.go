package controlplane

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type SecretSource int

const (
	SourceEnv SecretSource = iota
	SourceKeychain
	SourceFile
)

type SecretEntry struct {
	Name   string
	Source SecretSource
	Value  string
}

type SecretResolver struct {
	mu      sync.RWMutex
	cache   map[string]SecretEntry
	service string
}

func NewSecretResolver(keychainService string) *SecretResolver {
	return &SecretResolver{
		cache:   make(map[string]SecretEntry),
		service: keychainService,
	}
}

func (s *SecretResolver) Resolve(name string) (string, error) {
	s.mu.RLock()
	if cached, ok := s.cache[name]; ok {
		s.mu.RUnlock()
		return cached.Value, nil
	}
	s.mu.RUnlock()

	if val := os.Getenv(name); val != "" {
		s.store(name, val, SourceEnv)
		return val, nil
	}

	if val := s.fromKeychain(name); val != "" {
		s.store(name, val, SourceKeychain)
		return val, nil
	}

	return "", fmt.Errorf("secret %q not found in env or keychain", name)
}

func (s *SecretResolver) fromKeychain(name string) string {
	serviceName := s.service
	if serviceName == "" {
		serviceName = "helixon"
	}
	cmd := exec.Command("/usr/bin/security", "find-generic-password", "-s", serviceName, "-a", name, "-w")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *SecretResolver) store(name, value string, source SecretSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[name] = SecretEntry{Name: name, Source: source, Value: value}
}

func (s *SecretResolver) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]SecretEntry)
}

func (s *SecretResolver) CachedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}
