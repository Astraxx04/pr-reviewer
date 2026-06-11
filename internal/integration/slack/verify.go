// Package slack implements the two-way Slack bot: request-signature verification,
// PR-reference parsing, and a minimal Slack Web API client for replies.
package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// maxSkew is the largest accepted age of a Slack request timestamp (replay protection).
const maxSkew = 5 * time.Minute

// VerifySignature validates a Slack request signature per
// https://api.slack.com/authentication/verifying-requests-from-slack.
//
// basestring = "v0:" + timestamp + ":" + rawBody, signed with HMAC-SHA256 using the
// signing secret. The expected header value is "v0=" + hex(mac). now is injected so
// callers (and tests) control the clock.
func VerifySignature(signingSecret string, timestamp string, rawBody []byte, signature string, now time.Time) error {
	if signingSecret == "" {
		return fmt.Errorf("slack: signing secret not configured")
	}
	if timestamp == "" || signature == "" {
		return fmt.Errorf("slack: missing signature headers")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("slack: invalid timestamp")
	}
	if d := now.Sub(time.Unix(ts, 0)); d > maxSkew || d < -maxSkew {
		return fmt.Errorf("slack: stale request (possible replay)")
	}

	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte("v0:"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write(rawBody)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("slack: signature mismatch")
	}
	return nil
}
