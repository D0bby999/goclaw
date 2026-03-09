import { useState } from "react";
import { CalendarDays, Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/shared/empty-state";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { formatDate } from "@/lib/format";
import { ScheduleFormDialog } from "./schedule-form-dialog";
import { useContentSchedules } from "./hooks/use-content-schedules";
import type { ContentSchedule } from "@/types/social";

export function SchedulesTab() {
  const { schedules, loading, createSchedule, updateSchedule, deleteSchedule, toggleSchedule } = useContentSchedules();
  const [formOpen, setFormOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ContentSchedule | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ContentSchedule | null>(null);
  const showSkeleton = useDeferredLoading(loading && schedules.length === 0);

  const handleEdit = (s: ContentSchedule) => {
    setEditTarget(s);
    setFormOpen(true);
  };

  const handleFormClose = (open: boolean) => {
    setFormOpen(open);
    if (!open) setEditTarget(null);
  };

  if (showSkeleton) return <TableSkeleton rows={4} />;

  if (schedules.length === 0) {
    return (
      <>
        <EmptyState
          icon={CalendarDays}
          title="No schedules yet"
          description="Create a schedule to automatically generate and publish content."
          action={
            <Button size="sm" onClick={() => setFormOpen(true)} className="gap-1">
              <Plus className="h-3.5 w-3.5" /> New Schedule
            </Button>
          }
        />
        <ScheduleFormDialog
          open={formOpen}
          onOpenChange={handleFormClose}
          editSchedule={null}
          onSubmit={createSchedule}
        />
      </>
    );
  }

  return (
    <>
      <div className="mb-4 flex justify-end">
        <Button size="sm" onClick={() => setFormOpen(true)} className="gap-1">
          <Plus className="h-3.5 w-3.5" /> New Schedule
        </Button>
      </div>

      <div className="rounded-md border overflow-x-auto">
        <table className="w-full min-w-[700px] text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-4 py-3 text-left font-medium">Enabled</th>
              <th className="px-4 py-3 text-left font-medium">Name</th>
              <th className="px-4 py-3 text-left font-medium">Schedule</th>
              <th className="px-4 py-3 text-left font-medium">Pages</th>
              <th className="px-4 py-3 text-left font-medium">Last Run</th>
              <th className="px-4 py-3 text-left font-medium">Posts</th>
              <th className="px-4 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {schedules.map((s) => (
              <ScheduleRow
                key={s.id}
                schedule={s}
                onToggle={(enabled) => toggleSchedule(s.id, enabled)}
                onEdit={() => handleEdit(s)}
                onDelete={() => setDeleteTarget(s)}
              />
            ))}
          </tbody>
        </table>
      </div>

      <ScheduleFormDialog
        open={formOpen}
        onOpenChange={handleFormClose}
        editSchedule={editTarget}
        onSubmit={(data) =>
          editTarget ? updateSchedule(editTarget.id, data) : createSchedule(data)
        }
      />

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setDeleteTarget(null)}
          title="Delete Schedule"
          description={`Delete schedule "${deleteTarget.name}"? This cannot be undone.`}
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={async () => {
            await deleteSchedule(deleteTarget.id);
            setDeleteTarget(null);
          }}
        />
      )}
    </>
  );
}

interface ScheduleRowProps {
  schedule: ContentSchedule;
  onToggle: (enabled: boolean) => void;
  onEdit: () => void;
  onDelete: () => void;
}

function ScheduleRow({ schedule, onToggle, onEdit, onDelete }: ScheduleRowProps) {
  const pages = schedule.pages ?? [];

  return (
    <tr className="border-b last:border-0 hover:bg-muted/30">
      <td className="px-4 py-3">
        <Switch
          checked={schedule.enabled}
          onCheckedChange={onToggle}
          aria-label={`Toggle ${schedule.name}`}
        />
      </td>
      <td className="px-4 py-3 font-medium">{schedule.name}</td>
      <td className="px-4 py-3 font-mono text-xs text-muted-foreground whitespace-nowrap">
        {schedule.cron_expression}
      </td>
      <td className="px-4 py-3">
        {pages.length === 0 ? (
          <span className="text-muted-foreground">--</span>
        ) : (
          <div className="flex flex-wrap gap-1">
            {pages.slice(0, 3).map((p) => (
              <Badge key={p.id} variant="secondary" className="text-xs">
                {p.page_name || p.page_id}
              </Badge>
            ))}
            {pages.length > 3 && (
              <Badge variant="outline" className="text-xs">+{pages.length - 3}</Badge>
            )}
          </div>
        )}
      </td>
      <td className="px-4 py-3 text-muted-foreground whitespace-nowrap text-xs">
        {schedule.last_run_at ? formatDate(schedule.last_run_at) : "--"}
      </td>
      <td className="px-4 py-3 text-muted-foreground">{schedule.posts_count}</td>
      <td className="px-4 py-3">
        <div className="flex justify-end gap-1">
          <Button variant="ghost" size="icon" title="Edit" onClick={onEdit}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" title="Delete" onClick={onDelete}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </td>
    </tr>
  );
}
