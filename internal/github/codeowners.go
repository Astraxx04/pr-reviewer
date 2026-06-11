package github

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"pr-reviewer/pkg/logger"
)

type CODEOWNERSRule struct {
	Pattern string
	Owners  []string
}

// GetCODEOWNERS fetches and parses the CODEOWNERS file from the repo.
// Tries .github/CODEOWNERS, CODEOWNERS, docs/CODEOWNERS in order.
// Returns nil, nil if no file is found.
func (c *clientImpl) GetCODEOWNERS(ctx context.Context, owner, repo string) ([]CODEOWNERSRule, error) {
	for _, path := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		start := time.Now()
		content, _, _, err := c.client.Repositories.GetContents(ctx, owner, repo, path, nil)
		if err != nil {
			// A missing CODEOWNERS file is expected; try the next candidate path.
			continue
		}
		logger.ExternalCall(ctx, "github", "Repositories.GetContents", start, nil, "owner", owner, "repo", repo, "path", path)
		// GetContent() handles base64 decoding automatically.
		raw, err := content.GetContent()
		if err != nil {
			continue
		}
		return parseCODEOWNERS(raw), nil
	}
	return nil, nil
}

func parseCODEOWNERS(content string) []CODEOWNERSRule {
	var rules []CODEOWNERSRule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		var owners []string
		for _, o := range parts[1:] {
			if strings.HasPrefix(o, "@") {
				owners = append(owners, strings.TrimPrefix(o, "@"))
			}
		}
		if len(owners) > 0 {
			rules = append(rules, CODEOWNERSRule{Pattern: parts[0], Owners: owners})
		}
	}
	return rules
}

// MatchOwners returns unique owner logins matching the given file paths.
// Last matching rule wins, per CODEOWNERS spec.
func MatchOwners(rules []CODEOWNERSRule, files []string) []string {
	seen := make(map[string]bool)
	var owners []string
	for _, file := range files {
		for i := len(rules) - 1; i >= 0; i-- {
			if matchCOPattern(rules[i].Pattern, file) {
				for _, o := range rules[i].Owners {
					if !seen[o] {
						seen[o] = true
						owners = append(owners, o)
					}
				}
				break
			}
		}
	}
	return owners
}

func matchCOPattern(pattern, filePath string) bool {
	p := strings.TrimPrefix(pattern, "/")

	if strings.HasSuffix(p, "/") {
		return strings.HasPrefix(filePath, p)
	}
	if p == "**" {
		return true
	}
	if !strings.Contains(p, "/") {
		matched, _ := filepath.Match(p, filepath.Base(filePath))
		return matched
	}
	if strings.Contains(p, "**") {
		return matchDoubleGlob(p, filePath)
	}
	matched, _ := filepath.Match(p, filePath)
	return matched
}

func matchDoubleGlob(pattern, filePath string) bool {
	halves := strings.SplitN(pattern, "**", 2)
	prefix := halves[0]
	suffix := strings.TrimPrefix(halves[1], "/")

	if prefix != "" && !strings.HasPrefix(filePath, prefix) {
		return false
	}
	remainder := filePath[len(prefix):]
	if suffix == "" {
		return true
	}
	if matched, _ := filepath.Match(suffix, remainder); matched {
		return true
	}
	return strings.HasSuffix(remainder, "/"+suffix) || strings.Contains(remainder, "/"+suffix+"/")
}
