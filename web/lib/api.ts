const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "";

// On an auth failure (expired/revoked session, missing or invalid token) clear
// the stored token and bounce to /login, so a stale session sends the user to
// re-authenticate instead of surfacing an uncaught rejection on every request.
// Guarded to the browser — server-side callers just see the thrown error — and
// skipped when already on /login to avoid a redirect loop. The cookie name is
// inlined (rather than importing the "use client" auth module) so this file
// stays safe to import from server components.
function handleAuthFailure(): void {
  if (typeof window === "undefined") return;
  document.cookie = "pr_reviewer_token=; path=/; max-age=0";
  if (window.location.pathname !== "/login") {
    window.location.href = "/login";
  }
}

async function apiFetch<T>(
  path: string,
  options: RequestInit & { token?: string } = {}
): Promise<T> {
  const { token, ...init } = options;
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "ngrok-skip-browser-warning": "true",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    if (res.status === 401) handleAuthFailure();
    throw new Error(err.error ?? "API error");
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

// --- repos ---
export function listRepos(token: string) {
  return apiFetch<Repo[]>("/api/repos", { token });
}
export function updateRepo(token: string, id: number, body: Partial<{ enabled: boolean }>) {
  return apiFetch<Repo>(`/api/repos/${id}`, { method: "PATCH", token, body: JSON.stringify(body) });
}
export function syncRepos(token: string) {
  return apiFetch<{ synced: number; added: number }>("/api/repos/sync", { method: "POST", token });
}

// --- github app settings ---
export function getGithubApp(token: string) {
  return apiFetch<GithubAppStatus>("/api/settings/github-app", { token });
}
export function putGithubApp(token: string, body: { app_id: number; private_key: string; webhook_secret?: string; github_token?: string }) {
  return apiFetch<GithubAppStatus>("/api/settings/github-app", {
    method: "PUT", token, body: JSON.stringify(body),
  });
}
export function testGithubApp(token: string) {
  return apiFetch<{ ok: boolean; message: string }>("/api/settings/github-app/test", { method: "POST", token });
}
export function deleteGithubApp(token: string) {
  return apiFetch<GithubAppStatus>("/api/settings/github-app", { method: "DELETE", token });
}
export function getRepoConfig(token: string, id: number) {
  return apiFetch<{ config: Record<string, unknown> }>(`/api/repos/${id}/config`, { token });
}
export function putRepoConfig(token: string, id: number, config: Record<string, unknown>) {
  return apiFetch<{ config: Record<string, unknown> }>(`/api/repos/${id}/config`, {
    method: "PUT", token, body: JSON.stringify(config),
  });
}

// --- reviews ---
export function listReviews(token: string, page = 1, perPage = 20) {
  return apiFetch<ReviewList>(`/api/reviews?page=${page}&per_page=${perPage}`, { token });
}
export function getReview(token: string, id: number) {
  return apiFetch<ReviewDetail>(`/api/reviews/${id}`, { token });
}

// --- dashboard ---
export function getDashboardStats(token: string) {
  return apiFetch<DashboardStats>("/api/dashboard/stats", { token });
}

// --- team ---
export function listTeam(token: string) {
  return apiFetch<TeamMember[]>("/api/team", { token });
}
export function addTeamMember(token: string, login: string, role = "reviewer", email?: string) {
  return apiFetch<TeamMember>("/api/team/members", {
    method: "POST", token, body: JSON.stringify({ login, role, email: email || undefined }),
  });
}
export function updateTeamMember(token: string, id: number, role: string) {
  return apiFetch<void>(`/api/team/members/${id}`, {
    method: "PATCH", token, body: JSON.stringify({ role }),
  });
}
export function removeTeamMember(token: string, id: number) {
  return apiFetch<void>(`/api/team/members/${id}`, { method: "DELETE", token });
}

// --- analytics ---
export function getAnalytics(token: string, days = 30) {
  return apiFetch<AnalyticsData>(`/api/analytics?days=${days}`, { token });
}

// --- providers ---
export function listProviders(token: string) {
  return apiFetch<Provider[]>("/api/providers", { token });
}
export function createProvider(token: string, body: CreateProviderBody) {
  return apiFetch<{ id: number }>("/api/providers", { method: "POST", token, body: JSON.stringify(body) });
}
export function listProviderModels(
  token: string,
  body: { id?: number; type?: string; base_url?: string; api_key?: string },
) {
  return apiFetch<{ ok: boolean; message?: string; models: { id: string; display_name?: string }[] }>(
    "/api/providers/models",
    { method: "POST", token, body: JSON.stringify(body) },
  );
}
export function updateProvider(token: string, id: number, body: Partial<CreateProviderBody>) {
  return apiFetch<{ id: number }>(`/api/providers/${id}`, { method: "PUT", token, body: JSON.stringify(body) });
}
export function deleteProvider(token: string, id: number) {
  return apiFetch<void>(`/api/providers/${id}`, { method: "DELETE", token });
}
export function testProvider(token: string, id: number) {
  return apiFetch<{ ok: boolean; message: string }>(`/api/providers/${id}/test`, { method: "POST", token });
}

// --- setup ---
export function getSetupStatus() {
  return apiFetch<SetupStatus>("/api/setup/status");
}
export function completeSetup() {
  return apiFetch<{ ok: boolean }>("/api/setup/complete", { method: "POST" });
}

// --- PRs ---
export function listPRs(token: string, params: { page?: number; per_page?: number; repo?: string; author?: string; status?: string } = {}) {
  const q = new URLSearchParams();
  if (params.page) q.set("page", String(params.page));
  if (params.per_page) q.set("per_page", String(params.per_page));
  if (params.repo) q.set("repo", params.repo);
  if (params.author) q.set("author", params.author);
  if (params.status) q.set("status", params.status);
  return apiFetch<PRList>(`/api/prs?${q}`, { token });
}
export function getPR(token: string, owner: string, repo: string, number: number) {
  return apiFetch<PRDetail>(`/api/prs/${owner}/${repo}/${number}`, { token });
}
export function requestReReview(token: string, owner: string, repo: string, number: number) {
  return apiFetch<{ ok: boolean }>(`/api/prs/${owner}/${repo}/${number}/re-review`, { method: "POST", token });
}
export function getPRDiff(token: string, owner: string, repo: string, number: number) {
  return apiFetch<FileDiff[]>(`/api/prs/${owner}/${repo}/${number}/diff`, { token });
}

// --- system metrics ---
export function getSystemMetrics(token: string) {
  return apiFetch<SystemMetrics>("/api/metrics/system", { token });
}

// --- cost analytics ---
export function getCostAnalytics(token: string, days = 30) {
  return apiFetch<CostAnalytics>(`/api/analytics/cost?days=${days}`, { token });
}

// --- provider health ---
export function getProviderHealth(token: string) {
  return apiFetch<ProviderHealthEntry[]>("/api/providers/health", { token });
}

// --- webhook deliveries ---
export function listWebhookDeliveries(token: string, page = 1, perPage = 50) {
  return apiFetch<WebhookDeliveryList>(`/api/webhooks/deliveries?page=${page}&per_page=${perPage}`, { token });
}

// --- notification configs ---
export type NotificationChannel = "slack" | "email" | "webhook";

export interface SlackConfig {
  webhook_url: string;
  events: string[];
  score_threshold: number;
  template: string;
}

export interface EmailConfig {
  smtp_host?: string;
  smtp_port?: number;
  smtp_username?: string;
  smtp_password?: string; // write-only; never returned. blank on update = keep stored
  smtp_password_set?: boolean; // read-only flag: whether a password is stored
  from?: string;
  to: string[];
  events: string[];
  digest: "none" | "daily" | "weekly";
  template: string;
  score_threshold?: number;
}

export interface WebhookConfig {
  url: string;
  secret?: string; // write-only; never returned. blank on update = keep stored
  secret_set?: boolean; // read-only flag: whether a secret is stored
  events: string[];
  template: string;
  score_threshold?: number;
}

export interface NotificationConfig {
  ID: number;
  InstallationID: number;
  RepoID: number | null;
  Channel: NotificationChannel;
  Config: SlackConfig | EmailConfig | WebhookConfig;
  Enabled: boolean;
  CreatedAt: string;
  UpdatedAt: string;
}

export function listNotificationConfigs(token: string) {
  return apiFetch<NotificationConfig[]>("/api/settings/notifications", { token });
}
export function createNotificationConfig(
  token: string,
  body: { channel: NotificationChannel; config: SlackConfig | EmailConfig | WebhookConfig; repo_id?: number; enabled?: boolean }
) {
  return apiFetch<NotificationConfig>("/api/settings/notifications", {
    method: "POST", token, body: JSON.stringify(body),
  });
}
export function updateNotificationConfig(
  token: string,
  id: number,
  body: { channel?: NotificationChannel; config?: SlackConfig | EmailConfig | WebhookConfig; enabled?: boolean }
) {
  return apiFetch<NotificationConfig>(`/api/settings/notifications/${id}`, {
    method: "PUT", token, body: JSON.stringify(body),
  });
}
export function deleteNotificationConfig(token: string, id: number) {
  return apiFetch<void>(`/api/settings/notifications/${id}`, { method: "DELETE", token });
}
export function testNotificationConfig(token: string, id: number) {
  return apiFetch<{ ok: boolean; error?: string }>(`/api/settings/notifications/${id}/test`, { method: "POST", token });
}
export function triggerDigest(token: string, period: "daily" | "weekly" = "daily") {
  return apiFetch<{ ok: boolean; period: string }>(`/api/settings/notifications/digest/trigger?period=${period}`, {
    method: "POST",
    token,
  });
}

// --- export ---
export function exportReviewsCSVUrl(token: string, params: { start?: string; end?: string; repo?: string } = {}): string {
  const q = new URLSearchParams();
  if (params.start) q.set("start", params.start);
  if (params.end) q.set("end", params.end);
  if (params.repo) q.set("repo", params.repo);
  return `/api/reviews/export?${q}`;
}

// downloadReviewsPDF fetches the PDF report with the auth header and triggers a browser
// download. Used instead of window.open so the Bearer token reaches the protected route.
export async function downloadReviewsPDF(
  token: string,
  params: { start?: string; end?: string; repo?: string } = {}
): Promise<void> {
  const q = new URLSearchParams();
  if (params.start) q.set("start", params.start);
  if (params.end) q.set("end", params.end);
  if (params.repo) q.set("repo", params.repo);
  const res = await fetch(`${API_BASE}/api/reviews/export.pdf?${q}`, {
    headers: {
      "ngrok-skip-browser-warning": "true",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });
  if (!res.ok) {
    if (res.status === 401) handleAuthFailure();
    throw new Error("PDF export failed");
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `pr-reviewer-report_${new Date().toISOString().slice(0, 10)}.pdf`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// --- comment feedback ---
export interface CommentFeedbackSummary {
  up: number;
  down: number;
  my_vote: 1 | -1 | 0;
}

export function getCommentFeedback(token: string, commentID: number) {
  return apiFetch<CommentFeedbackSummary>(`/api/reviews/comments/${commentID}/feedback`, { token });
}

export function submitCommentFeedback(token: string, commentID: number, vote: 1 | -1) {
  return apiFetch<CommentFeedbackSummary>(`/api/reviews/comments/${commentID}/feedback`, {
    method: "POST",
    token,
    body: JSON.stringify({ vote }),
  });
}

export function explainComment(token: string, commentID: number) {
  return apiFetch<{ explanation: string }>(`/api/reviews/comments/${commentID}/explain`, {
    method: "POST",
    token,
  });
}

// --- audit log ---
export interface AuditLogEntry {
  ID: number;
  ActorLogin: string;
  ActorID: number;
  Action: string;
  EntityType: string;
  EntityID: string;
  Before: unknown;
  After: unknown;
  IPAddress: string;
  CreatedAt: string;
}

export interface AuditLogList {
  logs: AuditLogEntry[];
  total: number;
  page: number;
  per_page: number;
}

export function listAuditLogs(
  token: string,
  params: { page?: number; per_page?: number; entity_type?: string; actor?: string; since?: string } = {}
) {
  const q = new URLSearchParams();
  if (params.page) q.set("page", String(params.page));
  if (params.per_page) q.set("per_page", String(params.per_page));
  if (params.entity_type) q.set("entity_type", params.entity_type);
  if (params.actor) q.set("actor", params.actor);
  if (params.since) q.set("since", params.since);
  return apiFetch<AuditLogList>(`/api/audit?${q}`, { token });
}

export function auditExportCSVUrl(): string {
  return `/api/audit/export`;
}

// --- API tokens ---
export interface APIToken {
  ID: number;
  Name: string;
  Scope: string;
  Prefix: string;
  LastUsedAt: string | null;
  ExpiresAt: string | null;
  CreatedAt: string;
}

export interface CreatedAPIToken extends APIToken {
  token: string;
}

export function listAPITokens(token: string) {
  return apiFetch<APIToken[]>("/api/tokens", { token });
}

export function createAPIToken(
  token: string,
  body: { name: string; scope: "read" | "readwrite"; expires_at?: string }
) {
  return apiFetch<CreatedAPIToken>("/api/tokens", {
    method: "POST",
    token,
    body: JSON.stringify(body),
  });
}

export function revokeAPIToken(token: string, id: number) {
  return apiFetch<void>(`/api/tokens/${id}`, { method: "DELETE", token });
}

// --- data retention ---
export interface RetentionSettings {
  review_retention_days: number;
  purge_embeddings_on_disable: boolean;
}

export function getRetentionSettings(token: string) {
  return apiFetch<RetentionSettings>("/api/settings/retention", { token });
}

export function putRetentionSettings(token: string, body: RetentionSettings) {
  return apiFetch<void>("/api/settings/retention", {
    method: "PUT",
    token,
    body: JSON.stringify(body),
  });
}

export function eraseUserData(token: string, login: string) {
  return apiFetch<void>(`/api/users/${encodeURIComponent(login)}/data`, {
    method: "DELETE",
    token,
  });
}

// --- Jira integration ---
export interface JiraConfigStatus {
  id: number;
  base_url: string;
  email: string;
  enabled: boolean;
  configured: boolean;
  created_at: string;
  updated_at: string;
}

export function getJiraConfig(token: string) {
  return apiFetch<JiraConfigStatus>("/api/settings/integrations/jira", { token });
}

export function putJiraConfig(
  token: string,
  body: { base_url: string; email: string; api_token?: string; enabled?: boolean }
) {
  return apiFetch<void>("/api/settings/integrations/jira", {
    method: "PUT",
    token,
    body: JSON.stringify(body),
  });
}

export function deleteJiraConfig(token: string) {
  return apiFetch<void>("/api/settings/integrations/jira", { method: "DELETE", token });
}

export function testJiraConfig(token: string) {
  return apiFetch<{ ok: boolean; error?: string; display_name?: string; email?: string }>(
    "/api/settings/integrations/jira/test",
    { method: "POST", token },
  );
}

// --- Slack bot (two-way) ---
export interface SlackAppStatus {
  configured: boolean;
  enabled?: boolean;
  has_bot_token?: boolean;
  has_signing_key?: boolean;
  created_at?: string;
  updated_at?: string;
  server_url?: string; // the server's public base URL (from SERVER_URL) — what Slack must reach
}

export function getSlackApp(token: string) {
  return apiFetch<SlackAppStatus>("/api/settings/slack-app", { token });
}

export function putSlackApp(
  token: string,
  body: { signing_secret?: string; bot_token?: string; enabled?: boolean }
) {
  return apiFetch<void>("/api/settings/slack-app", {
    method: "PUT",
    token,
    body: JSON.stringify(body),
  });
}

export function deleteSlackApp(token: string) {
  return apiFetch<void>("/api/settings/slack-app", { method: "DELETE", token });
}

export function testSlackApp(token: string) {
  return apiFetch<{ ok: boolean; error?: string; team?: string; team_id?: string; user?: string; bot_id?: string; url?: string }>(
    "/api/settings/slack-app/test",
    { method: "POST", token },
  );
}

// --- SSO / OIDC ---
export interface SSOConfig {
  ID: number;
  Issuer: string;
  ClientID: string;
  RedirectURL: string;
  AttributeMapping: Record<string, string>;
  RoleMapping: Record<string, string>;
  Enforced: boolean;
  Enabled: boolean;
}

export function getSSOConfig(token: string) {
  return apiFetch<SSOConfig>("/api/settings/sso", { token });
}

export function putSSOConfig(
  token: string,
  body: {
    issuer: string;
    client_id: string;
    client_secret?: string;
    redirect_url?: string;
    attribute_mapping?: Record<string, string>;
    role_mapping?: Record<string, string>;
    enforced?: boolean;
    enabled?: boolean;
  }
) {
  return apiFetch<void>("/api/settings/sso", {
    method: "PUT",
    token,
    body: JSON.stringify(body),
  });
}

export function deleteSSOConfig(token: string) {
  return apiFetch<void>("/api/settings/sso", { method: "DELETE", token });
}

// --- types ---
export interface FileDiff {
  filename: string;
  status: "added" | "modified" | "removed" | "renamed" | string;
  patch: string;
  additions: number;
  deletions: number;
}

export interface Repo {
  ID: number;
  Owner: string;
  Name: string;
  Enabled: boolean;
  IndexingStatus: "idle" | "indexing" | "indexed" | "error";
  CreatedAt: string;
}

export function triggerRepoIndex(token: string, id: number) {
  return apiFetch<{ ok: boolean }>(`/api/repos/${id}/index`, { method: "POST", token });
}

export interface ReviewList {
  reviews: ReviewSummary[];
  total: number;
  page: number;
  per_page: number;
}

export interface ReviewSummary {
  ID: number;
  Status: string;
  Score: number;
  Summary: string;
  CreatedAt: string;
}

export interface ReviewDetail extends ReviewSummary {
  Comments: ReviewComment[];
  Assignments: Assignment[];
}

export interface ReviewComment {
  ID: number;
  Path: string;
  Line: number;
  Side: string;
  Body: string;
  Severity: string;
}

export interface Assignment {
  ID: number;
  AssigneeLogin: string;
  AssignedAt: string;
  CompletedAt: string | null;
}

export interface DashboardStats {
  total_reviews: number;
  avg_score: number;
  approvals: number;
  request_changes: number;
  comments: number;
  total_repos: number;
  enabled_repos: number;
}

export interface TeamMember {
  ID: number;
  Login: string;
  Role: string;
  CreatedAt: string;
}

export interface AnalyticsData {
  series: { date: string; count: number; avg_score: number }[];
  days: number;
}

export interface Provider {
  id: number;
  name: string;
  type: string;
  base_url: string;
  default_model: string;
  supports_embeddings: boolean;
  embedding_model: string;
  has_api_key: boolean;
}

export interface CreateProviderBody {
  name: string;
  type: string;
  api_key?: string;
  base_url?: string;
  default_model?: string;
  supports_embeddings?: boolean;
  embedding_model?: string;
}

export interface GithubAppStatus {
  configured: boolean;
  app_id?: number;
  has_webhook_secret: boolean;
  has_github_token: boolean;
}

export interface SetupStatus {
  database_ok: boolean;
  github_configured: boolean;
  setup_complete: boolean;
}

export type PRStatus = "APPROVED" | "CHANGES_REQUESTED" | "COMMENTED" | "PENDING";

export interface PRSummary {
  id: number;
  number: number;
  title: string;
  author: string;
  repo: string;
  head_sha: string;
  pr_status: PRStatus;
  current_score: number;
  review_count: number;
  last_reviewed_at: string | null;
  created_at: string;
}

export interface PRList {
  prs: PRSummary[];
  total: number;
  page: number;
  per_page: number;
}

export interface PRReviewSummary {
  id: number;
  status: string;
  score: number;
  summary: string;
  comment_count: number;
  created_at: string;
}

export interface PRComment {
  id: number;
  path: string;
  line: number;
  side: string;
  body: string;
  severity: string;
  priority: string;
  has_reply: boolean;
}

export interface PRDetail {
  id: number;
  number: number;
  title: string;
  author: string;
  repo: string;
  head_sha: string;
  pr_status: PRStatus;
  reviews: PRReviewSummary[];
  latest_comments: PRComment[];
  assignees: string[];
  created_at: string;
}

export interface SystemMetrics {
  queue_depth: number;
  reviews_today: number;
  reviews_week: number;
  reviews_month: number;
  latency_p50_ms: number;
  latency_p95_ms: number;
  latency_p99_ms: number;
  webhook_errors_24h: number;
  webhook_total_24h: number;
}

export interface CostAnalytics {
  days: number;
  input_tokens: number;
  output_tokens: number;
  est_cost_usd: number;
  by_repo: { repo: string; input_tokens: number; output_tokens: number; est_cost_usd: number }[];
}

export interface ProviderHealthEntry {
  provider_id: number;
  provider_name: string;
  provider_type: string;
  last_tested_at: string | null;
  latency_ms: number | null;
  ok: boolean | null;
  error_msg?: string;
  status: "healthy" | "degraded" | "unreachable" | "untested";
}

export interface WebhookDelivery {
  DeliveryID: string;
  ProcessedAt: string;
  EventType: string;
  Action: string;
  Owner: string;
  Repo: string;
  PRNumber: number;
  Status: string;
}

export interface WebhookDeliveryList {
  deliveries: WebhookDelivery[];
  total: number;
  page: number;
  per_page: number;
}

// Provider types with metadata for the UI.
export interface ProviderTypeMeta {
  label: string;
  needsApiKey: boolean;
  needsBaseUrl: boolean;
  presetBaseUrl?: string;
  defaultModel?: string;
}

export const PROVIDER_TYPE_META: Record<string, ProviderTypeMeta> = {
  openai: { label: "OpenAI", needsApiKey: true, needsBaseUrl: false, defaultModel: "gpt-4o" },
  anthropic: { label: "Anthropic", needsApiKey: true, needsBaseUrl: false, defaultModel: "claude-sonnet-4-6" },
  ollama: { label: "Ollama (local)", needsApiKey: false, needsBaseUrl: true },
  openai_compatible: { label: "OpenAI-compatible", needsApiKey: true, needsBaseUrl: true },
  gemini: {
    label: "Google Gemini",
    needsApiKey: true,
    needsBaseUrl: false,
    presetBaseUrl: "https://generativelanguage.googleapis.com/v1beta/openai/",
    defaultModel: "gemini-1.5-pro",
  },
  groq: {
    label: "Groq",
    needsApiKey: true,
    needsBaseUrl: false,
    presetBaseUrl: "https://api.groq.com/openai/v1",
    defaultModel: "llama-3.3-70b-versatile",
  },
  mistral: {
    label: "Mistral AI",
    needsApiKey: true,
    needsBaseUrl: false,
    presetBaseUrl: "https://api.mistral.ai/v1",
    defaultModel: "mistral-large-latest",
  },
  together_ai: {
    label: "Together AI",
    needsApiKey: true,
    needsBaseUrl: false,
    presetBaseUrl: "https://api.together.xyz/v1",
    defaultModel: "meta-llama/Llama-3-70b-chat-hf",
  },
  perplexity: {
    label: "Perplexity AI",
    needsApiKey: true,
    needsBaseUrl: false,
    presetBaseUrl: "https://api.perplexity.ai",
    defaultModel: "sonar-pro",
  },
};
