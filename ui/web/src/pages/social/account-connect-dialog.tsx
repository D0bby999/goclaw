import { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PlatformIcon, ALL_PLATFORMS, PLATFORM_META } from "./platform-icons";
import type { SocialAccount, SocialPlatform } from "@/types/social";

interface AccountConnectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (params: {
    platform: string;
    platform_user_id: string;
    access_token: string;
    platform_username?: string;
    display_name?: string;
  }) => Promise<void>;
  onUpdate?: (id: string, updates: Record<string, unknown>) => Promise<void>;
  editAccount?: SocialAccount | null;
}

export function AccountConnectDialog({ open, onOpenChange, onSubmit, onUpdate, editAccount }: AccountConnectDialogProps) {
  const [platform, setPlatform] = useState<SocialPlatform>("twitter");
  const [platformUserId, setPlatformUserId] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [saving, setSaving] = useState(false);

  const isEdit = !!editAccount;

  useEffect(() => {
    if (editAccount) {
      setPlatform(editAccount.platform);
      setPlatformUserId(editAccount.platform_user_id);
      setAccessToken("");
      setUsername(editAccount.platform_username ?? "");
      setDisplayName(editAccount.display_name ?? "");
    } else {
      setPlatform("twitter");
      setPlatformUserId("");
      setAccessToken("");
      setUsername("");
      setDisplayName("");
    }
  }, [editAccount, open]);

  const canSubmit = platform && platformUserId && (isEdit || accessToken);

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSaving(true);
    try {
      if (isEdit && onUpdate) {
        const updates: Record<string, unknown> = { platform_username: username || null, display_name: displayName || null };
        if (accessToken) updates.access_token = accessToken;
        await onUpdate(editAccount.id, updates);
      } else {
        await onSubmit({
          platform,
          platform_user_id: platformUserId,
          access_token: accessToken,
          platform_username: username || undefined,
          display_name: displayName || undefined,
        });
      }
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Account" : "Connect Account"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update account details." : "Connect a social media account."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Platform selector */}
          {!isEdit && (
            <div className="space-y-2">
              <Label>Platform</Label>
              <div className="grid grid-cols-4 gap-2">
                {ALL_PLATFORMS.map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => setPlatform(p)}
                    className={`flex flex-col items-center gap-1 rounded-md border p-2 text-xs transition-colors ${
                      platform === p ? "border-primary bg-primary/10" : "hover:bg-muted"
                    }`}
                  >
                    <PlatformIcon platform={p} className="h-5 w-5" />
                    <span className="truncate">{PLATFORM_META[p].label}</span>
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="space-y-2">
            <Label>Platform User ID</Label>
            <Input
              value={platformUserId}
              onChange={(e) => setPlatformUserId(e.target.value)}
              placeholder="e.g. 123456789"
              disabled={isEdit}
            />
          </div>

          <div className="space-y-2">
            <Label>{isEdit ? "New Access Token (leave blank to keep current)" : "Access Token"}</Label>
            <Input
              type="password"
              value={accessToken}
              onChange={(e) => setAccessToken(e.target.value)}
              placeholder={isEdit ? "Leave blank to keep current" : "Paste access token"}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label>Username</Label>
              <Input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="@handle"
              />
            </div>
            <div className="space-y-2">
              <Label>Display Name</Label>
              <Input
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Display name"
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || saving}>
            {saving ? "..." : isEdit ? "Save" : "Connect"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
