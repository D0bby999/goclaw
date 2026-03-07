import { useState, useEffect } from "react";
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
import type { NewsSource } from "./hooks/use-news-sources";

const SOURCE_TYPES = ["rss", "reddit", "website", "twitter"] as const;
const INTERVALS = ["hourly", "daily", "weekly"] as const;

interface NewsSourceFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: {
    name: string;
    sourceType: string;
    config?: Record<string, unknown>;
    scrapeInterval?: string;
  }) => Promise<void>;
  onUpdate?: (id: string, patch: Record<string, unknown>) => Promise<void>;
  editSource?: NewsSource | null;
}

export function NewsSourceFormDialog({ open, onOpenChange, onSubmit, onUpdate, editSource }: NewsSourceFormDialogProps) {
  const [name, setName] = useState("");
  const [sourceType, setSourceType] = useState<string>("rss");
  const [scrapeInterval, setScrapeInterval] = useState<string>("daily");
  const [configUrl, setConfigUrl] = useState("");
  const [configSubreddit, setConfigSubreddit] = useState("");
  const [saving, setSaving] = useState(false);

  const isEdit = !!editSource;

  useEffect(() => {
    if (editSource) {
      setName(editSource.name);
      setSourceType(editSource.sourceType);
      setScrapeInterval(editSource.scrapeInterval || "daily");
      const cfg = editSource.config || {};
      setConfigUrl((cfg.url as string) || "");
      setConfigSubreddit((cfg.subreddit as string) || "");
    } else {
      setName("");
      setSourceType("rss");
      setScrapeInterval("daily");
      setConfigUrl("");
      setConfigSubreddit("");
    }
  }, [editSource, open]);

  const buildConfig = (): Record<string, unknown> => {
    if (sourceType === "reddit") return { subreddit: configSubreddit.trim() };
    if (sourceType === "rss" || sourceType === "website") return { url: configUrl.trim() };
    return {};
  };

  const canSubmit = name.trim() && sourceType && (
    sourceType === "twitter" ||
    (sourceType === "reddit" && configSubreddit.trim()) ||
    ((sourceType === "rss" || sourceType === "website") && configUrl.trim())
  );

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSaving(true);
    try {
      if (isEdit && onUpdate) {
        await onUpdate(editSource!.id, {
          name: name.trim(),
          source_type: sourceType,
          config: buildConfig(),
          scrape_interval: scrapeInterval,
        });
      } else {
        await onSubmit({
          name: name.trim(),
          sourceType,
          config: buildConfig(),
          scrapeInterval,
        });
      }
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Source" : "Add Source"}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 overflow-y-auto min-h-0">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="My RSS Feed" />
          </div>

          <div className="space-y-2">
            <Label>Type</Label>
            <div className="flex gap-2">
              {SOURCE_TYPES.map((t) => (
                <Button
                  key={t}
                  variant={sourceType === t ? "default" : "outline"}
                  size="sm"
                  onClick={() => setSourceType(t)}
                >
                  {t}
                </Button>
              ))}
            </div>
          </div>

          {(sourceType === "rss" || sourceType === "website") && (
            <div className="space-y-2">
              <Label>URL</Label>
              <Input value={configUrl} onChange={(e) => setConfigUrl(e.target.value)} placeholder="https://example.com/feed.xml" />
            </div>
          )}

          {sourceType === "reddit" && (
            <div className="space-y-2">
              <Label>Subreddit</Label>
              <Input value={configSubreddit} onChange={(e) => setConfigSubreddit(e.target.value)} placeholder="technology" />
              <p className="text-xs text-muted-foreground">Without the r/ prefix</p>
            </div>
          )}

          {sourceType === "twitter" && (
            <p className="text-sm text-muted-foreground">
              Twitter source configuration is handled by the agent automatically.
            </p>
          )}

          <div className="space-y-2">
            <Label>Scrape Interval</Label>
            <div className="flex gap-2">
              {INTERVALS.map((i) => (
                <Button
                  key={i}
                  variant={scrapeInterval === i ? "default" : "outline"}
                  size="sm"
                  onClick={() => setScrapeInterval(i)}
                >
                  {i}
                </Button>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={saving || !canSubmit}>
            {saving ? (isEdit ? "Saving..." : "Adding...") : (isEdit ? "Save" : "Add Source")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
