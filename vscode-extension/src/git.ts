import { execFile } from "child_process";
import { promisify } from "util";

const execFileAsync = promisify(execFile);

export interface RepoRef {
  owner: string;
  repo: string;
}

// detectRepo reads the `origin` remote of the git repository at cwd and parses the
// GitHub owner/repo. Supports both SSH (git@github.com:owner/repo.git) and HTTPS
// (https://github.com/owner/repo(.git)) remote URLs.
export async function detectRepo(cwd: string): Promise<RepoRef | undefined> {
  let url: string;
  try {
    const { stdout } = await execFileAsync("git", ["remote", "get-url", "origin"], { cwd });
    url = stdout.trim();
  } catch {
    return undefined;
  }
  return parseRemote(url);
}

export function parseRemote(url: string): RepoRef | undefined {
  // git@host:owner/repo.git  OR  ssh://git@host/owner/repo.git
  // https://host/owner/repo(.git)
  const cleaned = url.replace(/\.git$/, "");
  const sshMatch = cleaned.match(/[:/]([^/:]+)\/([^/]+)$/);
  if (sshMatch) {
    return { owner: sshMatch[1], repo: sshMatch[2] };
  }
  return undefined;
}
