package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// WebhookPayload is the JSON body POSTed to an outbound webhook endpoint.
type WebhookPayload struct {
	Event     string         `json:"event"`
	PR        map[string]any `json:"pr"`
	Review    map[string]any `json:"review,omitempty"`
	Assignees []string       `json:"assignees,omitempty"`
	Score     int            `json:"score,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// PostWebhook delivers payload to url with HMAC-SHA256 signing, retrying up to 3 times.
func PostWebhook(ctx context.Context, url, secret string, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sig := ""
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sig = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	for attempt := range 3 {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if sig != "" {
			req.Header.Set("X-PR-Reviewer-Signature", sig)
		}
		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		logger.ExternalCall(ctx, "outbound-webhook", "POST", start, err, "url", url, "attempt", attempt+1)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 300 {
				return nil
			}
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}
	return fmt.Errorf("webhook delivery to %s failed after 3 attempts", url)
}
