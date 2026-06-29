package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

var httpClient = &http.Client{Timeout: 8 * time.Second}

// PostResponseURL posts a message back to a slash command's response_url. The response
// URL is a short-lived (30 min, 5 use) webhook Slack provides with each command.
func PostResponseURL(ctx context.Context, responseURL, text string) error {
	body, _ := json.Marshal(map[string]any{
		"response_type": "in_channel",
		"text":          text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := httpClient.Do(req)
	logger.ExternalCall(ctx, "slack", "POST response_url", start, err)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack: response_url returned %d", resp.StatusCode)
	}
	return nil
}

// PostMessage sends a message to a channel via chat.postMessage using a bot token (xoxb-).
// Used for app_mention replies, which (unlike slash commands) have no response_url.
func PostMessage(ctx context.Context, botToken, channel, text string) error {
	if botToken == "" {
		return fmt.Errorf("slack: bot token not configured")
	}
	body, _ := json.Marshal(map[string]any{"channel": channel, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+botToken)
	start := time.Now()
	resp, err := httpClient.Do(req)
	logger.ExternalCall(ctx, "slack", "chat.postMessage", start, err, "channel", channel)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("slack: decode chat.postMessage response: %w", err)
	}
	if !out.OK {
		return fmt.Errorf("slack: chat.postMessage failed: %s", out.Error)
	}
	return nil
}

// TestAuth verifies a bot token via auth.test.
// AuthInfo is the identity Slack returns from auth.test — useful for confirming
// exactly which workspace and bot user a token belongs to.
type AuthInfo struct {
	Team   string `json:"team"`
	TeamID string `json:"team_id"`
	User   string `json:"user"`
	UserID string `json:"user_id"`
	BotID  string `json:"bot_id"`
	URL    string `json:"url"`
}

// TestAuth verifies the bot token against Slack's auth.test and returns the
// workspace/bot identity it resolves to.
func TestAuth(ctx context.Context, botToken string) (*AuthInfo, error) {
	if botToken == "" {
		return nil, fmt.Errorf("slack: bot token not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/auth.test", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+botToken)
	start := time.Now()
	resp, err := httpClient.Do(req)
	logger.ExternalCall(ctx, "slack", "auth.test", start, err)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		AuthInfo
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("slack: auth.test failed: %s", out.Error)
	}
	return &out.AuthInfo, nil
}
