// Package jibble implements a worklog-only task provider backed by Jibble
// (https://jibble.io), a time-clock product. Jibble has no concept of tasks or
// issues — only Clients, Projects, and Time Entries — so this client implements
// the worklog surface (PostWorklog, FetchWorklogs, ListProjects) and stubs the
// rest of the TaskSource interface with ErrUnsupported. FetchTasks returns an
// empty slice so Jibble contributes no tasks to the tasks/schedule tabs.
//
// Authentication is OAuth2 client-credentials: an API key (client_id) and
// secret (client_secret) are exchanged at the identity endpoint for a
// short-lived JWT, cached and refreshed on expiry or a 401.
//
// API-shape assumptions to confirm against the live API (centralized here so a
// mismatch is a one-spot fix):
//   - Time is modelled as In/Out punches; a worked block is an "In" entry at the
//     start carrying the projectId+note, then an "Out" entry at start+duration.
//   - Backdated entries are accepted (Jibble supports manual time entries).
//   - OData collections are returned as {"value": [...]}.
package jibble

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// ErrUnsupported is returned for operations Jibble doesn't model (it has no tasks).
var ErrUnsupported = errors.New("operation not supported for Jibble (worklog-only provider)")

const (
	defaultIdentityURL = "https://identity.prod.jibble.io/connect/token"
	// Jibble splits its API across service hosts: organization structure
	// (People, Projects, Clients, Activities) lives on the workspace host,
	// while punches live on the time-tracking host.
	defaultWorkspaceBaseURL = "https://workspace.prod.jibble.io/v1"
	defaultTimeBaseURL      = "https://time-tracking.prod.jibble.io/v1"
)

// Client talks to the Jibble API as a worklog provider.
type Client struct {
	key    string
	secret string

	HTTPClient *http.Client

	identityURL      string
	workspaceBaseURL string // People, Projects, Clients
	timeBaseURL      string // TimeEntries

	platform platformInfo

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time

	personOnce sync.Once
	personID   string
	personErr  error

	projectsMu   sync.Mutex
	projectsDone bool
	labels       []string          // "Client / Project" labels, sorted
	labelToID    map[string]string // label -> project id
	nameByID     map[string]string // project id -> bare project name
	labelByID    map[string]string // project id -> "Client / Project" label
}

// NewClient creates a Jibble client from an API key/secret pair.
func NewClient(key, secret string) *Client {
	return &Client{
		key:              key,
		secret:           secret,
		HTTPClient:       &http.Client{Timeout: 30 * time.Second},
		identityURL:      defaultIdentityURL,
		workspaceBaseURL: defaultWorkspaceBaseURL,
		timeBaseURL:      defaultTimeBaseURL,
		platform:         buildPlatform(),
	}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

// token returns a valid bearer token, fetching a fresh one when the cache is
// empty or expired.
func (c *Client) token(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	return c.fetchTokenLocked(ctx)
}

func (c *Client) fetchTokenLocked(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.key},
		"client_secret": {c.secret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.identityURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("jibble token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("jibble token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jibble token: status %d: %s", resp.StatusCode, string(body))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("jibble token decode: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("jibble token: empty access_token")
	}
	c.accessToken = tok.AccessToken
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	// Refresh a minute early to avoid using a token mid-expiry.
	c.tokenExpiry = time.Now().Add(ttl - time.Minute)
	return c.accessToken, nil
}

func (c *Client) invalidateToken() {
	c.tokenMu.Lock()
	c.accessToken = ""
	c.tokenExpiry = time.Time{}
	c.tokenMu.Unlock()
}

// do performs an authenticated request against the given service host base,
// refreshing the token and retrying once on a 401.
func (c *Client) do(ctx context.Context, base, method, path string, body interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("jibble marshal request: %w", err)
		}
	}

	send := func() (*http.Response, error) {
		tok, err := c.token(ctx)
		if err != nil {
			return nil, err
		}
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("jibble create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return c.httpClient().Do(req)
	}

	resp, err := send()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.invalidateToken()
		return send()
	}
	return resp, nil
}

