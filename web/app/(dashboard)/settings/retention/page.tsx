"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { getRetentionSettings, putRetentionSettings, eraseUserData } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { toast } from "sonner";

export default function RetentionPage() {
  const { token } = useToken();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [retentionDays, setRetentionDays] = useState(0);
  const [purgeEmbeddings, setPurgeEmbeddings] = useState(false);
  const [eraseLogin, setEraseLogin] = useState("");
  const [eraseConfirmOpen, setEraseConfirmOpen] = useState(false);
  const [erasing, setErasing] = useState(false);

  useEffect(() => {
    if (!token) return;
    getRetentionSettings(token)
      .then((s) => {
        setRetentionDays(s.review_retention_days ?? 0);
        setPurgeEmbeddings(s.purge_embeddings_on_disable ?? false);
      })
      .catch(() => toast.error("Failed to load retention settings"))
      .finally(() => setLoading(false));
  }, [token]);

  async function handleSave() {
    if (!token) return;
    setSaving(true);
    try {
      await putRetentionSettings(token, {
        review_retention_days: retentionDays,
        purge_embeddings_on_disable: purgeEmbeddings,
      });
      toast.success("Retention settings saved");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleErase() {
    if (!token || !eraseLogin.trim()) return;
    setErasing(true);
    try {
      await eraseUserData(token, eraseLogin.trim());
      toast.success(`All data for ${eraseLogin} has been erased`);
      setEraseLogin("");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Erasure failed");
    } finally {
      setErasing(false);
      setEraseConfirmOpen(false);
    }
  }

  if (loading) {
    return (
      <div className="space-y-4 max-w-2xl">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-36 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold">Data Retention</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Configure how long review data is kept and manage GDPR erasure requests.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Review retention</CardTitle>
          <CardDescription>
            Automatically delete reviews older than the configured number of days.
            Set to 0 to keep reviews forever.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="retention-days">Retention period (days)</Label>
            <div className="flex items-center gap-3 mt-1">
              <Input
                id="retention-days"
                type="number"
                min={0}
                step={1}
                value={retentionDays}
                onChange={(e) => setRetentionDays(Number(e.target.value))}
                className="w-32"
              />
              <span className="text-sm text-muted-foreground">
                {retentionDays === 0 ? "Keep forever" : `Delete after ${retentionDays} days`}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Switch
              id="purge-embeddings"
              checked={purgeEmbeddings}
              onCheckedChange={setPurgeEmbeddings}
            />
            <Label htmlFor="purge-embeddings">
              Purge code embeddings when a repo is disabled
            </Label>
          </div>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? "Saving…" : "Save settings"}
          </Button>
        </CardContent>
      </Card>

      <Card className="border-destructive/40">
        <CardHeader>
          <CardTitle>GDPR — Right to erasure</CardTitle>
          <CardDescription>
            Permanently delete all data associated with a GitHub user login. This action cannot be undone.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="erase-login">GitHub login</Label>
            <Input
              id="erase-login"
              placeholder="github-username"
              value={eraseLogin}
              onChange={(e) => setEraseLogin(e.target.value)}
              className="mt-1 max-w-xs"
            />
          </div>
          <Button
            variant="destructive"
            disabled={!eraseLogin.trim() || erasing}
            onClick={() => setEraseConfirmOpen(true)}
          >
            Erase all data for this user
          </Button>
        </CardContent>
      </Card>

      <Dialog open={eraseConfirmOpen} onOpenChange={setEraseConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Erase all data for &ldquo;{eraseLogin}&rdquo;?</DialogTitle>
            <DialogDescription>
              This will permanently delete the user account, anonymise audit log entries, and revoke
              all API tokens for this user. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEraseConfirmOpen(false)}>Cancel</Button>
            <Button
              variant="destructive"
              onClick={handleErase}
              disabled={erasing}
            >
              {erasing ? "Erasing…" : "Erase"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
