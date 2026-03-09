import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { queryKeys } from "@/lib/query-keys";
import type { ContentSchedule, SocialAccount, SocialPage } from "@/types/social";

interface ScheduleFormData {
  name: string;
  cron_expression: string;
  timezone: string;
  agent_id?: string;
  prompt?: string;
  page_ids?: string[];
}

interface ScheduleFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editSchedule: ContentSchedule | null;
  onSubmit: (data: ScheduleFormData) => Promise<void>;
}

type PageWithPlatform = SocialPage & { _platform?: string; _accountId: string };

const TIMEZONES = typeof Intl !== "undefined" && "supportedValuesOf" in Intl
  ? (Intl as unknown as { supportedValuesOf: (k: string) => string[] }).supportedValuesOf("timeZone")
  : ["UTC", "America/New_York", "America/Los_Angeles", "Europe/London", "Asia/Tokyo"];

export function ScheduleFormDialog({ open, onOpenChange, editSchedule, onSubmit }: ScheduleFormDialogProps) {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const isEdit = !!editSchedule;

  const [name, setName] = useState("");
  const [cronExpr, setCronExpr] = useState("0 9 * * 1");
  const [timezone, setTimezone] = useState("UTC");
  const [agentId, setAgentId] = useState("");
  const [prompt, setPrompt] = useState("");
  const [selectedPageIds, setSelectedPageIds] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);

  // Load accounts then pages
  const { data: accounts = [] } = useQuery({
    queryKey: queryKeys.social.accounts,
    queryFn: async () => {
      const res = await http.get<{ accounts: SocialAccount[] }>("/v1/social/accounts");
      return res.accounts ?? [];
    },
    enabled: connected && open,
  });

  const { data: allPages = [] } = useQuery({
    queryKey: ["social", "all-pages-form", accounts.map((a) => a.id)],
    queryFn: async () => {
      const results = await Promise.all(
        accounts.map(async (acc) => {
          try {
            const res = await http.get<{ pages: SocialPage[] }>(`/v1/social/accounts/${acc.id}/pages`);
            return (res.pages ?? []).map((p): PageWithPlatform => ({
              ...p,
              _platform: acc.platform,
              _accountId: acc.id,
            }));
          } catch {
            return [] as PageWithPlatform[];
          }
        }),
      );
      return results.flat();
    },
    enabled: connected && open && accounts.length > 0,
  });

  useEffect(() => {
    if (!open) return;
    if (editSchedule) {
      setName(editSchedule.name);
      setCronExpr(editSchedule.cron_expression);
      setTimezone(editSchedule.timezone || "UTC");
      setAgentId(editSchedule.agent_id ?? "");
      setPrompt(editSchedule.prompt ?? "");
      setSelectedPageIds((editSchedule.pages ?? []).map((p) => p.page_id));
    } else {
      setName("");
      setCronExpr("0 9 * * 1");
      setTimezone("UTC");
      setAgentId("");
      setPrompt("");
      setSelectedPageIds([]);
    }
  }, [open, editSchedule]);

  const togglePage = (pageId: string) => {
    setSelectedPageIds((prev) =>
      prev.includes(pageId) ? prev.filter((id) => id !== pageId) : [...prev, pageId],
    );
  };

  const handleSubmit = async () => {
    if (!name.trim() || !cronExpr.trim()) return;
    setSaving(true);
    try {
      await onSubmit({
        name: name.trim(),
        cron_expression: cronExpr.trim(),
        timezone,
        agent_id: agentId.trim() || undefined,
        prompt: prompt.trim() || undefined,
        page_ids: selectedPageIds,
      });
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Schedule" : "New Schedule"}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 overflow-y-auto min-h-0 px-0.5 -mx-0.5">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Weekly posts" />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label>Cron Expression</Label>
              <Input
                value={cronExpr}
                onChange={(e) => setCronExpr(e.target.value)}
                placeholder="0 9 * * 1"
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">min hour day month weekday</p>
            </div>
            <div className="space-y-2">
              <Label>Timezone</Label>
              <select
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {TIMEZONES.map((tz) => (
                  <option key={tz} value={tz}>{tz}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="space-y-2">
            <Label>Agent ID <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Input value={agentId} onChange={(e) => setAgentId(e.target.value)} placeholder="default" />
          </div>

          <div className="space-y-2">
            <Label>Prompt <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Write a social post about..."
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label>Target Pages</Label>
            {allPages.length === 0 ? (
              <p className="text-sm text-muted-foreground">No pages found. Add pages in the Pages tab first.</p>
            ) : (
              <div className="space-y-1.5 max-h-40 overflow-y-auto rounded-md border p-2">
                {allPages.map((page) => (
                  <label
                    key={page.id}
                    className="flex items-center gap-2 cursor-pointer text-sm hover:bg-muted/50 rounded px-1 py-0.5"
                  >
                    <Checkbox
                      checked={selectedPageIds.includes(page.page_id)}
                      onCheckedChange={() => togglePage(page.page_id)}
                    />
                    <span className="flex-1">{page.page_name || page.page_id}</span>
                    <span className="text-xs text-muted-foreground">{page._platform}</span>
                  </label>
                ))}
              </div>
            )}
            {selectedPageIds.length > 0 && (
              <p className="text-xs text-muted-foreground">
                {selectedPageIds.length} page{selectedPageIds.length !== 1 ? "s" : ""} selected
              </p>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={saving || !name.trim() || !cronExpr.trim()}>
            {saving ? (isEdit ? "Saving..." : "Creating...") : (isEdit ? "Save" : "Create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