// getValue performs a GET against the given host base and decodes an OData
// {"value": [...]} envelope into out.
func (c *Client) getValue(ctx context.Context, base, path string, out interface{}) error {
	resp, err := c.do(ctx, base, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jibble GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	env := struct {
		Value json.RawMessage `json:"value"`
	}{}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("jibble decode %s: %w", path, err)
	}
	if err := json.Unmarshal(env.Value, out); err != nil {
		return fmt.Errorf("jibble decode %s value: %w", path, err)
	}
	return nil
}

type jibbleProject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ClientID string `json:"clientId"`
}

type jibbleClient struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jibblePerson struct {
	ID string `json:"id"`
}

// loadProjects fetches projects + clients once and builds the label/id maps.
func (c *Client) loadProjects(ctx context.Context) error {
	c.projectsMu.Lock()
	defer c.projectsMu.Unlock()
	if c.projectsDone {
		return nil
	}

	var projects []jibbleProject
	if err := c.getValue(ctx, c.workspaceBaseURL, "/Projects", &projects); err != nil {
		return fmt.Errorf("jibble load projects: %w", err)
	}
	var clients []jibbleClient
	if err := c.getValue(ctx, c.workspaceBaseURL, "/Clients", &clients); err != nil {
		return fmt.Errorf("jibble load clients: %w", err)
	}

	clientName := make(map[string]string, len(clients))
	for _, cl := range clients {
		clientName[cl.ID] = cl.Name
	}

	c.labelToID = make(map[string]string, len(projects))
	c.nameByID = make(map[string]string, len(projects))
	c.labelByID = make(map[string]string, len(projects))
	c.labels = c.labels[:0]
	for _, p := range projects {
		label := p.Name
		if cn := clientName[p.ClientID]; cn != "" {
			label = cn + " / " + p.Name
		}
		c.labels = append(c.labels, label)
		c.labelToID[label] = p.ID
		c.nameByID[p.ID] = p.Name
		c.labelByID[p.ID] = label
	}
	sort.Strings(c.labels)
	c.projectsDone = true
	return nil
}

// resolveProjectID maps a worklog target (a "Client / Project" label from
// ListProjects, or a raw project id) to a Jibble project id.
func (c *Client) resolveProjectID(ctx context.Context, target string) (string, error) {
	if err := c.loadProjects(ctx); err != nil {
		return "", err
	}
	c.projectsMu.Lock()
	defer c.projectsMu.Unlock()
	if id, ok := c.labelToID[target]; ok {
		return id, nil
	}
	if _, ok := c.nameByID[target]; ok {
		return target, nil // already an id
	}
	return "", fmt.Errorf("jibble: unknown project %q", target)
}

func (c *Client) projectName(id string) string {
	c.projectsMu.Lock()
	defer c.projectsMu.Unlock()
	if n, ok := c.nameByID[id]; ok {
		return n
	}
	return id
}

// projectLabel returns the "Client / Project" label for a project id, falling
// back to the bare project name.
func (c *Client) projectLabel(id string) string {
	c.projectsMu.Lock()
	defer c.projectsMu.Unlock()
	if l, ok := c.labelByID[id]; ok {
		return l
	}
	return c.nameByID[id]
}

// person resolves the single org member's id (one org, one member).
func (c *Client) person(ctx context.Context) (string, error) {
	c.personOnce.Do(func() {
		var people []jibblePerson
		if err := c.getValue(ctx, c.workspaceBaseURL, "/People", &people); err != nil {
			c.personErr = fmt.Errorf("jibble load person: %w", err)
			return
		}
		if len(people) == 0 {
			c.personErr = fmt.Errorf("jibble: no people in organization")
			return
		}
		c.personID = people[0].ID
	})
	return c.personID, c.personErr
}

// ListProjects returns the "Client / Project" labels used as worklog targets.
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}
	c.projectsMu.Lock()
	defer c.projectsMu.Unlock()
	out := make([]string, len(c.labels))
	copy(out, c.labels)
	return out, nil
}

