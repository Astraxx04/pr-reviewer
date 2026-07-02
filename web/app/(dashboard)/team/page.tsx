"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import {
  listTeam, listInvites, createInvite, revokeInvite, resendInvite,
  updateUserRole, removeUser, reactivateUser,
  type TeamMember, type Invite,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { RotateCcw, Trash2, Send, Mail, UserX, UserCheck } from "lucide-react";

const roleBadgeClass: Record<string, string> = {
  owner:    "bg-purple-50 text-purple-700 border-purple-200",
  admin:    "bg-orange-50 text-orange-700 border-orange-200",
  reviewer: "bg-blue-50 text-blue-700 border-blue-200",
};

type Role = "admin" | "reviewer";

type ConfirmState =
  | { type: "remove";      member: TeamMember }
  | { type: "role";        member: TeamMember; newRole: Role }
  | { type: "reactivate";  member: TeamMember }
  | null;

function isValidEmail(v: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v.trim());
}

export default function TeamPage() {
  const { token, userId: currentUserId } = useToken();
  const [members, setMembers]   = useState<TeamMember[]>([]);
  const [invites, setInvites]   = useState<Invite[]>([]);
  const [loading, setLoading]   = useState(true);

  // Invite dialog
  const [inviteOpen, setInviteOpen] = useState(false);
  const [email, setEmail]           = useState("");
  const [emailTouched, setEmailTouched] = useState(false);
  const [role, setRole]             = useState<Role>("reviewer");
  const [sending, setSending]       = useState(false);

  // Confirmation dialog
  const [confirm, setConfirm]   = useState<ConfirmState>(null);
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    if (!token) return;
    Promise.all([listTeam(token), listInvites(token)])
      .then(([m, i]) => { setMembers(m); setInvites(i); })
      .finally(() => setLoading(false));
  }, [token]);

  // ── invite ────────────────────────────────────────────────────────────────
  async function handleInvite() {
    if (!token || !isValidEmail(email)) return;
    setSending(true);
    try {
      const inv = await createInvite(token, { email: email.trim(), role });
      setInviteOpen(false);
      listInvites(token).then(setInvites).catch(() => {});
      setEmail("");
      setEmailTouched(false);
      setRole("reviewer");
      toast.success(`Invite sent to ${inv.email}`);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setSending(false);
    }
  }

  async function handleRevoke(id: string, invEmail: string) {
    if (!token) return;
    try {
      await revokeInvite(token, id);
      setInvites((prev) => prev.filter((i) => i.id !== id));
      toast.success(`Invite for ${invEmail} revoked`);
    } catch (e) { toast.error(String(e)); }
  }

  async function handleResend(id: string, invEmail: string) {
    if (!token) return;
    try {
      await resendInvite(token, id);
      toast.success(`Invite resent to ${invEmail}`);
    } catch (e) { toast.error(String(e)); }
  }

  // ── confirmation actions ──────────────────────────────────────────────────
  async function executeConfirm() {
    if (!confirm || !token) return;
    setConfirming(true);
    try {
      if (confirm.type === "remove") {
        await removeUser(token, confirm.member.id);
        setMembers((prev) => prev.map((m) =>
          m.id === confirm.member.id ? { ...m, status: "suspended" } : m
        ));
        toast.success(`${confirm.member.login} removed.`);
      } else if (confirm.type === "role") {
        await updateUserRole(token, confirm.member.id, confirm.newRole);
        setMembers((prev) => prev.map((m) =>
          m.id === confirm.member.id ? { ...m, role: confirm.newRole } : m
        ));
        toast.success(`Role updated — ${confirm.member.login} will re-login to get their new role.`);
      } else if (confirm.type === "reactivate") {
        await reactivateUser(token, confirm.member.id);
        setMembers((prev) => prev.map((m) =>
          m.id === confirm.member.id ? { ...m, status: "active" } : m
        ));
        toast.success(`${confirm.member.login} reactivated.`);
      }
      setConfirm(null);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setConfirming(false);
    }
  }

  // ── derived lists ─────────────────────────────────────────────────────────
  const activeMembers    = members.filter((m) => m.status === "active");
  const suspendedMembers = members.filter((m) => m.status === "suspended");
  const pendingInvites   = invites.filter((i) => i.pending);

  const emailError = emailTouched && email && !isValidEmail(email)
    ? "Enter a valid email address"
    : null;

  if (loading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-8 max-w-2xl">

      {/* ── header ── */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Team</h1>
          <p className="text-base text-muted-foreground mt-1">
            Invite teammates by email. They sign in with GitHub using the invited address.
          </p>
        </div>
        <Button onClick={() => setInviteOpen(true)}>
          <Send className="h-4 w-4 mr-2" />
          Invite
        </Button>
      </div>

      {/* ── active members ── */}
      <section className="space-y-3">
        <h2 className="text-base font-semibold text-muted-foreground uppercase tracking-wide">
          Members ({activeMembers.length})
        </h2>
        {activeMembers.length === 0 ? (
          <p className="text-muted-foreground text-base">No members yet.</p>
        ) : (
          <div className="rounded-lg border divide-y">
            {activeMembers.map((m) => (
              <div key={m.id} className="flex items-center gap-4 px-5 py-4">
                <Avatar className="h-10 w-10 shrink-0">
                  {m.avatar_url && <AvatarImage src={m.avatar_url} alt={m.login} />}
                  <AvatarFallback className="text-sm">{m.login?.[0]?.toUpperCase() ?? "?"}</AvatarFallback>
                </Avatar>
                <div className="flex-1 min-w-0">
                  <p className="text-base font-medium truncate">{m.login}</p>
                  {m.created_at && (
                    <p className="text-sm text-muted-foreground">
                      Joined {new Date(m.created_at).toLocaleDateString()}
                    </p>
                  )}
                </div>

                {/* role control */}
                {m.role === "owner" || m.id === currentUserId ? (
                  <Badge variant="outline" className={`text-sm px-3 py-1 ${roleBadgeClass[m.role] ?? ""}`}>{m.role}</Badge>
                ) : (
                  <div className="flex rounded-md border overflow-hidden shrink-0 text-sm">
                    {(["admin", "reviewer"] as Role[]).map((r) => (
                      <button
                        key={r}
                        onClick={() => m.role !== r && setConfirm({ type: "role", member: m, newRole: r })}
                        className={[
                          "px-4 py-1.5 transition-colors cursor-pointer",
                          r === "admin" ? "border-r" : "",
                          m.role === r
                            ? r === "admin"
                              ? "bg-orange-50 text-orange-700 font-medium"
                              : "bg-blue-50 text-blue-700 font-medium"
                            : "text-muted-foreground hover:bg-muted",
                        ].join(" ")}
                      >
                        {r}
                      </button>
                    ))}
                  </div>
                )}

                {/* remove */}
                {m.role !== "owner" && m.id !== currentUserId && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-destructive shrink-0"
                    title="Remove user"
                    onClick={() => setConfirm({ type: "remove", member: m })}
                  >
                    <UserX className="h-4 w-4" />
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* ── pending invites ── */}
      {pendingInvites.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-base font-semibold text-muted-foreground uppercase tracking-wide">
            Pending invites ({pendingInvites.length})
          </h2>
          <div className="rounded-lg border divide-y">
            {pendingInvites.map((inv) => (
              <div key={inv.id} className="flex items-center gap-4 px-5 py-4">
                <div className="rounded-full bg-muted p-2.5 shrink-0">
                  <Mail className="h-5 w-5 text-muted-foreground" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-base font-medium truncate">{inv.email}</p>
                  <p className="text-sm text-muted-foreground">
                    Expires {new Date(inv.expires_at).toLocaleDateString()} · invited by {inv.invited_by}
                  </p>
                </div>
                <Badge variant="outline" className={`text-sm px-3 py-1 ${roleBadgeClass[inv.role] ?? ""}`}>
                  {inv.role}
                </Badge>
                <Button
                  variant="ghost" size="icon"
                  className="h-8 w-8 text-muted-foreground hover:text-primary shrink-0"
                  title="Resend invite"
                  onClick={() => handleResend(inv.id, inv.email)}
                >
                  <RotateCcw className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost" size="icon"
                  className="h-8 w-8 text-muted-foreground hover:text-destructive shrink-0"
                  title="Revoke invite"
                  onClick={() => handleRevoke(inv.id, inv.email)}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* ── suspended ── */}
      {suspendedMembers.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-base font-semibold text-muted-foreground uppercase tracking-wide">
            Suspended ({suspendedMembers.length})
          </h2>
          <div className="rounded-lg border divide-y">
            {suspendedMembers.map((m) => (
              <div key={m.id} className="flex items-center gap-4 px-5 py-4 opacity-60">
                <Avatar className="h-10 w-10 shrink-0">
                  {m.avatar_url && <AvatarImage src={m.avatar_url} alt={m.login} />}
                  <AvatarFallback className="text-sm">{m.login?.[0]?.toUpperCase() ?? "?"}</AvatarFallback>
                </Avatar>
                <div className="flex-1 min-w-0">
                  <p className="text-base font-medium truncate">{m.login}</p>
                  <p className="text-sm text-muted-foreground">{m.role}</p>
                </div>
                <Badge variant="outline" className="text-sm px-3 py-1 bg-red-50 text-red-700 border-red-200">
                  suspended
                </Badge>
                <Button
                  variant="ghost" size="icon"
                  className="h-8 w-8 text-muted-foreground hover:text-green-600 shrink-0 opacity-100"
                  title="Reactivate user"
                  onClick={() => setConfirm({ type: "reactivate", member: m })}
                >
                  <UserCheck className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* ── invite dialog ── */}
      <Dialog open={inviteOpen} onOpenChange={(o) => { setInviteOpen(o); if (!o) { setEmail(""); setEmailTouched(false); setRole("reviewer"); } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-xl">Send invitation</DialogTitle>
            <DialogDescription className="text-base">
              They&apos;ll receive a link by email and sign in with the GitHub account that has this address verified.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            {/* email */}
            <div className="space-y-2">
              <Label className="text-base" htmlFor="invite-email">Email address</Label>
              <Input
                id="invite-email"
                type="email"
                placeholder="name@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onBlur={() => setEmailTouched(true)}
                onKeyDown={(e) => e.key === "Enter" && handleInvite()}
                className={`text-base h-11 ${emailError ? "border-destructive focus-visible:ring-destructive" : ""}`}
              />
              {emailError && (
                <p className="text-sm text-destructive">{emailError}</p>
              )}
            </div>

            {/* role */}
            <div className="space-y-2">
              <Label className="text-base">Role</Label>
              <div className="grid grid-cols-2 gap-3">
                {(["admin", "reviewer"] as Role[]).map((r) => (
                  <button
                    key={r}
                    type="button"
                    onClick={() => setRole(r)}
                    className={[
                      "flex flex-col items-start gap-1 rounded-lg border p-4 text-left transition-colors cursor-pointer",
                      role === r
                        ? "border-primary bg-primary/5"
                        : "border-border hover:bg-muted",
                    ].join(" ")}
                  >
                    <span className="text-base font-medium capitalize">{r}</span>
                    <span className="text-sm text-muted-foreground leading-snug">
                      {r === "admin"
                        ? "Manage settings, repos & team"
                        : "View and trigger reviews"}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button size="lg" variant="outline" onClick={() => setInviteOpen(false)}>Cancel</Button>
            <Button size="lg" onClick={handleInvite} disabled={sending || !isValidEmail(email)}>
              {sending ? "Sending…" : "Send invite"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── confirmation dialog ── */}
      <Dialog open={!!confirm} onOpenChange={(o) => { if (!o) setConfirm(null); }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle className="text-xl">
              {confirm?.type === "remove"     && "Remove user"}
              {confirm?.type === "role"       && "Change role"}
              {confirm?.type === "reactivate" && "Reactivate user"}
            </DialogTitle>
            <DialogDescription className="text-base">
              {confirm?.type === "remove" && (
                <>
                  <strong>{confirm.member.login}</strong> will lose access immediately and their sessions will be revoked.
                </>
              )}
              {confirm?.type === "role" && (
                <>
                  Change <strong>{confirm.member.login}</strong>&apos;s role from{" "}
                  <strong>{confirm.member.role}</strong> to <strong>{confirm.newRole}</strong>?
                  They&apos;ll need to re-login for the change to take effect.
                </>
              )}
              {confirm?.type === "reactivate" && (
                <>
                  Reactivate <strong>{confirm.member.login}</strong>? They&apos;ll be able to sign in again.
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button size="lg" variant="outline" onClick={() => setConfirm(null)} disabled={confirming}>
              Cancel
            </Button>
            <Button
              size="lg"
              variant={confirm?.type === "remove" ? "destructive" : "default"}
              onClick={executeConfirm}
              disabled={confirming}
            >
              {confirming ? "…" : confirm?.type === "remove" ? "Remove" : confirm?.type === "reactivate" ? "Reactivate" : "Change role"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

    </div>
  );
}
