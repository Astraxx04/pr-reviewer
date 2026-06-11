package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	gh "pr-reviewer/internal/github"
)

// Evaluate runs all rules against the diff and returns formatted violation strings.
// Only added lines (patch lines starting with "+") are checked.
func Evaluate(cfg *Config, diff []gh.FileDiff) []string {
	if cfg == nil || len(cfg.Rules) == 0 {
		return nil
	}

	// Compile each rule's regex once up front rather than per file. Rules with an
	// invalid pattern are skipped here so a single bad pattern can't abort the run.
	type compiledRule struct {
		rule Rule
		re   *regexp.Regexp
	}
	compiled := make([]compiledRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, compiledRule{rule: rule, re: re})
	}
	if len(compiled) == 0 {
		return nil
	}

	var violations []string
	for _, file := range diff {
		if ShouldIgnore(file.Filename, cfg.Ignore) {
			continue
		}
		if file.Patch == "" {
			continue
		}
		lines := strings.Split(file.Patch, "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "+") {
				continue
			}
			for _, cr := range compiled {
				if cr.re.MatchString(line) {
					violations = append(violations, fmt.Sprintf(
						"[%s] Rule %q: %s — in %s (patch line %d)",
						cr.rule.Severity, cr.rule.ID, cr.rule.Description, file.Filename, i+1,
					))
				}
			}
		}
	}
	return violations
}

// ShouldIgnore returns true if the filename matches any of the ignore patterns.
// Exported so review_job.go can use it for .pr-reviewer-ignore patterns.
func ShouldIgnore(filename string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(filename)); matched {
			return true
		}
	}
	return false
}
