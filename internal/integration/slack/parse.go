package slack

import "regexp"

// prRefRe matches "owner/repo#123" allowing optional surrounding whitespace and an
// optional leading "<https://github.com/...|...>" Slack auto-link wrapper is stripped
// by the caller before matching.
var prRefRe = regexp.MustCompile(`([\w.-]+)/([\w.-]+)#(\d+)`)

// PRRef identifies a pull request referenced in a Slack command or mention.
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// ParsePRRef extracts the first "owner/repo#N" reference from text. ok is false if none found.
func ParsePRRef(text string) (PRRef, bool) {
	m := prRefRe.FindStringSubmatch(text)
	if m == nil {
		return PRRef{}, false
	}
	n := 0
	for _, c := range m[3] {
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return PRRef{}, false
	}
	return PRRef{Owner: m[1], Repo: m[2], Number: n}, true
}
