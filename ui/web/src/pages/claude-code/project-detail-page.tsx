import { useState, useEffect, useCallback } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { ArrowLeft, Terminal, Plus, Save, Check, Trash2, Users } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import { DeferredSpinner } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useClaudeCode } from "./hooks/use-claude-code";
import { useTeams } from "../teams/hooks/use-teams";
import { SessionStartDialog } from "./session-start-dialog";
import { ROUTES } from "@/lib/constants";
import type { CCProject, CCSession } from "@/types/claude-code";

interface ProjectDetailPageProps {
  projectId: string;
  onBack: () => void;
}

function SessionRow({ session, onClick }: { session: CCSession; onClick: () => void }) {
  const statusColor: Record<string, string> = {
    running: "success", starting: "success", stopped: "secondary",
    completed: "secondary", failed: "destructive",
  };
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-3 w-full rounded-lg border p-3 text-left hover:border-primary/30 hover:bg-muted/30 transition-all"
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium truncate">
            {session.label ?? session.id.slice(0, 12)}
          </span>
          <Badge variant={(statusColor[session.status] as "success" | "secondary" | "destructive") ?? "secondary"} className="text-[11px] shrink-0">
            {session.status}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground mt-0.5">
          {new Date(session.started_at).toLocaleString()}
          {session.cost_usd > 0 && ` · $${session.cost_usd.toFixed(4)}`}
        </p>
      </div>
      <Terminal className="h-4 w-4 text-muted-foreground shrink-0" />
    </button>
  );
}

function SessionsTab({ project, projectId }: { project: CCProject; projectId: string }) {
  const navigate = useNavigate();
  const { listSessions, startSession } = useClaudeCode();
  const [sessions, setSessions] = useState<CCSession[]>([]);
  const [startOpen, setStartOpen] = useState(false);

  const reload = useCallback(async () => {
    try { setSessions(await listSessions(projectId)); } catch { /* ignore */ }
  }, [projectId, listSessions]);

  useEffect(() => { reload(); }, [reload]);

  const handleStart = async (prompt: string, label?: string) => {
    const s = await startSession(projectId, prompt, label);
    await reload();
    navigate(ROUTES.CC_SESSION.replace(":id", s.id));
  };

  const activeSessions = sessions.filter((s) => s.status === "running" || s.status === "starting").length;
  const canStart = activeSessions < project.max_sessions;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {activeSessions} active / {project.max_sessions} max
        </p>
        <Button size="sm" className="gap-1" onClick={() => setStartOpen(true)} disabled={!canStart}>
          <Plus className="h-4 w-4" /> New Session
        </Button>
      </div>

      {sessions.length === 0 ? (
        <p className="text-sm text-muted-foreground py-6 text-center">No sessions yet. Start one!</p>
      ) : (
        <div className="space-y-2">
          {sessions.map((s) => (
            <SessionRow
              key={s.id}
              session={s}
              onClick={() => navigate(ROUTES.CC_SESSION.replace(":id", s.id))}
            />
          ))}
        </div>
      )}

      <SessionStartDialog
        open={startOpen}
        onOpenChange={setStartOpen}
        onStart={handleStart}
      />
    </div>
  );
}

