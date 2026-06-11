package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"pr-reviewer/internal/db"
	"pr-reviewer/internal/db/models"
	"pr-reviewer/pkg/logger"
)

var ticketRe = regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,9}-\d+)\b`)

// DetectRefs returns Jira ticket keys found in text (e.g. "PROJ-123").
func DetectRefs(text string) []string {
	seen := map[string]bool{}
	var refs []string
	for _, m := range ticketRe.FindAllString(text, -1) {
		if !seen[m] {
			seen[m] = true
			refs = append(refs, m)
		}
	}
	return refs
}

// Client calls the Jira REST API v3.
type Client struct {
	baseURL    string // e.g. https://yourco.atlassian.net
	authHeader string // "Basic base64(email:token)"
	http       *http.Client
}

func NewClient(baseURL, email, apiToken string) *Client {
	creds := base64.StdEncoding.EncodeToString([]byte(email + ":" + apiToken))
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		authHeader: "Basic " + creds,
		http:       &http.Client{Timeout: 5 * time.Second},
	}
}

type Issue struct {
	Key         string
	Type        string
	Status      string
	Summary     string
	Description string
}

type issueResp struct {
	Key    string `json:"key"`
	Fields struct {
		Summary   string `json:"summary"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		// description is Atlassian Document Format (ADF) — a JSON node tree, not a string.
		Description json.RawMessage `json:"description"`
	} `json:"fields"`
}

// FetchIssue retrieves a single Jira issue by key.
func (c *Client) FetchIssue(ctx context.Context, key string) (*Issue, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=summary,status,issuetype,description", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := c.http.Do(req)
	logger.ExternalCall(ctx, "jira", "GET /issue", start, err, "key", key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // issue doesn't exist — not an error
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: unexpected status %d for issue %s", resp.StatusCode, key)
	}

	var data issueResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &Issue{
		Key:         data.Key,
		Type:        data.Fields.IssueType.Name,
		Status:      data.Fields.Status.Name,
		Summary:     data.Fields.Summary,
		Description: adfToText(data.Fields.Description),
	}, nil
}

// adfToText flattens an Atlassian Document Format (ADF) description into plain
// text by walking the node tree and concatenating text nodes, inserting newlines
// at block boundaries. Returns "" for empty/unparseable input.
func adfToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var b strings.Builder
	walkADF(raw, &b)
	return strings.TrimSpace(b.String())
}

func walkADF(raw json.RawMessage, b *strings.Builder) {
	var node struct {
		Type    string            `json:"type"`
		Text    string            `json:"text"`
		Content []json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &node) != nil {
		return
	}
	if node.Text != "" {
		b.WriteString(node.Text)
	}
	for _, c := range node.Content {
		walkADF(c, b)
	}
	switch node.Type {
	case "paragraph", "heading", "listItem", "blockquote", "codeBlock", "hardBreak":
		b.WriteString("\n")
	}
}

// TestAuth calls /rest/api/3/myself to verify credentials.
// AuthInfo is the identity returned by Jira's /myself — confirms which account
// the credentials resolve to.
type AuthInfo struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
	AccountID   string `json:"accountId"`
}

// TestAuth verifies the credentials against Jira's /myself and returns the
// authenticated account's identity.
func (c *Client) TestAuth(ctx context.Context) (*AuthInfo, error) {
	url := c.baseURL + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	start := time.Now()
	resp, err := c.http.Do(req)
	logger.ExternalCall(ctx, "jira", "GET /myself", start, err)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: auth failed (HTTP %d) — check base URL, email, and API token", resp.StatusCode)
	}
	var info AuthInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("jira: decode /myself: %w", err)
	}
	return &info, nil
}

// FetchContextForPR loads Jira config from DB, detects ticket refs in text, and returns
// a formatted context string suitable for injecting into the review prompt.
// Returns "" if Jira is not configured, disabled, or no refs found.
func FetchContextForPR(ctx context.Context, db_ *gorm.DB, encKey, text string) string {
	client, err := loadClient(ctx, db_, encKey)
	if err != nil || client == nil {
		return ""
	}

	refs := DetectRefs(text)
	if len(refs) == 0 {
		return ""
	}
	// Cap at 3 tickets to avoid slow review jobs.
	if len(refs) > 3 {
		refs = refs[:3]
	}

	var lines []string
	for _, ref := range refs {
		issue, err := client.FetchIssue(ctx, ref)
		if err != nil || issue == nil {
			continue
		}
		line := fmt.Sprintf("- %s [%s / %s]: %s", issue.Key, issue.Type, issue.Status, issue.Summary)
		if desc := truncateRunes(issue.Description, 1500); desc != "" {
			// Indent so it nests under the ticket line in the prompt.
			line += "\n  Description: " + strings.ReplaceAll(desc, "\n", "\n  ")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func truncateRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}

func loadClient(ctx context.Context, gdb *gorm.DB, encKey string) (*Client, error) {
	var cfg models.JiraConfig
	if err := gdb.WithContext(ctx).First(&cfg).Error; err != nil {
		return nil, nil // not configured
	}
	if !cfg.Enabled {
		return nil, nil
	}
	apiToken, err := db.Decrypt(cfg.APITokenEncrypted, encKey)
	if err != nil {
		return nil, fmt.Errorf("jira: decrypt token: %w", err)
	}
	return NewClient(cfg.BaseURL, cfg.Email, apiToken), nil
}
