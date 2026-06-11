package ai

import (
	"bytes"
	"text/template"
)

// PromptTemplate represents a reusable prompt.
type PromptTemplate string

const (
	SystemPrompt PromptTemplate = `You are a senior engineer reviewing a pull request.
Focus on: correctness, performance, clean architecture, and security.`

	ReviewPrompt PromptTemplate = `PR Title: {{.Title}}
{{- if .Body}}

PR Description:
{{.Body}}
{{- end}}
{{- if .TicketContext}}

Linked Jira tickets — this is what the PR is meant to accomplish. Use it to judge whether the
change actually does what the ticket asks: if the PR diverges from the ticket's intent, misses
described requirements, or does substantially more than requested, call that out. Don't repeat
ticket text verbatim in comments.
{{.TicketContext}}
{{- end}}
{{- if .PRTemplate}}

PR Template (check that the description covers all required sections):
{{.PRTemplate}}
{{- end}}
{{- if .RAGContext}}

Similar findings from past reviews of this repository:
{{.RAGContext}}
{{- end}}
{{- if .FalsePositives}}

The following patterns were previously marked as false positives — do NOT flag these:
{{.FalsePositives}}
{{- end}}
{{- if .CustomViolations}}

Custom rule violations already found in this diff (include these as comments):
{{.CustomViolations}}
{{- end}}
{{- if .DiffTruncated}}

NOTE: This diff exceeded the maximum allowed size. Review is based on file names and PR description only. Do not flag specific line numbers.
{{- else}}

Changes:
{{.Diff}}
{{- end}}`
)

// Render substitutes template variables using Go's text/template.
func (p PromptTemplate) Render(data map[string]interface{}) string {
	tmpl, err := template.New("").Parse(string(p))
	if err != nil {
		return string(p)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return string(p)
	}
	return buf.String()
}
