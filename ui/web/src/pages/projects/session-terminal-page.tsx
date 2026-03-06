import { useState, useEffect, useCallback, useRef } from "react";
import { useNavigate } from "react-router";
import { ArrowLeft, Loader2, Trash2, Pencil, Check, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAutoScroll } from "@/hooks/use-auto-scroll";
import { useWsEvent } from "@/hooks/use-ws-event";
import { Events } from "@/api/protocol";
import { useProjects } from "./hooks/use-projects";
import { useProjectStore } from "@/stores/use-project-store";
import { AgentStatusIndicator, deriveAgentStatus } from "./agent-status-indicator";
import { EventBlock } from "./terminal-event-block";
import { TerminalInputBar } from "./terminal-input-bar";
import type { ProjectSession, StreamEvent } from "@/types/project";

interface SessionTerminalPageProps {
  sessionId: string;
}

export function SessionTerminalPage({ sessionId }: SessionTerminalPageProps) {
  const navigate = useNavigate();
  const { getSession, sendPrompt, stopSession, deleteSession, updateSession, getSessionLogs } = useProjects();
  const { sessionLogs, appendLog, clearLogs, updateStatus, sessionStatuses } = useProjectStore();
  const [session, setSession] = useState<ProjectSession | null>(null);
  const [prompt, setPrompt] = useState("");
  const [sending, setSending] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editLabel, setEditLabel] = useState("");
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const lastEventTimeRef = useRef(Date.now());
  const logs = sessionLogs[sessionId] ?? [];
  const liveStatus = sessionStatuses[sessionId];
  const { ref: scrollRef, onScroll } = useAutoScroll<HTMLDivElement>([logs]);

  // Load session info
  useEffect(() => {
    let cancelled = false;
    getSession(sessionId).then((s) => { if (!cancelled) setSession(s); }).catch(() => {});
    return () => { cancelled = true; };
  }, [sessionId, getSession]);

  // Load historical logs once
  useEffect(() => {
    clearLogs(sessionId);
    getSessionLogs(sessionId).then((dbLogs) => {
      dbLogs.forEach((l) => {
        appendLog(sessionId, { type: l.event_type, raw: l.content, session_id: sessionId });
      });
    }).catch(() => {});
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  // Subscribe to live project.output events
  const handleOutput = useCallback((payload: unknown) => {
    const p = payload as { session_id?: string; event?: Partial<StreamEvent>; [k: string]: unknown };
    if (String(p.session_id ?? "") !== sessionId) return;
    lastEventTimeRef.current = Date.now();
    const evt = (p.event ?? p) as Partial<StreamEvent>;
    appendLog(sessionId, {
      type: evt.type ?? "unknown", subtype: evt.subtype, raw: evt.raw ?? {},
      session_id: sessionId, input_tokens: evt.input_tokens,
      output_tokens: evt.output_tokens, cost_usd: evt.cost_usd,
    });
    // Refresh session data on result (picks up claude_session_id, updated tokens)
    if (evt.type === "result") {
      getSession(sessionId).then(setSession).catch(() => {});
    }
  }, [sessionId, appendLog, getSession]);

  const handleStatusChange = useCallback((payload: unknown) => {
    const p = payload as { session_id?: string; status?: string };
    if (p.session_id !== sessionId) return;
    updateStatus(sessionId, p.status ?? "");
    if (session && p.status) setSession((s) => s ? { ...s, status: p.status as ProjectSession["status"] } : s);
  }, [sessionId, updateStatus, session]);

  useWsEvent(Events.PROJECT_OUTPUT, handleOutput);
  useWsEvent(Events.PROJECT_SESSION_STATUS, handleStatusChange);

  const currentStatus = (liveStatus ?? session?.status ?? "stopped") as string;
  const isRunning = currentStatus === "running" || currentStatus === "starting";
  const canResume = session?.claude_session_id != null;
  const canSendPrompt = canResume;
  // Show "idle" instead of "completed" when session can accept more input
  const rawAgentStatus = deriveAgentStatus(currentStatus, logs[logs.length - 1], lastEventTimeRef.current);
  const agentStatus = (rawAgentStatus === "completed" && canResume) ? "idle" as const : rawAgentStatus;

  const latestTokens = logs.reduce((acc, e) => ({
    input: e.input_tokens ?? acc.input,
    output: e.output_tokens ?? acc.output,
  }), { input: session?.input_tokens ?? 0, output: session?.output_tokens ?? 0 });
  const totalCost = logs.reduce((sum, e) => e.cost_usd ? e.cost_usd : sum, session?.cost_usd ?? 0);

  const handleStartRename = () => {
    setEditLabel(session?.label ?? sessionId.slice(0, 8));
    setEditing(true);
  };

  const handleSaveRename = async () => {
    const newLabel = editLabel.trim();
    if (!newLabel) { setEditing(false); return; }
    try {
      await updateSession(sessionId, { label: newLabel });
      setSession((s) => s ? { ...s, label: newLabel } : s);
    } catch { /* toast already shown */ }
    setEditing(false);
  };

  const handleDelete = async () => {
    try {
      await deleteSession(sessionId);
      navigate(-1);
    } catch { /* toast already shown */ }
    setShowDeleteConfirm(false);
  };

  const handleSend = async () => {
    const text = prompt.trim();
    if (!text || sending) return;
    setSending(true);
    setPrompt("");
    // Show user message in terminal immediately
    appendLog(sessionId, { type: "user", raw: { text }, session_id: sessionId });
    // Optimistic: mark as running so UI shows activity
    updateStatus(sessionId, "running");
    try {
      await sendPrompt(sessionId, text);
    } catch { setPrompt(text); } finally { setSending(false); }
  };

  const handleStop = async () => {
    try { await stopSession(sessionId); } catch { /* ignore */ }
  };

  return (
    <div className="flex flex-col h-full bg-[#0d1117] text-slate-100">
      {/* Status bar header */}
      <div className="border-b border-slate-800 px-4 py-2 shrink-0">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => navigate(-1)} className="text-slate-400 hover:text-slate-100">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="flex items-center gap-2 min-w-0 flex-1 text-sm">
            <span className="text-slate-400">Project:</span>
            <span className="font-medium truncate">{session?.project_name ?? session?.project_id ?? "Project"}</span>
            <span className="text-slate-600">|</span>
            <span className="text-slate-400">Session:</span>
            {editing ? (
              <div className="flex items-center gap-1">
                <input
                  autoFocus
                  className="bg-slate-800 border border-slate-600 rounded px-1.5 py-0.5 text-sm text-slate-100 w-48 outline-none focus:border-blue-500"
                  value={editLabel}
                  onChange={(e) => setEditLabel(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") handleSaveRename(); if (e.key === "Escape") setEditing(false); }}
                  onBlur={handleSaveRename}
                />
                <Button variant="ghost" size="icon" className="h-5 w-5 text-emerald-400 hover:text-emerald-300" onClick={handleSaveRename}>
                  <Check className="h-3 w-3" />
                </Button>
                <Button variant="ghost" size="icon" className="h-5 w-5 text-slate-400 hover:text-slate-300" onClick={() => setEditing(false)}>
                  <X className="h-3 w-3" />
                </Button>
              </div>
            ) : (
              <span className="font-medium truncate cursor-pointer hover:text-blue-400 transition-colors" onClick={handleStartRename} title="Click to rename">
                {session?.label ?? sessionId.slice(0, 8)}
              </span>
            )}
            <Button variant="ghost" size="icon" className="h-5 w-5 text-slate-500 hover:text-slate-300" onClick={handleStartRename} title="Rename session">
              <Pencil className="h-3 w-3" />
            </Button>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <AgentStatusIndicator status={agentStatus} />
            <Badge
              variant={isRunning ? "success" : currentStatus === "failed" ? "destructive" : "secondary"}
              className="text-[11px]"
            >
              {currentStatus}
            </Badge>
            <span className="text-xs text-slate-500 font-mono">
              {latestTokens.input.toLocaleString()}↑ {latestTokens.output.toLocaleString()}↓ · ${totalCost.toFixed(4)}
            </span>
            <div className="relative">
              {showDeleteConfirm ? (
                <div className="flex items-center gap-1 bg-slate-800 border border-red-800 rounded px-2 py-1">
                  <span className="text-xs text-red-400">Delete?</span>
                  <Button variant="ghost" size="icon" className="h-5 w-5 text-red-400 hover:text-red-300" onClick={handleDelete}>
                    <Check className="h-3 w-3" />
                  </Button>
                  <Button variant="ghost" size="icon" className="h-5 w-5 text-slate-400 hover:text-slate-300" onClick={() => setShowDeleteConfirm(false)}>
                    <X className="h-3 w-3" />
                  </Button>
                </div>
              ) : (
                <Button variant="ghost" size="icon" className="h-6 w-6 text-slate-500 hover:text-red-400" onClick={() => setShowDeleteConfirm(true)} title="Delete session">
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Terminal output */}
      <div ref={scrollRef} onScroll={onScroll} className="flex-1 overflow-y-auto px-4 py-3 space-y-2 font-mono text-sm">
        {logs.length === 0 && <div className="text-slate-600 text-sm mt-4">Waiting for output...</div>}
        {logs.map((event, i) => <EventBlock key={`${event.type}-${i}`} event={event} />)}
        {/* Live thinking indicator */}
        {isRunning && (agentStatus === "thinking" || agentStatus === "writing") && (
          <div className="flex items-center gap-2 text-xs text-slate-500 animate-pulse py-1">
            <Loader2 className="h-3 w-3 animate-spin" />
            <span>{agentStatus === "writing" ? "Writing..." : "Thinking..."}</span>
          </div>
        )}
      </div>

      {/* Input bar */}
      <TerminalInputBar
        prompt={prompt}
        onPromptChange={setPrompt}
        onSend={handleSend}
        onStop={handleStop}
        isRunning={isRunning}
        canSend={canSendPrompt}
        sending={sending}
      />
    </div>
  );
}
