import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
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
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { CronJob, CronSchedule } from "./hooks/use-cron";
import { slugify, isValidSlug } from "@/lib/slug";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/use-auth-store";
import { useAgents } from "@/pages/agents/hooks/use-agents";

interface PairedDevice {
  sender_id: string;
  channel: string;
  chat_id: string;
}

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

/** Encode multiple recipients as comma-separated "channel::chatId" pairs */
function encodeRecipients(selected: string[]): { channel: string; to: string } {
  const channels = new Set<string>();
  const tos: string[] = [];
  for (const s of selected) {
    const [ch, id] = s.split("::");
    if (ch && id) {
      channels.add(ch);
      tos.push(id);
    }
  }
  return { channel: [...channels].join(","), to: tos.join(",") };
}

/** Decode stored channel/to back to selection keys */
function decodeRecipients(channel: string, to: string): string[] {
  if (!channel || !to) return [];
  const channels = channel.split(",");
  const tos = to.split(",");
  // If single channel, pair with all tos
  if (channels.length === 1) {
    return tos.map((t) => `${channels[0]}::${t}`);
  }
  // Multi channel: zip
  return tos.map((t, i) => `${channels[i] || channels[0]}::${t}`);
}

export function CronFormDialog({ open, onOpenChange, editJob, onSubmit }: CronFormDialogProps) {
  const { t } = useTranslation("cron");
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  const [agentId, setAgentId] = useState("");
  const [scheduleKind, setScheduleKind] = useState<ScheduleKind>("every");
  const [everyValue, setEveryValue] = useState("60");
  const [cronExpr, setCronExpr] = useState("0 * * * *");
  const [deliver, setDeliver] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);

  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  const { agents } = useAgents();
  const { data: pairedDevices = [] } = useQuery({
    queryKey: ["paired-devices"],
    queryFn: async () => {
      const res = await ws.call<{ paired: PairedDevice[] }>(Methods.PAIRING_LIST, {});
      return res.paired ?? [];
    },
    enabled: connected && open,
  });

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
      setSelected(decodeRecipients(editJob.payload?.channel ?? "", editJob.payload?.to ?? ""));
    } else {
      setName("");
      setMessage("");
      setAgentId("");
      setScheduleKind("every");
      setEveryValue("60");
      setCronExpr("0 * * * *");
      setDeliver(false);
      setSelected([]);
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

    const { channel, to } = encodeRecipients(selected);

    setSaving(true);
    try {
      await onSubmit({
        name: name.trim(),
        schedule,
        message: message.trim(),
        agentId: agentId.trim() || undefined,
        deliver: deliver && selected.length > 0,
        channel: deliver ? channel : undefined,
        to: deliver ? to : undefined,
      });
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  const toggleDevice = (key: string) => {
    setSelected((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key],
    );
  };

  const devices = pairedDevices.filter((d) => d.chat_id != null && d.chat_id !== "");
  const isEdit = !!editJob;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Cron Job" : "Create Cron Job"}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 -mx-4 px-4 sm:-mx-6 sm:px-6 overflow-y-auto min-h-0">
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
            <Label>Agent</Label>
            <Select value={agentId || "__default__"} onValueChange={(v) => setAgentId(v === "__default__" ? "" : v)}>
              <SelectTrigger>
                <SelectValue placeholder="Select agent" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__default__">Default agent</SelectItem>
                {agents.map((a) => (
                  <SelectItem key={a.id} value={a.id}>
                    {a.display_name || a.agent_key || a.id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("create.scheduleType")}</Label>
            <div className="flex gap-2">
              {(["every", "cron", "at"] as const).map((kind) => (
                <Button
                  key={kind}
                  variant={scheduleKind === kind ? "default" : "outline"}
                  size="sm"
                  onClick={() => setScheduleKind(kind)}
                >
                  {kind === "every" ? t("create.every") : kind === "cron" ? t("create.cron") : t("create.once")}
                </Button>
              ))}
            </div>
          </div>

          {scheduleKind === "every" && (
            <div className="space-y-2">
              <Label>{t("create.intervalSeconds")}</Label>
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
              <Label>{t("create.cronExpression")}</Label>
              <Input
                value={cronExpr}
                onChange={(e) => setCronExpr(e.target.value)}
                placeholder="0 * * * *"
              />
              <p className="text-xs text-muted-foreground">{t("create.cronHint")}</p>
            </div>
          )}

          {scheduleKind === "at" && (
            <p className="text-sm text-muted-foreground">
              {t("create.onceDesc")}
            </p>
          )}

          <div className="space-y-2">
            <Label>{t("create.message")}</Label>
            <Textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder={t("create.messagePlaceholder")}
              rows={3}
            />
          </div>

          {/* Delivery settings */}
          <div className="space-y-3 rounded-md border p-3">
            <div className="flex items-center justify-between">
              <div>
                <Label>Send result to channel</Label>
                <p className="text-xs text-muted-foreground">Deliver agent response to bots/channels</p>
              </div>
              <Switch checked={deliver} onCheckedChange={setDeliver} />
            </div>

            {deliver && (
              <div className="space-y-2">
                <Label>Deliver to</Label>
                {devices.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No paired devices found. Pair a bot first.</p>
                ) : (
                  <div className="space-y-1.5 max-h-40 overflow-y-auto rounded-md border p-2">
                    {devices.map((d) => {
                      const key = `${d.channel}::${d.chat_id}`;
                      const chatId = String(d.chat_id ?? "");
                      const label = chatId.startsWith("-")
                        ? `Group ${chatId}`
                        : `User ${d.sender_id}`;
                      return (
                        <label key={key} className="flex items-center gap-2 cursor-pointer text-sm hover:bg-muted/50 rounded px-1 py-0.5">
                          <Checkbox
                            checked={selected.includes(key)}
                            onCheckedChange={() => toggleDevice(key)}
                          />
                          <span className="flex-1">{label}</span>
                          <span className="text-xs text-muted-foreground">{d.channel}</span>
                        </label>
                      );
                    })}
                  </div>
                )}
                {selected.length > 0 && (
                  <p className="text-xs text-muted-foreground">
                    {selected.length} recipient{selected.length > 1 ? "s" : ""} selected
                  </p>
                )}
              </div>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t("create.cancel")}
          </Button>
          <Button onClick={handleSubmit} disabled={saving || !name.trim() || (!isEdit && !isValidSlug(name.trim())) || !message.trim()}>
            {saving ? (isEdit ? "Saving..." : "Creating...") : (isEdit ? "Save" : "Create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