function SettingsTab({ projectId, project, onDeleted, onSaved }: {
  projectId: string;
  project: CCProject;
  onDeleted: () => void;
  onSaved: () => void;
}) {
  const { updateProject, deleteProject } = useClaudeCode();
  const { teams, load: loadTeams } = useTeams();
  const [name, setName] = useState(project.name);
  const [slug, setSlug] = useState(project.slug);
  const [description, setDescription] = useState(project.description ?? "");
  const [workDir, setWorkDir] = useState(project.work_dir);
  const [maxSessions, setMaxSessions] = useState(String(project.max_sessions));
  const [allowedTools, setAllowedTools] = useState((project.allowed_tools ?? []).join(", "));
  const [teamId, setTeamId] = useState(project.team_id ?? "none");
  const [status, setStatus] = useState(project.status);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  useEffect(() => { loadTeams(); }, [loadTeams]);

  // Detect if form has changes
  const hasChanges =
    name.trim() !== project.name ||
    slug !== project.slug ||
    (description.trim() || "") !== (project.description ?? "") ||
    workDir.trim() !== project.work_dir ||
    (parseInt(maxSessions, 10) || 3) !== project.max_sessions ||
    allowedTools.trim() !== (project.allowed_tools ?? []).join(", ") ||
    (teamId === "none" ? undefined : teamId) !== (project.team_id ?? undefined) ||
    status !== project.status;

  const handleSave = async () => {
    setSaving(true);
    try {
      const resolvedTeamId = teamId === "none" ? null : teamId;
      await updateProject(projectId, {
        name: name.trim(),
        slug: slug.trim(),
        description: description.trim() || undefined,
        work_dir: workDir.trim(),
        max_sessions: parseInt(maxSessions, 10) || 3,
        allowed_tools: allowedTools.trim() ? allowedTools.split(",").map((t) => t.trim()).filter(Boolean) : [],
        team_id: resolvedTeamId as string | undefined,
        status,
      });
      setSaved(true);
      onSaved();
      setTimeout(() => setSaved(false), 3000);
    } catch { /* ignore */ } finally { setSaving(false); }
  };

  const handleDelete = async () => {
    await deleteProject(projectId);
    onDeleted();
  };

  return (
    <div className="space-y-6 max-w-lg">
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label>Slug</Label>
          <Input value={slug} onChange={(e) => setSlug(e.target.value)} />
          <p className="text-xs text-muted-foreground">Lowercase, hyphens only. Used as unique identifier.</p>
        </div>
        <div className="space-y-1.5">
          <Label>Description</Label>
          <Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </div>
        <div className="space-y-1.5">
          <Label>Work Directory</Label>
          <Input value={workDir} onChange={(e) => setWorkDir(e.target.value)} />
        </div>
        {teams.length > 0 && (
          <div className="space-y-1.5">
            <Label>Team</Label>
            <Select value={teamId} onValueChange={setTeamId}>
              <SelectTrigger>
                <SelectValue placeholder="No team" />
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
          <Label>Status</Label>
          <Select value={status} onValueChange={(v) => setStatus(v as "active" | "archived")}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">Active</SelectItem>
              <SelectItem value="archived">Archived</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>Max Concurrent Sessions</Label>
          <Input type="number" min={1} max={20} value={maxSessions} onChange={(e) => setMaxSessions(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label>Allowed Tools (comma-separated)</Label>
          <Input value={allowedTools} onChange={(e) => setAllowedTools(e.target.value)} placeholder="Read,Edit,Bash" />
        </div>
      </div>

      <Button onClick={handleSave} disabled={saving || !hasChanges} className="gap-2">
        {saving ? "Saving..." : saved ? <><Check className="h-4 w-4" /> Saved</> : <><Save className="h-4 w-4" /> Save</>}
      </Button>

      <div className="border-t pt-6">
        <h3 className="text-sm font-medium text-destructive mb-3">Danger Zone</h3>
        <Button variant="destructive" size="sm" className="gap-2" onClick={() => setDeleteOpen(true)}>
          <Trash2 className="h-4 w-4" /> Delete Project
        </Button>
      </div>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete Project"
        description={`Are you sure you want to delete "${project.name}"? All sessions will be stopped. This cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}

export function ProjectDetailPage({ projectId, onBack }: ProjectDetailPageProps) {
  const { getProject } = useClaudeCode();
  const { teams, load: loadTeams } = useTeams();
  const [project, setProject] = useState<CCProject | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const defaultTab = searchParams.get("tab") === "settings" ? "settings" : "sessions";

  useEffect(() => { loadTeams(); }, [loadTeams]);

  const reload = useCallback(async () => {
    try { setProject(await getProject(projectId)); } catch { /* ignore */ }
  }, [projectId, getProject]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getProject(projectId)
      .then((p) => { if (!cancelled) setProject(p); })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [projectId, getProject]);

  const teamName = project?.team_id
    ? teams.find((t) => t.id === project.team_id)?.name
    : undefined;

  if (loading || !project) {
    return (
      <div className="p-4 sm:p-6">
        <Button variant="ghost" onClick={onBack} className="mb-4 gap-1">
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>
        <DeferredSpinner />
      </div>
    );
  }

  return (
    <div className="p-4 sm:p-6">
      <div className="mb-6 flex items-start gap-4">
        <Button variant="ghost" size="icon" onClick={onBack} className="mt-0.5 shrink-0">
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <Terminal className="h-6 w-6" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-xl font-semibold">{project.name}</h2>
            <Badge variant={project.status === "active" ? "success" : "secondary"}>
              {project.status}
            </Badge>
            {teamName && (
              <Badge variant="outline" className="gap-1">
                <Users className="h-3 w-3" /> {teamName}
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground truncate mt-0.5">{project.work_dir}</p>
          {project.description && (
            <p className="text-sm text-muted-foreground/70 mt-1">{project.description}</p>
          )}
        </div>
      </div>

      <div className="max-w-4xl rounded-xl border bg-card p-3 shadow-sm sm:p-4">
        <Tabs defaultValue={defaultTab}>
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="sessions">Sessions</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
          </TabsList>
          <TabsContent value="sessions" className="mt-4">
            <SessionsTab project={project} projectId={projectId} />
          </TabsContent>
          <TabsContent value="settings" className="mt-4">
            <SettingsTab
              projectId={projectId}
              project={project}
              onDeleted={() => navigate(ROUTES.CC_PROJECTS)}
              onSaved={reload}
            />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
