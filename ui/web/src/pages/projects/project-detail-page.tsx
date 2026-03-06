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
import { useProjects } from "./hooks/use-projects";
import { useTeams } from "../teams/hooks/use-teams";
import { SessionStartDialog } from "./session-start-dialog";
import { ROUTES } from "@/lib/constants";
import type { Project, ProjectSession } from "@/types/project";

interface ProjectDetailPageProps {
  projectId: string;
  onBack: () => void;
}

const KNOWN_TOOLS = ["Read", "Edit", "Write", "Bash", "Glob", "Grep", "WebSearch", "WebFetch", "Agent"];

const MODEL_OPTIONS = [
  { value: "", label: "Default (auto)" },
  { value: "claude-sonnet-4-5-20250514", label: "claude-sonnet-4-5-20250514" },
  { value: "claude-opus-4-5-20250414", label: "claude-opus-4-5-20250414" },
  { value: "claude-haiku-4-5-20251001", label: "claude-haiku-4-5-20251001" },
];

/** Parse allowed_tools into checked known tools + extra string */
function parseAllowedTools(tools: string[]): { checked: Set<string>; extra: string } {
  const checked = new Set<string>();
  const extra: string[] = [];
  for (const t of tools) {
    if (KNOWN_TOOLS.includes(t)) checked.add(t);
    else extra.push(t);
  }
  return { checked, extra: extra.join(", ") };
}

/** Build allowed_tools array from checked set + extra string */
function buildAllowedTools(checked: Set<string>, extra: string): string[] {
  const extraList = extra.split(",").map((t) => t.trim()).filter(Boolean);
  return [...KNOWN_TOOLS.filter((t) => checked.has(t)), ...extraList];
}

interface ToolsInputProps {
  tools: string[];
  onChange: (tools: string[]) => void;
}

function ToolsInput({ tools, onChange }: ToolsInputProps) {
  const { checked, extra } = parseAllowedTools(tools);
  const [extraInput, setExtraInput] = useState(extra);

  const toggleTool = (tool: string) => {
    const next = new Set(checked);
    if (next.has(tool)) next.delete(tool);
    else next.add(tool);
    onChange(buildAllowedTools(next, extraInput));
  };

  const handleExtraChange = (val: string) => {
    setExtraInput(val);
    onChange(buildAllowedTools(checked, val));
  };

  return (
    <div className="space-y-2">
      <div className="grid grid-cols-3 gap-1.5">
        {KNOWN_TOOLS.map((tool) => (
          <label key={tool} className="flex items-center gap-1.5 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={checked.has(tool)}
              onChange={() => toggleTool(tool)}
              className="h-3.5 w-3.5 rounded border-input accent-primary"
            />
            <span className="text-sm">{tool}</span>
          </label>
        ))}
      </div>
      <Input
        value={extraInput}
        onChange={(e) => handleExtraChange(e.target.value)}
        placeholder="Additional tools (comma-separated)"
        className="text-sm"
      />
      <p className="text-xs text-muted-foreground">Leave all unchecked to allow all tools.</p>
    </div>
  );
}

