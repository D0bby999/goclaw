import { useState, useEffect, useCallback } from "react";
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
import { useHttp } from "@/hooks/use-ws";
import { toast } from "@/stores/use-toast-store";
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
  onOAuthComplete?: () => void;
}

export function AccountConnectDialog({ open, onOpenChange, onSubmit, onUpdate, editAccount, onOAuthComplete }: AccountConnectDialogProps) {
  const http = useHttp();
  const [platform, setPlatform] = useState<SocialPlatform>("twitter");
  const [platformUserId, setPlatformUserId] = useState("");
  const [accessToken, setAccessToken] = useState("");
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [saving, setSaving] = useState(false);
  const [oauthPlatforms, setOAuthPlatforms] = useState<Record<string, boolean>>({});
  const [oauthLoading, setOAuthLoading] = useState<SocialPlatform | null>(null);

  const isEdit = !!editAccount;
  // Bluesky is always manual-only; all other platforms use OAuth if configured.
  const isOAuthPlatform = !isEdit && platform !== "bluesky" && oauthPlatforms[platform] === true;

  // Check OAuth status on mount.
  useEffect(() => {
    http.get<{ platforms: Record<string, boolean> }>("/v1/social/oauth/status")
      .then((res) => setOAuthPlatforms(res.platforms ?? {}))
      .catch(() => setOAuthPlatforms({}));
  }, [http]);

  // Listen for OAuth callback result via postMessage.
  const handleOAuthMessage = useCallback(
    (event: MessageEvent) => {
      // Validate origin to prevent cross-origin message spoofing.
      if (event.origin !== window.location.origin) return;
      if (event.data?.type !== "social-oauth-result") return;
      const { success, message } = event.data;
      setOAuthLoading(null);
      if (success) {
        toast.success("Account connected", message);
        onOAuthComplete?.();
        onOpenChange(false);
      } else {
        toast.error("OAuth failed", message);
      }
    },
    [onOAuthComplete, onOpenChange],
  );

  useEffect(() => {
    window.addEventListener("message", handleOAuthMessage);
    return () => window.removeEventListener("message", handleOAuthMessage);
  }, [handleOAuthMessage]);

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

  const handleOAuthConnect = async (p: SocialPlatform) => {
    setOAuthLoading(p);
    try {
      const res = await http.get<{ auth_url: string }>("/v1/social/oauth/start", { platform: p });
      if (res.auth_url) {
        // Open popup for OAuth.
        const w = 600, h = 700;
        const left = window.screenX + (window.outerWidth - w) / 2;
        const top = window.screenY + (window.outerHeight - h) / 2;
        window.open(res.auth_url, "social-oauth", `width=${w},height=${h},left=${left},top=${top}`);
      }
    } catch (err) {
      setOAuthLoading(null);
      toast.error("Failed to start OAuth", err instanceof Error ? err.message : "Unknown error");
    }
  };

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

          {/* OAuth connect button for Meta platforms */}
          {isOAuthPlatform && (
            <div className="space-y-2">
              <Button
                className="w-full gap-2"
                onClick={() => handleOAuthConnect(platform)}
                disabled={oauthLoading !== null}
              >
                <PlatformIcon platform={platform} className="h-4 w-4" />
                {oauthLoading === platform
                  ? "Waiting for authorization..."
                  : `Connect with ${PLATFORM_META[platform].label}`}
              </Button>
              <p className="text-xs text-muted-foreground text-center">
                Connect via {PLATFORM_META[platform]?.label ?? platform} OAuth to securely link your account.
              </p>
              <div className="relative my-2">
                <div className="absolute inset-0 flex items-center"><span className="w-full border-t" /></div>
                <div className="relative flex justify-center text-xs"><span className="bg-background px-2 text-muted-foreground">or enter manually</span></div>
              </div>
            </div>
          )}

          {/* Manual token entry */}
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
