package rules

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	gh "github.com/Astraxx04/pr-reviewer/internal/github"
)

// Rule defines a custom pattern to flag in diffs.
type Rule struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Pattern     string `yaml:"pattern"`
	Severity    string `yaml:"severity"` // p0|p1|p2|p3
}

// Config is the parsed .pr-reviewer.yml file.
type Config struct {
	Rules  []Rule   `yaml:"rules"`
	Ignore []string `yaml:"ignore"`
}

// FetchAndParse fetches and parses .pr-reviewer.yml from the repo.
// Returns nil, nil if the file does not exist.
func FetchAndParse(ctx context.Context, client gh.Client, owner, repo string) (*Config, error) {
	content, err := client.GetFileContent(ctx, owner, repo, ".pr-reviewer.yml")
	if err != nil {
		return nil, nil // file not found is expected
	}
	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("rules: parse .pr-reviewer.yml: %w", err)
	}
	return &cfg, nil
}