function SessionRow({
  session,
  onClick,
  onDelete,
}: {
  session: ProjectSession;
  onClick: () => void;
  onDelete: (e: React.MouseEvent) => void;
}) {
  const statusColor: Record<string, string> = {
    running: "success", starting: "success", stopped: "secondary",
    completed: "secondary", failed: "destructive",
  };
  return (
    <div className="flex items-center gap-2 w-full rounded-lg border p-3 hover:border-primary/30 hover:bg-muted/30 transition-all">
      <button
        type="button"
        onClick={onClick}
        className="flex items-center gap-3 min-w-0 flex-1 text-left"
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
      <button
        type="button"
        onClick={onDelete}
        title="Delete session"
        className="p-1.5 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors shrink-0"
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

type SessionStatusFilter = "all" | "running" | "completed" | "failed" | "stopped";

function SessionsTab({ project, projectId }: { project: Project; projectId: string }) {
  const navigate = useNavigate();
  const { listSessions, startSession, deleteSession } = useProjects();
  const [sessions, setSessions] = useState<ProjectSession[]>([]);
  const [startOpen, setStartOpen] = useState(false);
  const [statusFilter, setStatusFilter] = useState<SessionStatusFilter>("all");

  const reload = useCallback(async () => {
    try { setSessions(await listSessions(projectId)); } catch { /* ignore */ }
  }, [projectId, listSessions]);

  useEffect(() => { reload(); }, [reload]);

  const handleStart = async (prompt: string, label?: string) => {
    const s = await startSession(projectId, prompt, label);
    await reload();
    navigate(ROUTES.PROJECTS_SESSION.replace(":id", s.id));
  };

  const handleDelete = async (e: React.MouseEvent, sessionId: string) => {
    e.stopPropagation();
    if (!window.confirm("Delete this session? This cannot be undone.")) return;
    try {
      await deleteSession(sessionId);
      await reload();
    } catch { /* toast shown by hook */ }
  };

  const activeSessions = sessions.filter((s) => s.status === "running" || s.status === "starting").length;
  const canStart = activeSessions < project.max_sessions;

  const filterOptions: { value: SessionStatusFilter; label: string }[] = [
    { value: "all", label: "All" },
    { value: "running", label: "Running" },
    { value: "completed", label: "Completed" },
    { value: "failed", label: "Failed" },
    { value: "stopped", label: "Stopped" },
  ];

  const filtered = statusFilter === "all"
    ? sessions
    : sessions.filter((s) => {
        if (statusFilter === "running") return s.status === "running" || s.status === "starting";
        return s.status === statusFilter;
      });

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

      {sessions.length > 0 && (
        <div className="flex items-center gap-1.5 flex-wrap">
          {filterOptions.map((opt) => {
            const count = opt.value === "all"
              ? sessions.length
              : sessions.filter((s) => {
                  if (opt.value === "running") return s.status === "running" || s.status === "starting";
                  return s.status === opt.value;
                }).length;
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => setStatusFilter(opt.value)}
                className={`px-2.5 py-1 rounded-full text-xs font-medium transition-colors ${
                  statusFilter === opt.value
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground hover:bg-muted/80"
                }`}
              >
                {opt.label} {count > 0 && `(${count})`}
              </button>
            );
          })}
        </div>
      )}

      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground py-6 text-center">
          {sessions.length === 0 ? "No sessions yet. Start one!" : "No sessions match the filter."}
        </p>
      ) : (
        <div className="space-y-2">
          {filtered.map((s) => (
            <SessionRow
              key={s.id}
              session={s}
              onClick={() => navigate(ROUTES.PROJECTS_SESSION.replace(":id", s.id))}
              onDelete={(e) => handleDelete(e, s.id)}
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
  project: Project;
  onDeleted: () => void;
  onSaved: () => void;
}) {
  const { updateProject, deleteProject } = useProjects();
  const { teams, load: loadTeams } = useTeams();
  const [name, setName] = useState(project.name);
  const [slug, setSlug] = useState(project.slug);
  const [description, setDescription] = useState(project.description ?? "");
  const [workDir, setWorkDir] = useState(project.work_dir);
  const [maxSessions, setMaxSessions] = useState(String(project.max_sessions));
  const [allowedTools, setAllowedTools] = useState<string[]>(project.allowed_tools ?? []);
  const [model, setModel] = useState<string>((project.claude_config?.model as string) ?? "");
  const [teamId, setTeamId] = useState(project.team_id ?? "none");
  const [status, setStatus] = useState(project.status);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  useEffect(() => { loadTeams(); }, [loadTeams]);

  const hasChanges =
    name.trim() !== project.name ||
    slug !== project.slug ||
    (description.trim() || "") !== (project.description ?? "") ||
    workDir.trim() !== project.work_dir ||
    (parseInt(maxSessions, 10) || 3) !== project.max_sessions ||
    JSON.stringify(allowedTools.slice().sort()) !== JSON.stringify((project.allowed_tools ?? []).slice().sort()) ||
    model !== ((project.claude_config?.model as string) ?? "") ||
    (teamId === "none" ? undefined : teamId) !== (project.team_id ?? undefined) ||
    status !== project.status;

  const handleSave = async () => {
    setSaving(true);
    try {
      const resolvedTeamId = teamId === "none" ? null : teamId;
      const claudeConfig = model
        ? { ...(project.claude_config ?? {}), model }
        : (() => {
            const cfg = { ...(project.claude_config ?? {}) };
            delete cfg.model;
            return cfg;
          })();
      await updateProject(projectId, {
        name: name.trim(),
        slug: slug.trim(),
        description: description.trim() || undefined,
        work_dir: workDir.trim(),
        max_sessions: parseInt(maxSessions, 10) || 3,
        allowed_tools: allowedTools,
        team_id: resolvedTeamId as string | undefined,
        status,
        claude_config: claudeConfig,
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
          <Label>Allowed Tools</Label>
          <ToolsInput tools={allowedTools} onChange={setAllowedTools} />
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
  const { getProject } = useProjects();
  const { teams, load: loadTeams } = useTeams();
  const [project, setProject] = useState<Project | null>(null);
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
              onDeleted={() => navigate(ROUTES.PROJECTS)}
              onSaved={reload}
            />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
