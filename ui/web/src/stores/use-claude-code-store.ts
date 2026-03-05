import { create } from "zustand";
import type { StreamEvent } from "@/types/claude-code";

interface ClaudeCodeState {
  sessionLogs: Record<string, StreamEvent[]>;
  sessionStatuses: Record<string, string>;

  appendLog: (sessionId: string, event: StreamEvent) => void;
  clearLogs: (sessionId: string) => void;
  updateStatus: (sessionId: string, status: string) => void;
}

export const useClaudeCodeStore = create<ClaudeCodeState>((set, get) => ({
  sessionLogs: {},
  sessionStatuses: {},

  appendLog: (sessionId, event) => {
    const current = get().sessionLogs[sessionId] ?? [];
    set({
      sessionLogs: {
        ...get().sessionLogs,
        [sessionId]: [...current, event],
      },
    });
  },

  clearLogs: (sessionId) => {
    const logs = { ...get().sessionLogs };
    delete logs[sessionId];
    set({ sessionLogs: logs });
  },

  updateStatus: (sessionId, status) => {
    set({
      sessionStatuses: {
        ...get().sessionStatuses,
        [sessionId]: status,
      },
    });
  },
}));
