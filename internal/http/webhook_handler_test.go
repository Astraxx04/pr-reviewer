package http_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Astraxx04/pr-reviewer/internal/ai"
	gh "github.com/Astraxx04/pr-reviewer/internal/github"
	prHttp "github.com/Astraxx04/pr-reviewer/internal/http"
	"github.com/Astraxx04/pr-reviewer/internal/pr"
	"github.com/Astraxx04/pr-reviewer/internal/review"
	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

const secret = "test-secret"

// --- mocks ---

type mockPRService struct{}

func (m *mockPRService) BuildContext(_ context.Context, owner, repo string, number int, action string) (*pr.PRContext, error) {
	return &pr.PRContext{Repo: owner + "/" + repo, Number: number, Title: "Test PR", Action: action}, nil
}

type mockAIService struct{}

func (m *mockAIService) Review(_ context.Context, _ ai.AnalysisRequest) (*ai.ReviewResult, error) {
	return &ai.ReviewResult{Summary: "LGTM", Score: 100}, nil
}

func (m *mockAIService) Explain(_ context.Context, _, _ string) (string, error) { return "", nil }

type mockAggregator struct{}

func (m *mockAggregator) Aggregate(_ context.Context, _ []ai.ReviewResult) (*review.FinalReview, error) {
	return &review.FinalReview{Summary: "LGTM", Status: "APPROVE"}, nil
}

type mockGHClient struct{}

func (m *mockGHClient) GetPullRequest(_ context.Context, _, _ string, _ int) (*gh.PullRequest, error) {
	return &gh.PullRequest{}, nil
}

func (m *mockGHClient) ListRepoCollaborators(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockGHClient) GetPullRequestDiff(_ context.Context, _, _ string, _ int) ([]gh.FileDiff, error) {
	return nil, nil
}
func (m *mockGHClient) PostComment(_ context.Context, _, _ string, _ int, _ *gh.ReviewComment) error {
	return nil
}
func (m *mockGHClient) PostReview(_ context.Context, _, _ string, _ int, _ *gh.ReviewSubmission) (int64, error) {
	return 0, nil
}
func (m *mockGHClient) GetReviewCommentsByReview(_ context.Context, _, _ string, _ int, _ int64) ([]gh.ReviewCommentRef, error) {
	return nil, nil
}
func (m *mockGHClient) PostReviewCommentReply(_ context.Context, _, _ string, _ int, _ int64, _ string) error {
	return nil
}
func (m *mockGHClient) PostSummaryComment(_ context.Context, _, _ string, _ int, _ *gh.ReviewSubmission, _ int) error {
	return nil
}
func (m *mockGHClient) GetCODEOWNERS(_ context.Context, _, _ string) ([]gh.CODEOWNERSRule, error) {
	return nil, nil
}
func (m *mockGHClient) RequestReviewers(_ context.Context, _, _ string, _ int, _ []string) error {
	return nil
}
func (m *mockGHClient) GetFileContent(_ context.Context, _, _, _ string) (string, error) {
	return "", nil
}
func (m *mockGHClient) EnsureLabel(_ context.Context, _, _, _, _, _ string) error { return nil }
func (m *mockGHClient) AddLabelsToIssue(_ context.Context, _, _ string, _ int, _ []string) error {
	return nil
}
func (m *mockGHClient) GetRepoTreeEntries(_ context.Context, _, _, _ string) ([]gh.TreeEntry, error) {
	return nil, nil
}
func (m *mockGHClient) GetDefaultBranchSHA(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockGHClient) CreateStatus(_ context.Context, _, _, _ string, _ *gh.CommitStatus) error {
	return nil
}

// --- helpers ---

// newHandler builds an InProcessWebhookHandler (no DB/River dependency) for unit tests.
func newHandler() prHttp.WebhookHandlerIface {
	log := logger.New("test")
	return prHttp.NewInProcessWebhookHandler(log, &mockPRService{}, &mockAIService{}, &mockAggregator{}, &mockGHClient{}, secret)
}

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// --- tests ---

func TestHandle_InvalidSignature(t *testing.T) {
	body := []byte(`{"action":"opened","number":1,"pull_request":{"base":{"repo":{"owner":{"login":"o"},"name":"r"}}}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=badhash")

	w := httptest.NewRecorder()
	newHandler().Handle(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandle_NonPRAction(t *testing.T) {
	body := []byte(`{"action":"labeled","number":1,"pull_request":{"base":{"repo":{"owner":{"login":"o"},"name":"r"}}}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body))

	w := httptest.NewRecorder()
	newHandler().Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for non-PR action, got %d", w.Code)
	}
}

func TestHandle_OpenedPR_Returns202(t *testing.T) {
	body := []byte(`{"action":"opened","number":42,"pull_request":{"base":{"repo":{"owner":{"login":"owner"},"name":"repo"}}}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body))

	w := httptest.NewRecorder()
	newHandler().Handle(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
}

func TestHandle_SynchronizePR_Returns202(t *testing.T) {
	body := []byte(`{"action":"synchronize","number":7,"pull_request":{"base":{"repo":{"owner":{"login":"o"},"name":"r"}}}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body))

	w := httptest.NewRecorder()
	newHandler().Handle(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
}
