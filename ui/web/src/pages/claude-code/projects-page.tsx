import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router";
import { Plus, Terminal } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { SearchInput } from "@/components/shared/search-input";
import { Pagination } from "@/components/shared/pagination";
import { CardSkeleton } from "@/components/shared/loading-skeleton";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { usePagination } from "@/hooks/use-pagination";
import { useClaudeCode } from "./hooks/use-claude-code";
import { ProjectCard } from "./project-card";
import { ProjectCreateDialog } from "./project-create-dialog";
import { ProjectDetailPage } from "./project-detail-page";
import { ROUTES } from "@/lib/constants";

export function ProjectsPage() {
  const { id: detailId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { projects, loading, loadProjects, createProject, deleteProject } = useClaudeCode();
  const showSkeleton = useDeferredLoading(loading && projects.length === 0);

  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);

  useEffect(() => { loadProjects(); }, [loadProjects]);

  if (detailId) {
    return (
      <ProjectDetailPage
        projectId={detailId}
        onBack={() => navigate(ROUTES.CC_PROJECTS)}
      />
    );
  }

  const filtered = projects.filter((p) => {
    const q = search.toLowerCase();
    return (
      p.name.toLowerCase().includes(q) ||
      p.slug.toLowerCase().includes(q) ||
      (p.description ?? "").toLowerCase().includes(q) ||
      p.work_dir.toLowerCase().includes(q)
    );
  });

  const { pageItems, pagination, setPage, setPageSize, resetPage } = usePagination(filtered);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { resetPage(); }, [search]);

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

      <div className="mt-4">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder="Search projects..."
          className="max-w-sm"
        />
      </div>

      <div className="mt-6">
        {showSkeleton ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => <CardSkeleton key={i} />)}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={Terminal}
            title={search ? "No matching projects" : "No projects yet"}
            description={
              search
                ? "Try a different search term."
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
                  onClick={() => navigate(`${ROUTES.CC_PROJECTS}/${project.id}`)}
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
