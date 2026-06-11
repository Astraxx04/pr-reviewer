// Minimal client for the PR Reviewer platform API. Auth uses a long-lived API token
// (prt_...) sent as a Bearer header — the same scheme the CLI uses.

export interface PRSummary {
  number: number;
  title: string;
  author: string;
  repo: string;
  pr_status: string;
  current_score: number;
}

export interface PRComment {
  path: string;
  line: number;
  body: string;
  severity: string;
  priority: string;
}

export interface PRDetail {
  number: number;
  title: string;
  repo: string;
  latest_comments: PRComment[];
}

export class ApiError extends Error {}

export class Client {
  constructor(private serverUrl: string, private token: string) {
    this.serverUrl = serverUrl.replace(/\/+$/, "");
  }

  private async request<T>(path: string, method = "GET"): Promise<T> {
    let res: Response;
    try {
      res = await fetch(`${this.serverUrl}${path}`, {
        method,
        headers: {
          Authorization: `Bearer ${this.token}`,
          "ngrok-skip-browser-warning": "true",
        },
        signal: AbortSignal.timeout(10_000),
      });
    } catch (e) {
      const cause = e instanceof Error ? e.message : String(e);
      throw new ApiError(`Cannot reach ${this.serverUrl}: ${cause}`);
    }
    if (res.status === 401) {
      throw new ApiError("Unauthorized — check your API token (Settings → prReviewer.apiToken).");
    }
    if (!res.ok) {
      throw new ApiError(`Request failed (${res.status} ${res.statusText})`);
    }
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  listPRs(repo: string): Promise<{ prs: PRSummary[] }> {
    return this.request(`/api/prs?repo=${encodeURIComponent(repo)}&per_page=50`);
  }

  getPR(owner: string, repo: string, number: number): Promise<PRDetail> {
    return this.request(`/api/prs/${owner}/${repo}/${number}`);
  }

  triggerReview(owner: string, repo: string, number: number): Promise<{ ok: boolean }> {
    return this.request(`/api/prs/${owner}/${repo}/${number}/re-review`, "POST");
  }
}
