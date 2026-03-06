import { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { TeamData } from "@/types/team";

interface ProjectCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  teams?: TeamData[];
  onCreate: (data: {
    name: string;
    slug: string;
    work_dir: string;
    description?: string;
    max_sessions: number;
    allowed_tools?: string[];
    team_id?: string;
    claude_config?: Record<string, unknown>;
  }) => Promise<void>;
}

const KNOWN_TOOLS = ["Read", "Edit", "Write", "Bash", "Glob", "Grep", "WebSearch", "WebFetch", "Agent"];

const MODEL_OPTIONS = [
  { value: "", label: "Default (auto)" },
  { value: "claude-sonnet-4-5-20250514", label: "claude-sonnet-4-5-20250514" },
  { value: "claude-opus-4-5-20250414", label: "claude-opus-4-5-20250414" },
  { value: "claude-haiku-4-5-20251001", label: "claude-haiku-4-5-20251001" },
];

function toSlug(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function ProjectCreateDialog({ open, onOpenChange, teams, onCreate }: ProjectCreateDialogProps) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [workDir, setWorkDir] = useState("");
  const [description, setDescription] = useState("");
  const [maxSessions, setMaxSessions] = useState("3");
  const [checkedTools, setCheckedTools] = useState<Set<string>>(new Set());
  const [extraTools, setExtraTools] = useState("");
  const [model, setModel] = useState("");
  const [teamId, setTeamId] = useState<string>("");
  const [slugEdited, setSlugEdited] = useState(false);
  const [loading, setLoading] = useState(false);

  // Auto-generate slug from name unless user has edited it
  useEffect(() => {
    if (!slugEdited) setSlug(toSlug(name));
  }, [name, slugEdited]);

  const toggleTool = (tool: string) => {
    setCheckedTools((prev) => {
      const next = new Set(prev);
      if (next.has(tool)) next.delete(tool);
      else next.add(tool);
      return next;
    });
  };

  const reset = () => {
    setName(""); setSlug(""); setWorkDir(""); setDescription("");
    setMaxSessions("3"); setCheckedTools(new Set()); setExtraTools("");
    setModel(""); setTeamId(""); setSlugEdited(false);
  };

  const handleCreate = async () => {
    if (!name.trim() || !workDir.trim()) return;
    setLoading(true);
    try {
      const knownSelected = KNOWN_TOOLS.filter((t) => checkedTools.has(t));
      const extraList = extraTools.split(",").map((t) => t.trim()).filter(Boolean);
      const tools = [...knownSelected, ...extraList];

      await onCreate({
        name: name.trim(),
        slug: slug || toSlug(name),
        work_dir: workDir.trim(),
        description: description.trim() || undefined,
        max_sessions: parseInt(maxSessions, 10) || 3,
        allowed_tools: tools.length > 0 ? tools : undefined,
        team_id: teamId && teamId !== "none" ? teamId : undefined,
        claude_config: model ? { model } : undefined,
      });
      reset();
      onOpenChange(false);
    } catch {
      // error handled upstream
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) reset(); onOpenChange(v); }}>
      <DialogContent className="max-h-[90vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>New Project</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2 overflow-y-auto min-h-0">
          <div className="space-y-1.5">
            <Label htmlFor="proj-name">Name *</Label>
            <Input
              id="proj-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. My App"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="proj-slug">Slug</Label>
            <Input
              id="proj-slug"
              value={slug}
              onChange={(e) => { setSlug(e.target.value); setSlugEdited(true); }}
              placeholder="my-app"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="proj-workdir">Work Directory *</Label>
            <Input
              id="proj-workdir"
              value={workDir}
              onChange={(e) => setWorkDir(e.target.value)}
              placeholder="/path/to/project"
            />
          </div>

          {teams && teams.length > 0 && (
            <div className="space-y-1.5">
              <Label>Team</Label>
              <Select value={teamId} onValueChange={setTeamId}>
                <SelectTrigger>
                  <SelectValue placeholder="No team (personal)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">No team (personal)</SelectItem>
                  {teams.map((t) => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">Assign to a team for shared access.</p>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="proj-desc">Description</Label>
            <Textarea
              id="proj-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description..."
              rows={2}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="proj-max">Max Concurrent Sessions</Label>
            <Input
              id="proj-max"
              type="number"
              min={1}
              max={20}
              value={maxSessions}
              onChange={(e) => setMaxSessions(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label>Default Model</Label>
            <Select value={model} onValueChange={setModel}>
              <SelectTrigger>
                <SelectValue placeholder="Default (auto)" />
              </SelectTrigger>
              <SelectContent>
                {MODEL_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Allowed Tools</Label>
            <div className="grid grid-cols-3 gap-1.5">
              {KNOWN_TOOLS.map((tool) => (
                <label key={tool} className="flex items-center gap-1.5 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={checkedTools.has(tool)}
                    onChange={() => toggleTool(tool)}
                    className="h-3.5 w-3.5 rounded border-input accent-primary"
                  />
                  <span className="text-sm">{tool}</span>
                </label>
              ))}
            </div>
            <Input
              value={extraTools}
              onChange={(e) => setExtraTools(e.target.value)}
              placeholder="Additional tools (comma-separated)"
              className="text-sm"
            />
            <p className="text-xs text-muted-foreground">Leave all unchecked to allow all tools.</p>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button
            onClick={handleCreate}
            disabled={!name.trim() || !workDir.trim() || loading}
          >
            {loading ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
