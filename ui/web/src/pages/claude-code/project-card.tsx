import { Terminal, Folder } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { CCProject } from "@/types/claude-code";

interface ProjectCardProps {
  project: CCProject;
  activeSessions?: number;
  onClick: () => void;
}

export function ProjectCard({ project, activeSessions = 0, onClick }: ProjectCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex cursor-pointer flex-col gap-3 rounded-lg border bg-card p-4 text-left transition-all hover:border-primary/30 hover:shadow-md"
    >
      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Terminal className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <span className="truncate text-sm font-semibold">{project.name}</span>
          {project.slug && (
            <p className="truncate text-xs text-muted-foreground">{project.slug}</p>
          )}
        </div>
        <Badge variant={project.status === "active" ? "success" : "secondary"} className="shrink-0">
          {project.status}
        </Badge>
      </div>

      {project.description && (
        <div className="line-clamp-2 text-xs text-muted-foreground/70">
          {project.description}
        </div>
      )}

      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Folder className="h-3 w-3 shrink-0" />
        <span className="truncate">{project.work_dir}</span>
      </div>

      <div className="flex items-center gap-1.5">
        {activeSessions > 0 && (
          <Badge variant="outline" className="text-[11px]">
            {activeSessions} active session{activeSessions !== 1 ? "s" : ""}
          </Badge>
        )}
        <Badge variant="outline" className="text-[11px]">
          max {project.max_sessions}
        </Badge>
      </div>
    </button>
  );
}
