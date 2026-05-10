// Package focus stores the user's curated subset of tasks ("focus list")
// and their manual ordering. Persisted as JSON under the active profile dir.
package focus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/iruoy/fylla/internal/config"
)

// Entry is a single focused task: provider + task key, plus the user's
// chosen position. Lower positions render earlier.
type Entry struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Position int    `json:"position"`
}

// State is the full focus list.
type State struct {
	Entries []Entry `json:"entries"`
}

// DefaultPath returns the active profile's focus.json path.
func DefaultPath() (string, error) {
	dir, err := config.ProfileDir()
	if err != nil {
		return "", fmt.Errorf("profile dir: %w", err)
	}
	return filepath.Join(dir, "focus.json"), nil
}

// Load reads the focus state from path. Returns an empty state if the
// file does not exist.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read focus state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse focus state: %w", err)
	}
	return &s, nil
}

// Save writes the state to path. Removes the file when state is empty.
func Save(s *State, path string) error {
	if s == nil || len(s.Entries) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove focus state: %w", err)
		}
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create focus dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal focus state: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Contains reports whether (provider, key) is in the focus list.
func (s *State) Contains(provider, key string) bool {
	for _, e := range s.Entries {
		if e.Provider == provider && e.Key == key {
			return true
		}
	}
	return false
}

// Add appends (provider, key) to the focus list with the next position.
// No-op if already present.
func (s *State) Add(provider, key string) {
	if s.Contains(provider, key) {
		return
	}
	pos := 0
	for _, e := range s.Entries {
		if e.Position >= pos {
			pos = e.Position + 1
		}
	}
	s.Entries = append(s.Entries, Entry{Provider: provider, Key: key, Position: pos})
}

// Remove drops (provider, key) from the focus list. No-op if absent.
func (s *State) Remove(provider, key string) {
	out := s.Entries[:0]
	for _, e := range s.Entries {
		if e.Provider == provider && e.Key == key {
			continue
		}
		out = append(out, e)
	}
	s.Entries = out
}

// Toggle adds (provider, key) if absent, removes it if present. Returns
// true if the entry is now in the list.
func (s *State) Toggle(provider, key string) bool {
	if s.Contains(provider, key) {
		s.Remove(provider, key)
		return false
	}
	s.Add(provider, key)
	return true
}

// Move shifts (provider, key) up (delta < 0) or down (delta > 0) by
// swapping positions with the neighbour. delta of any magnitude collapses
// to a single-step swap.
func (s *State) Move(provider, key string, delta int) {
	if delta == 0 {
		return
	}
	sorted := s.sortedIndices()
	idx := -1
	for i, ei := range sorted {
		e := s.Entries[ei]
		if e.Provider == provider && e.Key == key {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	target := idx + sign(delta)
	if target < 0 || target >= len(sorted) {
		return
	}
	a := &s.Entries[sorted[idx]]
	b := &s.Entries[sorted[target]]
	a.Position, b.Position = b.Position, a.Position
}

// Sorted returns entries ordered by position ascending.
func (s *State) Sorted() []Entry {
	out := make([]Entry, len(s.Entries))
	copy(out, s.Entries)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Position < out[j].Position
	})
	return out
}

// Prune drops entries whose composite key (provider+"|"+key) is not in
// seen. Returns true if any entry was removed.
func (s *State) Prune(seen map[string]bool) bool {
	out := s.Entries[:0]
	mutated := false
	for _, e := range s.Entries {
		if seen[e.Provider+"|"+e.Key] {
			out = append(out, e)
			continue
		}
		mutated = true
	}
	s.Entries = out
	return mutated
}

func (s *State) sortedIndices() []int {
	idx := make([]int, len(s.Entries))
	for i := range s.Entries {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		return s.Entries[idx[i]].Position < s.Entries[idx[j]].Position
	})
	return idx
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}
