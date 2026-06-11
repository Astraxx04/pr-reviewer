package notifications

import (
	"encoding/json"

	dbpkg "pr-reviewer/internal/db"
)

// encryptionKey is the AES key (hex) used to encrypt channel secrets at rest.
// Set once at startup from ENCRYPTION_KEY; empty disables encryption (dev).
var encryptionKey string

// SetEncryptionKey registers the key used to encrypt/decrypt channel secrets.
func SetEncryptionKey(k string) { encryptionKey = k }

// encryptSecret encrypts a plaintext secret for storage. Best-effort: on any
// failure (or no key) it returns the input unchanged.
func encryptSecret(plain string) string {
	if plain == "" || encryptionKey == "" {
		return plain
	}
	if enc, err := dbpkg.Encrypt(plain, encryptionKey); err == nil {
		return enc
	}
	return plain
}

// DecryptSecret reverses encryptSecret. If the value isn't decryptable (e.g. a
// legacy plaintext row written before encryption was added, or no key), it is
// returned as-is so existing configs keep working.
func DecryptSecret(enc string) string {
	if enc == "" || encryptionKey == "" {
		return enc
	}
	if dec, err := dbpkg.Decrypt(enc, encryptionKey); err == nil {
		return dec
	}
	return enc
}

// PrepareConfigForStore encrypts a channel config's secret fields before it is
// persisted. For updates, pass the previously stored (already-encrypted) config
// as existing so a blank secret preserves the stored value rather than wiping it;
// pass nil on create. Non-secret fields and unknown channels pass through intact.
func PrepareConfigForStore(channel string, incoming, existing []byte) ([]byte, error) {
	switch channel {
	case "email":
		var in EmailChannelConfig
		if err := json.Unmarshal(incoming, &in); err != nil {
			return nil, err
		}
		if in.SMTPPassword == "" {
			in.SMTPPassword = existingEmailPassword(existing)
		} else {
			in.SMTPPassword = encryptSecret(in.SMTPPassword)
		}
		return json.Marshal(in)
	case "webhook":
		var in WebhookChannelConfig
		if err := json.Unmarshal(incoming, &in); err != nil {
			return nil, err
		}
		if in.Secret == "" {
			in.Secret = existingWebhookSecret(existing)
		} else {
			in.Secret = encryptSecret(in.Secret)
		}
		return json.Marshal(in)
	default:
		return incoming, nil
	}
}

func existingEmailPassword(existing []byte) string {
	if len(existing) == 0 {
		return ""
	}
	var ex EmailChannelConfig
	if json.Unmarshal(existing, &ex) == nil {
		return ex.SMTPPassword
	}
	return ""
}

func existingWebhookSecret(existing []byte) string {
	if len(existing) == 0 {
		return ""
	}
	var ex WebhookChannelConfig
	if json.Unmarshal(existing, &ex) == nil {
		return ex.Secret
	}
	return ""
}

// RedactConfig strips secret fields from a channel config for API responses and
// adds a boolean "<field>_set" flag so the UI can show whether a secret is
// stored without ever returning it. Other fields are preserved verbatim.
func RedactConfig(channel string, cfgJSON []byte) []byte {
	var secretField string
	switch channel {
	case "email":
		secretField = "smtp_password"
	case "webhook":
		secretField = "secret"
	default:
		return cfgJSON
	}
	var m map[string]any
	if json.Unmarshal(cfgJSON, &m) != nil {
		return cfgJSON
	}
	v, present := m[secretField]
	delete(m, secretField)
	m[secretField+"_set"] = present && v != nil && v != ""
	out, err := json.Marshal(m)
	if err != nil {
		return cfgJSON
	}
	return out
}
