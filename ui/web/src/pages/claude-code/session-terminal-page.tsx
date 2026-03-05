import { useState, useEffect, useCallback, useRef } from "react";
import { useNavigate } from "react-router";
import { ArrowLeft, Send, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { useAutoScroll } from "@/hooks/use-auto-scroll";
import { useWsEvent } from "@/hooks/use-ws-event";
import { Events } from "@/api/protocol";
import { useClaudeCode } from "./hooks/use-claude-code";
import { useClaudeCodeStore } from "@/stores/use-claude-code-store";
import { AgentStatusIndicator, deriveAgentStatus } from "./agent-status-indicator";
import type { CCSession, StreamEvent } from "@/types/claude-code";
import { cn } from "@/lib/utils";

interface SessionTerminalPageProps {
  sessionId: string;
}

function EventBlock({ event }: { event: StreamEvent }) {
  const [expanded, setExpanded] = useState(true);
  const { type, raw } = event;

  if (type === "assistant") {
    const content = (raw?.message as { content?: Array<{ type: string; text?: string; name?: string; input?: unknown }> })?.content ?? [];
    return (
      <div className="space-y-1">
        {content.map((block, i) => {
          if (block.type === "text") {
            return (
              <div key={i} className="text-emerald-400 whitespace-pre-wrap text-sm leading-relaxed">
                {block.text}
              </div>
            );
          }
          if (block.type === "tool_use") {
            return (
              <ToolBlock
                key={i}
                name={block.name ?? "Tool"}
                input={block.input}
                expanded={expanded}
                onToggle={() => setExpanded((v) => !v)}
                color="blue"
              />
            );
          }
          return null;
        })}
      </div>
    );
  }

  if (type === "tool_result") {
    const isError = raw?.is_error === true;
    const content = raw?.content;
    const text = typeof content === "string" ? content : JSON.stringify(content, null, 2);
    return (
      <div className={cn("text-xs font-mono pl-4 border-l-2", isError ? "border-red-500 text-red-400" : "border-slate-600 text-slate-400")}>
        <button type="button" onClick={() => setExpanded((v) => !v)} className="text-left w-full hover:opacity-80">
          {isError ? "[Error] " : "[Result] "}{expanded ? "▾" : "▸"} {text?.slice(0, 80)}{(text?.length ?? 0) > 80 ? "..." : ""}
        </button>
        {expanded && text && text.length > 80 && (
          <pre className="mt-1 whitespace-pre-wrap break-all">{text}</pre>
        )}
      </div>
    );
  }

  if (type === "result") {
    const isError = raw?.is_error === true;
    const cost = event.cost_usd ? ` · $${event.cost_usd.toFixed(4)}` : "";
    return (
      <div className={cn("text-xs font-semibold", isError ? "text-red-400" : "text-green-500")}>
        {isError ? "✗ Failed" : "✓ Completed"}{cost}
      </div>
    );
  }

  if (type === "system") {
    return (
      <div className="text-xs text-slate-500 italic">
        {typeof raw?.subtype === "string" ? `[system:${raw.subtype}]` : "[system]"}
      </div>
    );
  }

  return null;
}

function ToolBlock({
  name, input, expanded, onToggle, color,
}: {
  name: string;
  input: unknown;
  expanded: boolean;
  onToggle: () => void;
  color: string;
}) {
  const colorClass = color === "blue" ? "text-blue-400 border-blue-700" : "text-amber-400 border-amber-700";
  const inputStr = typeof input === "object" ? JSON.stringify(input, null, 2) : String(input ?? "");
  return (
    <div className={cn("pl-3 border-l-2 text-xs font-mono", colorClass)}>
      <button type="button" onClick={onToggle} className="flex items-center gap-1 hover:opacity-80 text-left w-full">
        <span>[Tool: {name}]</span>
        <span>{expanded ? "▾" : "▸"}</span>
      </button>
      {expanded && inputStr && (
        <pre className="mt-0.5 whitespace-pre-wrap break-all text-slate-400">{inputStr.slice(0, 500)}{inputStr.length > 500 ? "\n..." : ""}</pre>
      )}
    </div>
  );
}

export function SessionTerminalPage({ sessionId }: SessionTerminalPageProps) {
  const navigate = useNavigate();
  const { getSession, sendPrompt, stopSession, getSessionLogs } = useClaudeCode();
  const { sessionLogs, appendLog, clearLogs, updateStatus, sessionStatuses } = useClaudeCodeStore();
  const [session, setSession] = useState<CCSession | null>(null);
  const [prompt, setPrompt] = useState("");
  const [sending, setSending] = useState(false);
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
        appendLog(sessionId, {
          type: l.event_type,
          raw: l.content,
          session_id: sessionId,
        });
      });
    }).catch(() => {});
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  // Subscribe to live cc.output events
  const handleOutput = useCallback((payload: unknown) => {
    const p = payload as { session_id?: string; type?: string; raw?: Record<string, unknown>; input_tokens?: number; output_tokens?: number; cost_usd?: number };
    if (p.session_id !== sessionId) return;
    lastEventTimeRef.current = Date.now();
    appendLog(sessionId, {
      type: p.type ?? "unknown",
      raw: p.raw ?? {},
      session_id: sessionId,
      input_tokens: p.input_tokens,
      output_tokens: p.output_tokens,
      cost_usd: p.cost_usd,
    });
  }, [sessionId, appendLog]);

  const handleStatusChange = useCallback((payload: unknown) => {
    const p = payload as { session_id?: string; status?: string };
    if (p.session_id !== sessionId) return;
    updateStatus(sessionId, p.status ?? "");
    if (session && p.status) setSession((s) => s ? { ...s, status: p.status as CCSession["status"] } : s);
  }, [sessionId, updateStatus, session]);

  useWsEvent(Events.CC_OUTPUT, handleOutput);
  useWsEvent(Events.CC_SESSION_STATUS, handleStatusChange);

  const currentStatus = (liveStatus ?? session?.status ?? "stopped") as string;
  const isRunning = currentStatus === "running" || currentStatus === "starting";
  const latestLog = logs[logs.length - 1];
  const agentStatus = deriveAgentStatus(currentStatus, latestLog, lastEventTimeRef.current);

  const latestTokens = logs.reduce((acc, e) => ({
    input: e.input_tokens ?? acc.input,
    output: e.output_tokens ?? acc.output,
  }), { input: session?.input_tokens ?? 0, output: session?.output_tokens ?? 0 });

  const handleSend = async () => {
    if (!prompt.trim() || sending) return;
    setSending(true);
    try {
      await sendPrompt(sessionId, prompt.trim());
      setPrompt("");
    } catch {
      // ignore
    } finally {
      setSending(false);
    }
  };

  const handleStop = async () => {
    try { await stopSession(sessionId); } catch { /* ignore */ }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  const projectName = session?.project_name ?? session?.project_id ?? "Project";
  const sessionLabel = session?.label ?? sessionId.slice(0, 8);

  return (
    <div className="flex flex-col h-full bg-[#0d1117] text-slate-100">
      {/* Header */}
      <div className="flex items-center gap-3 border-b border-slate-800 px-4 py-2 shrink-0">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => navigate(-1)}
          className="text-slate-400 hover:text-slate-100"
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <span className="text-sm text-slate-400 shrink-0">Project:</span>
          <span className="text-sm font-medium truncate">{projectName}</span>
          <span className="text-slate-600">|</span>
          <span className="text-sm text-slate-400 shrink-0">Session:</span>
          <span className="text-sm font-medium truncate">{sessionLabel}</span>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <AgentStatusIndicator status={agentStatus} />
          <Badge
            variant={isRunning ? "success" : currentStatus === "failed" ? "destructive" : "secondary"}
            className="text-[11px]"
          >
            {currentStatus}
          </Badge>
          <span className="text-xs text-slate-500">
            {latestTokens.input.toLocaleString()}↑ / {latestTokens.output.toLocaleString()}↓ tokens
          </span>
        </div>
      </div>

      {/* Terminal output */}
      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="flex-1 overflow-y-auto px-4 py-3 space-y-2 font-mono text-sm"
      >
        {logs.length === 0 && (
          <div className="text-slate-600 text-sm mt-4">Waiting for output...</div>
        )}
        {logs.map((event, i) => (
          <EventBlock key={i} event={event} />
        ))}
      </div>

      {/* Input bar */}
      <div className="border-t border-slate-800 px-4 py-3 flex items-center gap-2 shrink-0">
        <Input
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Send a follow-up prompt..."
          disabled={sending || !isRunning}
          className="flex-1 bg-slate-900 border-slate-700 text-slate-100 placeholder:text-slate-600 font-mono text-sm"
        />
        <Button
          onClick={handleSend}
          disabled={!prompt.trim() || sending || !isRunning}
          size="icon"
          className="shrink-0"
        >
          <Send className="h-4 w-4" />
        </Button>
        <Button
          onClick={handleStop}
          disabled={!isRunning}
          variant="destructive"
          size="icon"
          className="shrink-0"
        >
          <Square className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
