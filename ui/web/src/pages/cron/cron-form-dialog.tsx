import { useEffect, useState } from "react";
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
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { CronJob, CronSchedule } from "./hooks/use-cron";
import { slugify, isValidSlug } from "@/lib/slug";
import { useChannelInstances } from "@/pages/channels/hooks/use-channel-instances";

interface CronFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editJob?: CronJob | null;
  onSubmit: (data: {
    name: string;
    schedule: CronSchedule;
    message: string;
    agentId?: string;
    deliver?: boolean;
    channel?: string;
    to?: string;
  }) => Promise<void>;
}

type ScheduleKind = "every" | "cron" | "at";

export function CronFormDialog({ open, onOpenChange, editJob, onSubmit }: CronFormDialogProps) {
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  const [agentId, setAgentId] = useState("");
  const [scheduleKind, setScheduleKind] = useState<ScheduleKind>("every");
  const [everyValue, setEveryValue] = useState("60");
  const [cronExpr, setCronExpr] = useState("0 * * * *");
  const [deliver, setDeliver] = useState(false);
  const [channel, setChannel] = useState("");
  const [to, setTo] = useState("");
  const [saving, setSaving] = useState(false);

  const { instances: channelInstances } = useChannelInstances({ limit: 50 });

  // Populate form when editing
  useEffect(() => {
    if (!open) return;
    if (editJob) {
      setName(editJob.name);
      setMessage(editJob.payload?.message ?? "");
      setAgentId(editJob.agentId ?? "");
      setScheduleKind(editJob.schedule.kind as ScheduleKind);
      setEveryValue(editJob.schedule.everyMs ? String(editJob.schedule.everyMs / 1000) : "60");
      setCronExpr(editJob.schedule.expr ?? "0 * * * *");
      setDeliver(editJob.payload?.deliver ?? false);
      setChannel(editJob.payload?.channel ?? "");
      setTo(editJob.payload?.to ?? "");
    } else {
      setName("");
      setMessage("");
      setAgentId("");
      setScheduleKind("every");
      setEveryValue("60");
      setCronExpr("0 * * * *");
      setDeliver(false);
      setChannel("");
      setTo("");
    }
  }, [open, editJob]);

  const handleSubmit = async () => {
    if (!name.trim() || !message.trim()) return;

    let schedule: CronSchedule;
    if (scheduleKind === "every") {
      schedule = { kind: "every", everyMs: Number(everyValue) * 1000 };
    } else if (scheduleKind === "cron") {
      schedule = { kind: "cron", expr: cronExpr };
    } else {
      schedule = { kind: "at", atMs: Date.now() + 60000 };
    }

    setSaving(true);
    try {
      await onSubmit({
        name: name.trim(),
        schedule,
        message: message.trim(),
        agentId: agentId.trim() || undefined,
        deliver,
        channel: deliver ? channel : undefined,
        to: deliver ? to : undefined,
      });
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  const isEdit = !!editJob;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Cron Job" : "Create Cron Job"}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 px-0.5 -mx-0.5 overflow-y-auto min-h-0">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(slugify(e.target.value))}
              placeholder="my-daily-task"
              disabled={isEdit}
            />
            {!isEdit && (
              <p className="text-xs text-muted-foreground">Lowercase letters, numbers, and hyphens only</p>
            )}
          </div>

          <div className="space-y-2">
            <Label>Agent ID (optional)</Label>
            <Input value={agentId} onChange={(e) => setAgentId(e.target.value)} placeholder="default" />
          </div>

          <div className="space-y-2">
            <Label>Schedule Type</Label>
            <div className="flex gap-2">
              {(["every", "cron", "at"] as const).map((kind) => (
                <Button
                  key={kind}
                  variant={scheduleKind === kind ? "default" : "outline"}
                  size="sm"
                  onClick={() => setScheduleKind(kind)}
                >
                  {kind === "every" ? "Every" : kind === "cron" ? "Cron" : "Once"}
                </Button>
              ))}
            </div>
          </div>

          {scheduleKind === "every" && (
            <div className="space-y-2">
              <Label>Interval (seconds)</Label>
              <Input
                type="number"
                min={1}
                value={everyValue}
                onChange={(e) => setEveryValue(e.target.value)}
                placeholder="60"
              />
            </div>
          )}

          {scheduleKind === "cron" && (
            <div className="space-y-2">
              <Label>Cron Expression</Label>
              <Input
                value={cronExpr}
                onChange={(e) => setCronExpr(e.target.value)}
                placeholder="0 * * * *"
              />
              <p className="text-xs text-muted-foreground">Standard 5-field cron: min hour day month weekday</p>
            </div>
          )}

          {scheduleKind === "at" && (
            <p className="text-sm text-muted-foreground">
              The job will run once, approximately 1 minute from now.
            </p>
          )}

          <div className="space-y-2">
            <Label>Message</Label>
            <Textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="What should the agent do?"
              rows={3}
            />
          </div>

          {/* Delivery settings */}
          <div className="space-y-3 rounded-md border p-3">
            <div className="flex items-center justify-between">
              <div>
                <Label>Send result to channel</Label>
                <p className="text-xs text-muted-foreground">Deliver agent response to a bot/channel</p>
              </div>
              <Switch checked={deliver} onCheckedChange={setDeliver} />
            </div>

            {deliver && (
              <>
                <div className="space-y-2">
                  <Label>Channel</Label>
                  <Select value={channel} onValueChange={setChannel}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select channel..." />
                    </SelectTrigger>
                    <SelectContent>
                      {channelInstances.map((ch) => (
                        <SelectItem key={ch.id} value={ch.channel_type}>
                          {ch.name || ch.channel_type} ({ch.channel_type})
                        </SelectItem>
                      ))}
                      <SelectItem value="telegram">Telegram</SelectItem>
                      <SelectItem value="discord">Discord</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Chat ID / Recipient</Label>
                  <Input
                    value={to}
                    onChange={(e) => setTo(e.target.value)}
                    placeholder="e.g. 7690222162"
                  />
                  <p className="text-xs text-muted-foreground">Telegram user/group ID, Discord channel ID, etc.</p>
                </div>
              </>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={saving || !name.trim() || (!isEdit && !isValidSlug(name.trim())) || !message.trim()}>
            {saving ? (isEdit ? "Saving..." : "Creating...") : (isEdit ? "Save" : "Create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
