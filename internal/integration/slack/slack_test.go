package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func validSig(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + ts + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	secret := "8f742231b10e8888abcd99yyyzzz85a5"
	now := time.Unix(1_700_000_000, 0)
	ts := "1700000000"
	body := []byte("token=abc&command=/review&text=acme/web%231")
	good := validSig(secret, ts, body)

	if err := VerifySignature(secret, ts, body, good, now); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifySignature(secret, ts, body, "v0=deadbeef", now); err == nil {
		t.Fatal("expected mismatch error for bad signature")
	}
	if err := VerifySignature(secret, ts, body, good, now.Add(10*time.Minute)); err == nil {
		t.Fatal("expected stale-request error for old timestamp")
	}
	if err := VerifySignature("", ts, body, good, now); err == nil {
		t.Fatal("expected error when signing secret is empty")
	}
}

func TestParsePRRef(t *testing.T) {
	cases := []struct {
		text   string
		ok     bool
		owner  string
		repo   string
		number int
	}{
		{"acme/web#42", true, "acme", "web", 42},
		{"please review octo-org/my.repo#7 thanks", true, "octo-org", "my.repo", 7},
		{"<@U123> look at acme/web#100", true, "acme", "web", 100},
		{"no ref here", false, "", "", 0},
		{"acme/web#0", false, "", "", 0},
	}
	for _, c := range cases {
		ref, ok := ParsePRRef(c.text)
		if ok != c.ok {
			t.Errorf("%q: ok = %v, want %v", c.text, ok, c.ok)
			continue
		}
		if ok && (ref.Owner != c.owner || ref.Repo != c.repo || ref.Number != c.number) {
			t.Errorf("%q: got %+v, want %s/%s#%d", c.text, ref, c.owner, c.repo, c.number)
		}
	}
}
