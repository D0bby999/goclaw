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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PlatformIcon } from "./platform-icons";
import type { SocialAccount } from "@/types/social";

interface PageConnectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  accounts: SocialAccount[];
  onSubmit: (params: {
    accountId: string;
    pageId: string;
    pageName: string;
    pageToken: string;
    pageType: string;
  }) => Promise<void>;
}

const PAGE_TYPES = ["page", "business", "channel"] as const;

export function PageConnectDialog({ open, onOpenChange, accounts, onSubmit }: PageConnectDialogProps) {
  const [accountId, setAccountId] = useState("");
  const [pageId, setPageId] = useState("");
  const [pageName, setPageName] = useState("");
  const [pageToken, setPageToken] = useState("");
  const [pageType, setPageType] = useState("page");
  const [saving, setSaving] = useState(false);

  // Reset form when dialog opens.
  useEffect(() => {
    if (open) {
      setAccountId(accounts[0]?.id ?? "");
      setPageId("");
      setPageName("");
      setPageToken("");
      setPageType("page");
    }
  }, [open, accounts]);

  const canSubmit = accountId && pageId && pageName && pageToken;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSaving(true);
    try {
      await onSubmit({ accountId, pageId, pageName, pageToken, pageType });
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Page</DialogTitle>
          <DialogDescription>Manually connect a social page to an account.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>Account</Label>
            <Select value={accountId} onValueChange={setAccountId}>
              <SelectTrigger>
                <SelectValue placeholder="Select an account" />
              </SelectTrigger>
              <SelectContent>
                {accounts.map((acc) => (
                  <SelectItem key={acc.id} value={acc.id}>
                    <span className="flex items-center gap-2">
                      <PlatformIcon platform={acc.platform} className="h-4 w-4" />
                      {acc.display_name || acc.platform_username || acc.platform}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>Page Type</Label>
            <Select value={pageType} onValueChange={setPageType}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PAGE_TYPES.map((t) => (
                  <SelectItem key={t} value={t}>
                    {t.charAt(0).toUpperCase() + t.slice(1)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>Page ID</Label>
            <Input
              value={pageId}
              onChange={(e) => setPageId(e.target.value)}
              placeholder="e.g. 123456789"
            />
          </div>

          <div className="space-y-2">
            <Label>Page Name</Label>
            <Input
              value={pageName}
              onChange={(e) => setPageName(e.target.value)}
              placeholder="My Page"
            />
          </div>

          <div className="space-y-2">
            <Label>Page Token</Label>
            <Input
              type="password"
              value={pageToken}
              onChange={(e) => setPageToken(e.target.value)}
              placeholder="Paste page access token"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || saving}>
            {saving ? "..." : "Add Page"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
