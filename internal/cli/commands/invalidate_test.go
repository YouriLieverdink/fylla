package commands

import (
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

func seedCache(providers ...string) *TaskCache {
	c := NewTaskCache(time.Minute)
	for _, p := range providers {
		c.Set(p, []task.Task{{Key: p + "-1"}})
	}
	return c
}

func cached(c *TaskCache, provider string) bool {
	_, found, _ := c.Get(provider)
	return found
}

func TestInvalidateProvider_ExplicitProvider(t *testing.T) {
	c := seedCache("kendo", "todoist", "github")
	invalidateProvider(c, "anything", "kendo")

	if cached(c, "kendo") {
		t.Error("kendo entry should be dropped")
	}
	if !cached(c, "todoist") || !cached(c, "github") {
		t.Error("only the affected provider should be dropped")
	}
}

func TestInvalidateProvider_InferFromKey(t *testing.T) {
	c := seedCache("kendo", "todoist", "github")
	// Numeric key → todoist.
	invalidateProvider(c, "123", "")

	if cached(c, "todoist") {
		t.Error("todoist entry should be dropped (inferred from numeric key)")
	}
	if !cached(c, "kendo") || !cached(c, "github") {
		t.Error("kendo/github should remain cached")
	}
}

func TestInvalidate_BulkUnion(t *testing.T) {
	c := seedCache("kendo", "todoist", "github")
	// Mixed-provider bulk result: a Kendo key and a GitHub key.
	for _, k := range []string{"PROJ-1", "owner/repo#5"} {
		c.Invalidate(providerForKey(k))
	}

	if cached(c, "kendo") || cached(c, "github") {
		t.Error("kendo and github entries should both be dropped")
	}
	if !cached(c, "todoist") {
		t.Error("todoist was untouched and should remain cached")
	}
}
