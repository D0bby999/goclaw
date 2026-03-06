import { Terminal, Folder, Users, Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Project } from "@/types/project";

interface ProjectCardProps {
  project: Project;
  activeSessions?: number;
  teamName?: string;
  onClick: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
}

export function ProjectCard({ project, activeSessions = 0, teamName, onClick, onEdit, onDelete }: ProjectCardProps) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") onClick(); }}
      className="group relative flex cursor-pointer flex-col gap-3 rounded-lg border bg-card p-4 text-left transition-all hover:border-primary/30 hover:shadow-md"
    >
      {/* Action buttons - top right, visible on hover */}
      {(onEdit || onDelete) && (
        <div className="absolute top-2 right-2 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {onEdit && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-foreground"
              onClick={(e) => { e.stopPropagation(); onEdit(); }}
              title="Edit project"
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
          )}
          {onDelete && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              onClick={(e) => { e.stopPropagation(); onDelete(); }}
              title="Delete project"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      )}

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
        <Badge variant={project.status === "active" ? "success" : "secondary"} className="shrink-0 mr-14 group-hover:mr-14">
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

      <div className="flex flex-wrap items-center gap-1.5">
        {teamName && (
          <Badge variant="outline" className="text-[11px] gap-1">
            <Users className="h-3 w-3" /> {teamName}
          </Badge>
        )}
        {activeSessions > 0 && (
          <Badge variant="outline" className="text-[11px]">
            {activeSessions} active session{activeSessions !== 1 ? "s" : ""}
          </Badge>
        )}
        <Badge variant="outline" className="text-[11px]">
          max {project.max_sessions}
        </Badge>
      </div>
    </div>
  );
}