// hourEntryPost is the create body for a Jibble HourEntry — a manual time block
// modelled as a date + duration (Jibble's "add time entry" feature), avoiding
// the In/Out punch pairing of TimeEntries.
type hourEntryPost struct {
	PersonID   string       `json:"personId"`
	Date       string       `json:"date"`     // YYYY-MM-DD (Edm.Date)
	Duration   string       `json:"duration"` // ISO-8601 (Edm.Duration), e.g. "PT1H30M"
	ClientType string       `json:"clientType"`
	Platform   platformInfo `json:"platform"`
	ProjectID  string       `json:"projectId,omitempty"`
	Note       string       `json:"note,omitempty"`
}

// clientTypeWeb is a valid TimeEntryClientType enum member (per OData $metadata).
const clientTypeWeb = "Web"

// platformInfo is the required client-metadata object on created entries.
// Fields mirror Jibble's PlatformModel (per the OData $metadata) — note there is
// no clientType here; that is a separate top-level field. Empty fields are
// omitted rather than sent as placeholders.
type platformInfo struct {
	ClientVersion string `json:"clientVersion,omitempty"`
	OS            string `json:"os,omitempty"`
	DeviceModel   string `json:"deviceModel,omitempty"`
	DeviceName    string `json:"deviceName,omitempty"`
}

// buildPlatform describes this fylla install as a Jibble PlatformModel using
// real host details instead of placeholder "unknown" values.
func buildPlatform() platformInfo {
	osName := runtime.GOOS
	switch osName {
	case "darwin":
		osName = "macOS"
	case "windows":
		osName = "Windows"
	case "linux":
		osName = "Linux"
	}
	host, _ := os.Hostname()
	return platformInfo{
		ClientVersion: "fylla",
		OS:            osName,
		DeviceName:    host,
	}
}

