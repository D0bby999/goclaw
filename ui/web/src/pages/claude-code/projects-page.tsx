import { useState, useEffect, useMemo } from "react";
import { useParams, useNavigate } from "react-router";
import { Plus, Terminal, Users } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { SearchInput } from "@/components/shared/search-input";
import { Pagination } from "@/components/shared/pagination";
import { CardSkeleton } from "@/components/shared/loading-skeleton";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { usePagination } from "@/hooks/use-pagination";
import { useClaudeCode } from "./hooks/use-claude-code";
import { useTeams } from "../teams/hooks/use-teams";
import { ProjectCard } from "./project-card";
import { ProjectCreateDialog } from "./project-create-dialog";
import { ProjectDetailPage } from "./project-detail-page";
import { ROUTES } from "@/lib/constants";

export function ProjectsPage() {
  const { id: detailId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { projects, loading, error, createProject, deleteProject } = useClaudeCode();
  const { teams, load: loadTeams } = useTeams();
  const showSkeleton = useDeferredLoading(loading && projects.length === 0);

  const [search, setSearch] = useState("");
  const [teamFilter, setTeamFilter] = useState("all");
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);

  // Load teams once for filter + create dialog
  useEffect(() => { loadTeams(); }, [loadTeams]);

  // Build team name lookup map
  const teamNameMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const t of teams) map.set(t.id, t.name);
    return map;
  }, [teams]);

  if (detailId) {
    return (
      <ProjectDetailPage
        projectId={detailId}
        onBack={() => navigate(ROUTES.CC_PROJECTS)}
      />
    );
  }

  const filtered = projects.filter((p) => {
    // Team filter
    if (teamFilter === "personal" && p.team_id) return false;
    if (teamFilter !== "all" && teamFilter !== "personal" && p.team_id !== teamFilter) return false;

    // Search filter
    const q = search.toLowerCase();
    if (!q) return true;
    return (
      p.name.toLowerCase().includes(q) ||
      p.slug.toLowerCase().includes(q) ||
      (p.description ?? "").toLowerCase().includes(q) ||
      p.work_dir.toLowerCase().includes(q) ||
      (p.team_id ? (teamNameMap.get(p.team_id) ?? "").toLowerCase().includes(q) : false)
    );
  });

  const { pageItems, pagination, setPage, setPageSize, resetPage } = usePagination(filtered);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { resetPage(); }, [search, teamFilter]);

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title="Claude Code"
        description="Manage Claude Code orchestration projects and sessions"
        actions={
          <Button onClick={() => setCreateOpen(true)} className="gap-1">
            <Plus className="h-4 w-4" /> New Project
          </Button>
        }
      />

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder="Search projects..."
          className="max-w-sm"
        />
        {teams.length > 0 && (
          <Select value={teamFilter} onValueChange={setTeamFilter}>
            <SelectTrigger className="w-[180px]">
              <Users className="mr-2 h-4 w-4" />
              <SelectValue placeholder="All teams" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All projects</SelectItem>
              <SelectItem value="personal">Personal only</SelectItem>
              {teams.map((t) => (
                <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      <div className="mt-6">
        {error && (
          <div className="mb-4 rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </div>
        )}
        {showSkeleton ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => <CardSkeleton key={i} />)}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={Terminal}
            title={search || teamFilter !== "all" ? "No matching projects" : "No projects yet"}
            description={
              search || teamFilter !== "all"
                ? "Try a different search term or filter."
                : "Create your first Claude Code project to get started."
            }
          />
        ) : (
          <>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {pageItems.map((project) => (
                <ProjectCard
                  key={project.id}
                  project={project}
                  teamName={project.team_id ? teamNameMap.get(project.team_id) : undefined}
                  onClick={() => navigate(`${ROUTES.CC_PROJECTS}/${project.id}`)}
                  onEdit={() => navigate(`${ROUTES.CC_PROJECTS}/${project.id}?tab=settings`)}
                  onDelete={() => setDeleteTarget({ id: project.id, name: project.name })}
                />
              ))}
            </div>
            <div className="mt-4">
              <Pagination
                page={pagination.page}
                pageSize={pagination.pageSize}
                total={pagination.total}
                totalPages={pagination.totalPages}
                onPageChange={setPage}
                onPageSizeChange={setPageSize}
              />
            </div>
          </>
        )}
      </div>

      <ProjectCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        teams={teams}
        onCreate={async (data) => {
          await createProject(data);
        }}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={() => setDeleteTarget(null)}
        title="Delete Project"
        description={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={async () => {
          if (deleteTarget) {
            await deleteProject(deleteTarget.id);
            setDeleteTarget(null);
          }
        }}
      />
    </div>
  );
}
