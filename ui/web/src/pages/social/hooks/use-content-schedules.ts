import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { ContentSchedule, ContentScheduleLog } from "@/types/social";

export function useContentSchedules() {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const queryClient = useQueryClient();

  const { data: schedules = [], isLoading: loading } = useQuery({
    queryKey: queryKeys.social.schedules,
    queryFn: async () => {
      const res = await http.get<{ schedules: ContentSchedule[] }>("/v1/schedules");
      return res.schedules ?? [];
    },
    enabled: connected,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.social.schedules }),
    [queryClient],
  );

  const createSchedule = useCallback(
    async (data: Partial<ContentSchedule> & { page_ids?: string[] }) => {
      try {
        await http.post("/v1/schedules", data);
        await invalidate();
        toast.success("Schedule created");
      } catch (err) {
        toast.error("Failed to create schedule", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate],
  );

  const updateSchedule = useCallback(
    async (id: string, data: Partial<ContentSchedule> & { page_ids?: string[] }) => {
      try {
        await http.put(`/v1/schedules/${id}`, data);
        await invalidate();
        toast.success("Schedule updated");
      } catch (err) {
        toast.error("Failed to update schedule", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate],
  );

  const deleteSchedule = useCallback(
    async (id: string) => {
      try {
        await http.delete(`/v1/schedules/${id}`);
        await invalidate();
        toast.success("Schedule deleted");
      } catch (err) {
        toast.error("Failed to delete schedule", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate],
  );

  const toggleSchedule = useCallback(
    async (id: string, enabled: boolean) => {
      try {
        await http.post(`/v1/schedules/${id}/toggle`, { enabled });
        await invalidate();
      } catch (err) {
        toast.error("Failed to toggle schedule", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate],
  );

  return { schedules, loading, refresh: invalidate, createSchedule, updateSchedule, deleteSchedule, toggleSchedule };
}

export function useScheduleLogs(id: string) {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);

  const { data: logs = [], isLoading: loading } = useQuery({
    queryKey: queryKeys.social.scheduleLogs(id),
    queryFn: async () => {
      const res = await http.get<{ logs: ContentScheduleLog[] }>(`/v1/schedules/${id}/logs`);
      return res.logs ?? [];
    },
    enabled: connected && !!id,
  });

  return { logs, loading };
}
