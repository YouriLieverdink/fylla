package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/iruoy/fylla/internal/config"
)

type fakeKendoResolver struct {
	projectID int
	err       error
}

func (f fakeKendoResolver) ProjectIDForKey(_ context.Context, _ string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.projectID, nil
}

func TestBuildTaskProviderURL(t *testing.T) {
	ctx := context.Background()

	t.Run("github pull request url", func(t *testing.T) {
		cfg := &config.Config{}
		url, err := buildTaskProviderURL(ctx, cfg, "fylla#42", "github", "iruoy/fylla", "Pull Request", nil)
		if err != nil {
			t.Fatalf("buildTaskProviderURL: %v", err)
		}
		want := "https://github.com/iruoy/fylla/pull/42"
		if url != want {
			t.Fatalf("url = %q, want %q", url, want)
		}
	})

	t.Run("github issue url from key owner repo", func(t *testing.T) {
		cfg := &config.Config{}
		url, err := buildTaskProviderURL(ctx, cfg, "iruoy/fylla#99", "github", "", "Issue", nil)
		if err != nil {
			t.Fatalf("buildTaskProviderURL: %v", err)
		}
		want := "https://github.com/iruoy/fylla/issues/99"
		if url != want {
			t.Fatalf("url = %q, want %q", url, want)
		}
	})

	t.Run("github short key resolved via configured repos", func(t *testing.T) {
		cfg := &config.Config{
			GitHub: config.GitHubConfig{
				Repos: []string{"iruoy/fylla"},
			},
		}
		url, err := buildTaskProviderURL(ctx, cfg, "fylla#7", "github", "", "Issue", nil)
		if err != nil {
			t.Fatalf("buildTaskProviderURL: %v", err)
		}
		want := "https://github.com/iruoy/fylla/issues/7"
		if url != want {
			t.Fatalf("url = %q, want %q", url, want)
		}
	})

	t.Run("github short key without configured repo errors", func(t *testing.T) {
		cfg := &config.Config{}
		_, err := buildTaskProviderURL(ctx, cfg, "fylla#7", "github", "", "Issue", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "github repo") {
			t.Fatalf("err = %v, want github repo error", err)
		}
	})

	t.Run("kendo url uses project id path", func(t *testing.T) {
		cfg := &config.Config{
			Kendo: config.KendoConfig{URL: "https://script.kendo.dev"},
		}
		url, err := buildTaskProviderURL(ctx, cfg, "HW-0052", "kendo", "HW", "Task", fakeKendoResolver{projectID: 3})
		if err != nil {
			t.Fatalf("buildTaskProviderURL: %v", err)
		}
		want := "https://script.kendo.dev/projects/3/issues/HW-0052"
		if url != want {
			t.Fatalf("url = %q, want %q", url, want)
		}
	})

	t.Run("kendo project lookup failure errors", func(t *testing.T) {
		cfg := &config.Config{
			Kendo: config.KendoConfig{URL: "https://script.kendo.dev"},
		}
		_, err := buildTaskProviderURL(ctx, cfg, "HW-0052", "kendo", "HW", "Task", fakeKendoResolver{err: context.DeadlineExceeded})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "resolve kendo project id") {
			t.Fatalf("err = %v, want resolve error", err)
		}
	})

	t.Run("todoist url", func(t *testing.T) {
		cfg := &config.Config{}
		url, err := buildTaskProviderURL(ctx, cfg, "123456", "todoist", "", "Task", nil)
		if err != nil {
			t.Fatalf("buildTaskProviderURL: %v", err)
		}
		want := "https://app.todoist.com/app/task/123456"
		if url != want {
			t.Fatalf("url = %q, want %q", url, want)
		}
	})

	t.Run("local provider unsupported", func(t *testing.T) {
		cfg := &config.Config{}
		_, err := buildTaskProviderURL(ctx, cfg, "L-1", "local", "", "Task", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "does not support") {
			t.Fatalf("err = %v, want unsupported error", err)
		}
	})

	t.Run("provider is required", func(t *testing.T) {
		cfg := &config.Config{}
		_, err := buildTaskProviderURL(ctx, cfg, "HW-0052", "", "HW", "Task", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "provider is required") {
			t.Fatalf("err = %v, want provider required error", err)
		}
	})
}