// PostWorklog records a worked block as a single Jibble HourEntry (date +
// duration) against a Jibble Project. issueKey is a worklog target (a label from
// ListProjects or a raw project id). The work day is taken from started's local
// date; HourEntry has no clock time, only date + duration.
func (c *Client) PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error {
	if timeSpent <= 0 {
		return nil
	}
	projectID, err := c.resolveProjectID(ctx, issueKey)
	if err != nil {
		return err
	}
	personID, err := c.person(ctx)
	if err != nil {
		return err
	}

	payload := hourEntryPost{
		PersonID:   personID,
		Date:       started.Format("2006-01-02"),
		Duration:   iso8601Duration(timeSpent),
		ClientType: clientTypeWeb,
		Platform:   c.platform,
		ProjectID:  projectID,
		Note:       description,
	}
	resp, err := c.do(ctx, c.timeBaseURL, http.MethodPost, "/HourEntries", payload)
	if err != nil {
		return fmt.Errorf("jibble post hour entry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jibble post hour entry: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

type hourEntryJSON struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Duration  string `json:"duration"`
	ProjectID string `json:"projectId"`
	Note      string `json:"note"`
}

// FetchWorklogs lists HourEntries in the date window. WorklogEntry.Project
// carries the bare Jibble Project name so hour targets (keyed by project name)
// match; Started is the entry's local date (HourEntry has no clock time).
func (c *Client) FetchWorklogs(ctx context.Context, since, until time.Time, filter task.WorklogFilter) ([]task.WorklogEntry, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("$filter", fmt.Sprintf("date ge %s and date le %s",
		since.Format("2006-01-02"), until.Format("2006-01-02")))
	q.Set("$orderby", "date")

	var entries []hourEntryJSON
	if err := c.getValue(ctx, c.timeBaseURL, "/HourEntries?"+q.Encode(), &entries); err != nil {
		return nil, err
	}

	var out []task.WorklogEntry
	for _, e := range entries {
		dur := iso8601Parse(e.Duration)
		if dur <= 0 {
			continue
		}
		name := c.projectName(e.ProjectID)
		if filter.Project != "" && !strings.EqualFold(filter.Project, name) {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", e.Date, time.Local)
		if err != nil {
			continue
		}
		out = append(out, task.WorklogEntry{
			ID:       e.ID,
			Provider: "jibble",
			Project:  name,
			// IssueSummary carries the "Client / Project" label for display;
			// Project stays the bare name so hour targets match.
			IssueSummary: c.projectLabel(e.ProjectID),
			Description:  e.Note,
			Started:      day,
			TimeSpent:    dur,
			DateOnly:     true,
		})
	}
	return out, nil
}

// UpdateWorklog edits an existing HourEntry. worklogID is the entry id;
// issueKey, when non-empty, is a worklog target whose project is applied.
func (c *Client) UpdateWorklog(ctx context.Context, issueKey, worklogID string, timeSpent time.Duration, description string, started time.Time) error {
	payload := map[string]interface{}{
		"date":     started.Format("2006-01-02"),
		"duration": iso8601Duration(timeSpent),
		"note":     description,
	}
	if issueKey != "" {
		projectID, err := c.resolveProjectID(ctx, issueKey)
		if err != nil {
			return err
		}
		payload["projectId"] = projectID
	}
	resp, err := c.do(ctx, c.timeBaseURL, http.MethodPatch, fmt.Sprintf("/HourEntries(%s)", worklogID), payload)
	if err != nil {
		return fmt.Errorf("jibble update hour entry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jibble update hour entry: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteWorklog removes an HourEntry by id.
func (c *Client) DeleteWorklog(ctx context.Context, _, worklogID string) error {
	resp, err := c.do(ctx, c.timeBaseURL, http.MethodDelete, fmt.Sprintf("/HourEntries(%s)", worklogID), nil)
	if err != nil {
		return fmt.Errorf("jibble delete hour entry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jibble delete hour entry: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

var isoDurationRe = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

// iso8601Duration formats a duration as an ISO-8601 / Edm.Duration string.
func iso8601Duration(d time.Duration) string {
	total := int(d.Round(time.Second) / time.Second)
	if total <= 0 {
		return "PT0S"
	}
	h, m, s := total/3600, (total%3600)/60, total%60
	var b strings.Builder
	b.WriteString("PT")
	if h > 0 {
		fmt.Fprintf(&b, "%dH", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dM", m)
	}
	if s > 0 {
		fmt.Fprintf(&b, "%dS", s)
	}
	if b.Len() == 2 {
		b.WriteString("0S")
	}
	return b.String()
}

// iso8601Parse parses an ISO-8601 / Edm.Duration string (e.g. "PT1H30M",
// "P0DT1H30M0S"). Returns 0 if unparseable.
func iso8601Parse(s string) time.Duration {
	m := isoDurationRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var d time.Duration
	if m[1] != "" {
		n, _ := strconv.Atoi(m[1])
		d += time.Duration(n) * 24 * time.Hour
	}
	if m[2] != "" {
		n, _ := strconv.Atoi(m[2])
		d += time.Duration(n) * time.Hour
	}
	if m[3] != "" {
		n, _ := strconv.Atoi(m[3])
		d += time.Duration(n) * time.Minute
	}
	if m[4] != "" {
		f, _ := strconv.ParseFloat(m[4], 64)
		d += time.Duration(f * float64(time.Second))
	}
	return d
}

// FetchTasks returns no tasks: Jibble is a worklog-only provider. It must return
// (nil, nil) — an error would make the multi-provider fetcher report a partial
// failure on every fetch.
func (c *Client) FetchTasks(_ context.Context, _ string) ([]task.Task, error) {
	return nil, nil
}

// --- Unsupported task operations (Jibble has no tasks) ---

func (c *Client) CreateTask(_ context.Context, _ task.CreateInput) (string, error) {
	return "", fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) CompleteTask(_ context.Context, _ string) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) DeleteTask(_ context.Context, _ string) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) GetEstimate(_ context.Context, _ string) (time.Duration, error) {
	return 0, fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) UpdateEstimate(_ context.Context, _ string, _ time.Duration) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) GetDueDate(_ context.Context, _ string) (*time.Time, error) {
	return nil, fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) UpdateDueDate(_ context.Context, _ string, _ time.Time) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) RemoveDueDate(_ context.Context, _ string) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) GetPriority(_ context.Context, _ string) (int, error) {
	return 0, fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) UpdatePriority(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) GetSummary(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("jibble: %w", ErrUnsupported)
}

func (c *Client) UpdateSummary(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("jibble: %w", ErrUnsupported)
}
