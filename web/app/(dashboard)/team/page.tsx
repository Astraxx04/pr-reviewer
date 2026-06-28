"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { listTeam, addTeamMember, updateTeamMember, removeTeamMember, type TeamMember } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { Trash2, Copy, Check } from "lucide-react";

const ROLES = ["admin", "reviewer", "viewer"] as const;
type Role = (typeof ROLES)[number];

const roleColors: Record<Role, string> = {
  admin: "text-orange-600 bg-orange-50 border-orange-200",
  reviewer: "text-blue-600 bg-blue-50 border-blue-200",
  viewer: "text-gray-600 bg-gray-50 border-gray-200",
};

export default function TeamPage() {
  const { token } = useToken();
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [inviteLogin, setInviteLogin] = useState<string | null>(null);
  const [login, setLogin] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("reviewer");
  const [adding, setAdding] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!token) return;
    listTeam(token).then(setMembers).finally(() => setLoading(false));
  }, [token]);

  async function handleAdd() {
    if (!token || !login.trim()) return;
    setAdding(true);
    try {
      const m = await addTeamMember(token, login.trim(), role, email.trim() || undefined);
      setMembers((prev) => [...prev, m]);
      setOpen(false);
      setInviteLogin(login.trim());
      setLogin("");
      setEmail("");
      setRole("reviewer");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setAdding(false);
    }
  }

  async function handleRoleChange(id: number, newRole: string) {
    if (!token) return;
    try {
      await updateTeamMember(token, id, newRole);
      setMembers((prev) => prev.map((m) => m.ID === id ? { ...m, Role: newRole } : m));
    } catch (e) {
      toast.error(String(e));
    }
  }

  async function handleRemove(id: number, memberLogin: string) {
    if (!token) return;
    try {
      await removeTeamMember(token, id);
      setMembers((prev) => prev.filter((m) => m.ID !== id));
      toast.success(`${memberLogin} removed`);
    } catch (e) {
      toast.error(String(e));
    }
  }

  function copyInviteLink() {
    const url = `${window.location.origin}/login`;
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  if (loading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Team</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Add members by GitHub username. They sign in with GitHub — their role takes effect on first login.
          </p>
        </div>
        <Button onClick={() => setOpen(true)} style={{ cursor: 'pointer' }}>Add Member</Button>
      </div>

      {members.length === 0 ? (
        <p className="text-muted-foreground text-sm">No team members yet.</p>
      ) : (
        <div className="rounded-lg border divide-y">
          {members.map((m) => (
            <div key={m.ID} className="flex items-center gap-3 px-4 py-3">
              <Avatar className="h-8 w-8 shrink-0">
                <AvatarFallback>{m.Login[0].toUpperCase()}</AvatarFallback>
              </Avatar>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate">{m.Login}</p>
                <p className="text-xs text-muted-foreground">Added {new Date(m.CreatedAt).toLocaleDateString()}</p>
              </div>
              <Select value={m.Role} onValueChange={(v) => { if (v) handleRoleChange(m.ID, v); }}>
                <SelectTrigger className={`w-28 h-7 text-xs border ${roleColors[m.Role as Role] ?? ""}`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ROLES.map((r) => (
                    <SelectItem key={r} value={r} className="text-xs">{r}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 text-muted-foreground hover:text-destructive shrink-0"
                onClick={() => handleRemove(m.ID, m.Login)}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* Add Member Dialog */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Team Member</DialogTitle>
            <DialogDescription>
              They sign in with GitHub — no separate account needed.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1">
              <Label>GitHub Username</Label>
              <Input
                placeholder="octocat"
                value={login}
                onChange={(e) => setLogin(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleAdd()}
              />
            </div>
            <div className="space-y-1">
              <Label>Role</Label>
              <Select value={role} onValueChange={(v) => setRole(v as Role)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="admin">admin — manage settings, repos, and team</SelectItem>
                  <SelectItem value="reviewer">reviewer — view and trigger reviews</SelectItem>
                  <SelectItem value="viewer">viewer — read-only access</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label>
                Email
                <span className="text-muted-foreground font-normal ml-1 text-xs">— optional, sends invite if email notifications are configured</span>
              </Label>
              <Input
                type="email"
                placeholder="name@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>Cancel</Button>
            <Button onClick={handleAdd} disabled={adding || !login.trim()}>
              {adding ? "Adding…" : "Add"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Invite Link Dialog */}
      <Dialog open={!!inviteLogin} onOpenChange={() => setInviteLogin(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{inviteLogin} added</DialogTitle>
            <DialogDescription>
              Share this link so they can sign in with GitHub. Their role will be applied automatically on first login.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 rounded-md border bg-muted px-3 py-2 text-sm font-mono">
            <span className="flex-1 truncate">{typeof window !== "undefined" ? window.location.origin : ""}/login</span>
            <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0" onClick={copyInviteLink}>
              {copied ? <Check className="h-3.5 w-3.5 text-green-500" /> : <Copy className="h-3.5 w-3.5" />}
            </Button>
          </div>
          <DialogFooter>
            <Button onClick={() => setInviteLogin(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
