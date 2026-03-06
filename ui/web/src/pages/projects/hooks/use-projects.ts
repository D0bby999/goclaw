import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWs, useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods } from "@/api/protocol";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { Project, ProjectSession, ProjectSessionLog } from "@/types/project";

export function useProjects() {
  const ws = useWs();
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const queryClient = useQueryClient();

  const { data: projects = [], isLoading: loading, error: queryError } = useQuery({
    queryKey: queryKeys.cc.projects,
    queryFn: async () => {
      // Try HTTP first (reliable, no WS dependency)
      try {
        const res = await http.get<{ projects: Project[]; count: number }>("/v1/cc/projects");
        if (res.projects) return res.projects;
      } catch {
        // HTTP may fail — fall through to WS
      }

      // Fallback: WS
      if (!ws.isConnected) return [];
      const res = await ws.call<{ projects: Project[]; count: number }>(
        Methods.PROJECTS_LIST,
      );
      return res.projects ?? [];
    },
    enabled: connected,
  });

  const error = queryError instanceof Error ? queryError.message : queryError ? "Failed to load projects" : null;

  const invalidateProjects = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.cc.projects }),
    [queryClient],
  );

  const createProject = useCallback(
    async (params: {
      name: string;
      slug: string;
      work_dir: string;
      description?: string;
      max_sessions?: number;
      allowed_tools?: string[];
      team_id?: string;
      claude_config?: Record<string, unknown>;
    }) => {
      try {
        await ws.call(Methods.PROJECTS_CREATE, params);
        await invalidateProjects();
        toast.success("Project created", `${params.name} has been added`);
      } catch (err) {
        toast.error("Failed to create project", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidateProjects],
  );

  const getProject = useCallback(
    async (projectId: string) => {
      const res = await ws.call<{ project: Project }>(
        Methods.PROJECTS_GET,
        { id: projectId },
      );
      return res.project;
    },
    [ws],
  );

  const updateProject = useCallback(
    async (
      projectId: string,
      params: Partial<Pick<Project, "name" | "slug" | "description" | "work_dir" | "max_sessions" | "allowed_tools" | "status" | "team_id" | "claude_config">>,
    ) => {
      try {
        await ws.call(Methods.PROJECTS_UPDATE, { id: projectId, updates: params });
        await invalidateProjects();
        toast.success("Project updated");
      } catch (err) {
        toast.error("Failed to update project", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidateProjects],
  );

  const deleteProject = useCallback(
    async (projectId: string) => {
      try {
        await ws.call(Methods.PROJECTS_DELETE, { id: projectId });
        await invalidateProjects();
        toast.success("Project deleted");
      } catch (err) {
        toast.error("Failed to delete project", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidateProjects],
  );

  const listSessions = useCallback(
    async (projectId: string) => {
      const res = await ws.call<{ sessions: ProjectSession[]; total: number }>(
        Methods.PROJECT_SESSIONS_LIST,
        { project_id: projectId },
      );
      return res.sessions ?? [];
    },
    [ws],
  );

  const startSession = useCallback(
    async (projectId: string, prompt: string, label?: string) => {
      const res = await ws.call<{ session: ProjectSession }>(
        Methods.PROJECT_SESSIONS_START,
        { project_id: projectId, prompt, label },
      );
      return res.session;
    },
    [ws],
  );

  const getSession = useCallback(
    async (sessionId: string) => {
      const res = await ws.call<{ session: ProjectSession }>(
        Methods.PROJECT_SESSIONS_GET,
        { id: sessionId },
      );
      return res.session;
    },
    [ws],
  );

  const sendPrompt = useCallback(
    async (sessionId: string, prompt: string): Promise<void> => {
      await ws.call(Methods.PROJECT_SESSIONS_PROMPT, { id: sessionId, prompt });
    },
    [ws],
  );

  const stopSession = useCallback(
    async (sessionId: string) => {
      await ws.call(Methods.PROJECT_SESSIONS_STOP, { id: sessionId });
    },
    [ws],
  );

  const deleteSession = useCallback(
    async (sessionId: string) => {
      try {
        await ws.call(Methods.PROJECT_SESSIONS_DELETE, { id: sessionId });
        toast.success("Session deleted");
      } catch (err) {
        toast.error("Failed to delete session", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws],
  );

  const updateSession = useCallback(
    async (sessionId: string, params: { label: string }) => {
      try {
        await ws.call(Methods.PROJECT_SESSIONS_UPDATE, { id: sessionId, label: params.label });
        toast.success("Session updated");
      } catch (err) {
        toast.error("Failed to update session", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws],
  );

  const getSessionLogs = useCallback(
    async (sessionId: string, limit?: number) => {
      const res = await ws.call<{ logs: ProjectSessionLog[] }>(
        Methods.PROJECT_SESSIONS_LOGS,
        { session_id: sessionId, limit: limit ?? 200 },
      );
      return res.logs ?? [];
    },
    [ws],
  );

  return {
    projects,
    loading,
    error,
    loadProjects: invalidateProjects,
    createProject,
    getProject,
    updateProject,
    deleteProject,
    listSessions,
    startSession,
    getSession,
    sendPrompt,
    stopSession,
    deleteSession,
    updateSession,
    getSessionLogs,
  };
}
