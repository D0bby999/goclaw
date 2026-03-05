import { useState, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import type { CCProject, CCSession, CCSessionLog } from "@/types/claude-code";

export function useClaudeCode() {
  const ws = useWs();
  const [projects, setProjects] = useState<CCProject[]>([]);
  const [loading, setLoading] = useState(false);

  const loadProjects = useCallback(async () => {
    if (!ws.isConnected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ projects: CCProject[]; count: number }>(
        Methods.CC_PROJECTS_LIST,
      );
      setProjects(res.projects ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws]);

  const createProject = useCallback(
    async (params: {
      name: string;
      slug: string;
      work_dir: string;
      description?: string;
      max_sessions?: number;
      allowed_tools?: string[];
    }) => {
      await ws.call(Methods.CC_PROJECTS_CREATE, params);
      loadProjects();
    },
    [ws, loadProjects],
  );

  const getProject = useCallback(
    async (projectId: string) => {
      const res = await ws.call<{ project: CCProject }>(
        Methods.CC_PROJECTS_GET,
        { id: projectId },
      );
      return res.project;
    },
    [ws],
  );

  const updateProject = useCallback(
    async (
      projectId: string,
      params: Partial<Pick<CCProject, "name" | "description" | "work_dir" | "max_sessions" | "allowed_tools" | "status">>,
    ) => {
      await ws.call(Methods.CC_PROJECTS_UPDATE, { id: projectId, updates: params });
    },
    [ws],
  );

  const deleteProject = useCallback(
    async (projectId: string) => {
      await ws.call(Methods.CC_PROJECTS_DELETE, { id: projectId });
      loadProjects();
    },
    [ws, loadProjects],
  );

  const listSessions = useCallback(
    async (projectId: string) => {
      const res = await ws.call<{ sessions: CCSession[]; count: number }>(
        Methods.CC_SESSIONS_LIST,
        { project_id: projectId },
      );
      return res.sessions ?? [];
    },
    [ws],
  );

  const startSession = useCallback(
    async (projectId: string, prompt: string, label?: string) => {
      const res = await ws.call<{ session: CCSession }>(
        Methods.CC_SESSIONS_START,
        { project_id: projectId, prompt, label },
      );
      return res.session;
    },
    [ws],
  );

  const getSession = useCallback(
    async (sessionId: string) => {
      const res = await ws.call<{ session: CCSession }>(
        Methods.CC_SESSIONS_GET,
        { id: sessionId },
      );
      return res.session;
    },
    [ws],
  );

  const sendPrompt = useCallback(
    async (sessionId: string, prompt: string) => {
      await ws.call(Methods.CC_SESSIONS_PROMPT, { id: sessionId, prompt });
    },
    [ws],
  );

  const stopSession = useCallback(
    async (sessionId: string) => {
      await ws.call(Methods.CC_SESSIONS_STOP, { id: sessionId });
    },
    [ws],
  );

  const getSessionLogs = useCallback(
    async (sessionId: string, limit?: number) => {
      const res = await ws.call<{ logs: CCSessionLog[] }>(
        Methods.CC_SESSIONS_LOGS,
        { session_id: sessionId, limit: limit ?? 200 },
      );
      return res.logs ?? [];
    },
    [ws],
  );

  return {
    projects,
    loading,
    loadProjects,
    createProject,
    getProject,
    updateProject,
    deleteProject,
    listSessions,
    startSession,
    getSession,
    sendPrompt,
    stopSession,
    getSessionLogs,
  };
}
