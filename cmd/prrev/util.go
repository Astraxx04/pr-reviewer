package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseID(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid id %q: must be a positive integer", s)
	}
	return uint(n), nil
}

// parsePRRef parses "owner/repo#42" into its components.
func parsePRRef(ref string) (owner, repo string, number int, err error) {
	hashIdx := strings.LastIndex(ref, "#")
	if hashIdx < 0 {
		return "", "", 0, fmt.Errorf("expected format owner/repo#N, got %q", ref)
	}
	repoPath := ref[:hashIdx]
	numStr := ref[hashIdx+1:]

	slashIdx := strings.Index(repoPath, "/")
	if slashIdx < 0 {
		return "", "", 0, fmt.Errorf("expected format owner/repo#N, got %q", ref)
	}
	owner = repoPath[:slashIdx]
	repo = repoPath[slashIdx+1:]
	if owner == "" || repo == "" {
		return "", "", 0, fmt.Errorf("owner and repo must not be empty in %q", ref)
	}

	number, err = strconv.Atoi(numStr)
	if err != nil || number <= 0 {
		return "", "", 0, fmt.Errorf("invalid PR number %q in %q", numStr, ref)
	}
	return owner, repo, number, nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// fmtTime renders an optional timestamp as a short date, or "-" when nil/zero.
func fmtTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
