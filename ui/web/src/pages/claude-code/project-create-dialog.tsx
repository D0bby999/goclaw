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
  }) => Promise<void>;
}

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
  const [allowedTools, setAllowedTools] = useState("");
  const [teamId, setTeamId] = useState<string>("");
  const [slugEdited, setSlugEdited] = useState(false);
  const [loading, setLoading] = useState(false);

  // Auto-generate slug from name unless user has edited it
  useEffect(() => {
    if (!slugEdited) setSlug(toSlug(name));
  }, [name, slugEdited]);

  const reset = () => {
    setName(""); setSlug(""); setWorkDir(""); setDescription("");
    setMaxSessions("3"); setAllowedTools(""); setTeamId(""); setSlugEdited(false);
  };

  const handleCreate = async () => {
    if (!name.trim() || !workDir.trim()) return;
    setLoading(true);
    try {
      const tools = allowedTools.trim()
        ? allowedTools.split(",").map((t) => t.trim()).filter(Boolean)
        : undefined;
      await onCreate({
        name: name.trim(),
        slug: slug || toSlug(name),
        work_dir: workDir.trim(),
        description: description.trim() || undefined,
        max_sessions: parseInt(maxSessions, 10) || 3,
        allowed_tools: tools,
        team_id: teamId && teamId !== "none" ? teamId : undefined,
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
            <Label htmlFor="cc-name">Name *</Label>
            <Input
              id="cc-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. My App"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cc-slug">Slug</Label>
            <Input
              id="cc-slug"
              value={slug}
              onChange={(e) => { setSlug(e.target.value); setSlugEdited(true); }}
              placeholder="my-app"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cc-workdir">Work Directory *</Label>
            <Input
              id="cc-workdir"
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
            <Label htmlFor="cc-desc">Description</Label>
            <Textarea
              id="cc-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description..."
              rows={2}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cc-max">Max Concurrent Sessions</Label>
            <Input
              id="cc-max"
              type="number"
              min={1}
              max={20}
              value={maxSessions}
              onChange={(e) => setMaxSessions(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cc-tools">Allowed Tools (comma-separated)</Label>
            <Input
              id="cc-tools"
              value={allowedTools}
              onChange={(e) => setAllowedTools(e.target.value)}
              placeholder="Read,Edit,Bash,Glob"
            />
            <p className="text-xs text-muted-foreground">Leave empty to allow all tools.</p>
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
