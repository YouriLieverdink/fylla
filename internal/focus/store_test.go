package focus

import (
	"path/filepath"
	"testing"
)

func TestAddRemoveContains(t *testing.T) {
	s := &State{}
	if s.Contains("kendo", "P-1") {
		t.Fatal("empty state should not contain anything")
	}
	s.Add("kendo", "P-1")
	s.Add("kendo", "P-2")
	s.Add("kendo", "P-1") // duplicate, should no-op
	if got := len(s.Entries); got != 2 {
		t.Fatalf("expected 2 entries, got %d", got)
	}
	if !s.Contains("kendo", "P-1") {
		t.Fatal("expected P-1 to be present")
	}
	s.Remove("kendo", "P-1")
	if s.Contains("kendo", "P-1") {
		t.Fatal("expected P-1 to be removed")
	}
	if len(s.Entries) != 1 {
		t.Fatalf("expected 1 entry after remove, got %d", len(s.Entries))
	}
}

func TestAddAssignsIncreasingPositions(t *testing.T) {
	s := &State{}
	s.Add("p", "a")
	s.Add("p", "b")
	s.Add("p", "c")
	got := s.Sorted()
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].Key != "a" || got[1].Key != "b" || got[2].Key != "c" {
		t.Fatalf("expected order a,b,c got %+v", got)
	}
}

func TestToggle(t *testing.T) {
	s := &State{}
	if added := s.Toggle("p", "a"); !added {
		t.Fatal("first toggle should add")
	}
	if added := s.Toggle("p", "a"); added {
		t.Fatal("second toggle should remove")
	}
	if len(s.Entries) != 0 {
		t.Fatalf("expected empty after toggle off, got %d", len(s.Entries))
	}
}

func TestMove(t *testing.T) {
	s := &State{}
	s.Add("p", "a")
	s.Add("p", "b")
	s.Add("p", "c")

	s.Move("p", "a", 1) // a down -> b, a, c
	got := s.Sorted()
	if got[0].Key != "b" || got[1].Key != "a" || got[2].Key != "c" {
		t.Fatalf("expected b,a,c got %v", keys(got))
	}

	s.Move("p", "c", -1) // c up -> b, c, a
	got = s.Sorted()
	if got[0].Key != "b" || got[1].Key != "c" || got[2].Key != "a" {
		t.Fatalf("expected b,c,a got %v", keys(got))
	}

	// Moving past edges is a no-op.
	s.Move("p", "b", -1)
	got = s.Sorted()
	if got[0].Key != "b" {
		t.Fatalf("expected b at top, got %v", keys(got))
	}
	s.Move("p", "a", 1)
	got = s.Sorted()
	if got[2].Key != "a" {
		t.Fatalf("expected a at bottom, got %v", keys(got))
	}

	// Moving an absent entry is a no-op.
	s.Move("p", "missing", 1)
}

func TestPrune(t *testing.T) {
	s := &State{}
	s.Add("kendo", "P-1")
	s.Add("kendo", "P-2")
	s.Add("github", "x/y#3")

	seen := map[string]bool{
		"kendo|P-1":    true,
		"github|x/y#3": true,
	}
	if !s.Prune(seen) {
		t.Fatal("expected Prune to report mutation")
	}
	if len(s.Entries) != 2 {
		t.Fatalf("expected 2 after prune, got %d", len(s.Entries))
	}
	if s.Contains("kendo", "P-2") {
		t.Fatal("P-2 should have been pruned")
	}
	// Idempotent.
	if s.Prune(seen) {
		t.Fatal("second prune should report no mutation")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "focus.json")

	s := &State{}
	s.Add("kendo", "P-1")
	s.Add("github", "iruoy/fylla#42")
	if err := Save(s, path); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got.Entries))
	}
	if !got.Contains("kendo", "P-1") || !got.Contains("github", "iruoy/fylla#42") {
		t.Fatalf("missing entries after round-trip: %+v", got.Entries)
	}
}

func TestSaveEmptyRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "focus.json")

	s := &State{}
	s.Add("p", "a")
	if err := Save(s, path); err != nil {
		t.Fatalf("save: %v", err)
	}
	s.Remove("p", "a")
	if err := Save(s, path); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(got.Entries) != 0 {
		t.Fatalf("expected empty state, got %+v", got.Entries)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if got == nil || len(got.Entries) != 0 {
		t.Fatalf("expected empty state, got %+v", got)
	}
}

func keys(es []Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Key
	}
	return out
}
