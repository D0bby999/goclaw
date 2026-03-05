import { Eye, Code, Terminal, Search, CheckCircle, XCircle, PauseCircle, Pencil } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentStatus } from "@/types/claude-code";

interface AgentStatusIndicatorProps {
  status: AgentStatus;
  className?: string;
}

const STATUS_CONFIG: Record<AgentStatus, { label: string; icon: React.ElementType; color: string }> = {
  writing:     { label: "Writing",         icon: Pencil,      color: "text-emerald-500" },
  reading:     { label: "Reading",         icon: Eye,         color: "text-blue-500" },
  editing:     { label: "Editing",         icon: Code,        color: "text-amber-500" },
  running_cmd: { label: "Running command", icon: Terminal,    color: "text-purple-500" },
  searching:   { label: "Searching",       icon: Search,      color: "text-cyan-500" },
  thinking:    { label: "Thinking",        icon: Pencil,      color: "text-gray-400" },
  completed:   { label: "Completed",       icon: CheckCircle, color: "text-green-500" },
  failed:      { label: "Failed",          icon: XCircle,     color: "text-red-500" },
  idle:        { label: "Idle",            icon: PauseCircle, color: "text-gray-400" },
};

export function AgentStatusIndicator({ status, className }: AgentStatusIndicatorProps) {
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.idle;
  const Icon = cfg.icon;
  const isThinking = status === "thinking";

  return (
    <div className={cn("flex items-center gap-1.5 text-sm", className)}>
      <Icon
        className={cn(
          "h-4 w-4",
          cfg.color,
          isThinking && "animate-pulse",
        )}
      />
      <span className={cn("font-medium", cfg.color)}>{cfg.label}</span>
    </div>
  );
}

/** Derive agent status from the latest stream event */
export function deriveAgentStatus(
  sessionStatus: string,
  latestEvent: { type?: string; raw?: Record<string, unknown> } | undefined,
  lastEventMs: number,
): AgentStatus {
  if (sessionStatus === "stopped" || sessionStatus === "completed") return "completed";
  if (sessionStatus === "failed") return "failed";
  if (sessionStatus !== "running" && sessionStatus !== "starting") return "idle";

  if (!latestEvent) return "thinking";

  const { type, raw } = latestEvent;

  if (type === "result") {
    const isError = raw?.is_error === true;
    return isError ? "failed" : "completed";
  }

  if (type === "assistant") {
    const content = raw?.message as { content?: unknown[] } | undefined;
    const firstBlock = content?.content?.[0] as { type?: string } | undefined;
    if (firstBlock?.type === "tool_use") {
      const toolName = (firstBlock as { name?: string }).name ?? "";
      if (["Read", "Glob", "Grep"].includes(toolName)) return "reading";
      if (["Edit", "Write"].includes(toolName)) return "editing";
      if (toolName === "Bash") return "running_cmd";
      if (["WebSearch", "WebFetch"].includes(toolName)) return "searching";
      return "reading";
    }
    return "writing";
  }

  if (type === "tool_use") {
    const toolName = (raw?.name as string) ?? "";
    if (["Read", "Glob", "Grep"].includes(toolName)) return "reading";
    if (["Edit", "Write"].includes(toolName)) return "editing";
    if (toolName === "Bash") return "running_cmd";
    if (["WebSearch", "WebFetch"].includes(toolName)) return "searching";
    return "reading";
  }

  // No event for >5s while running → thinking
  if (Date.now() - lastEventMs > 5000) return "thinking";

  return "thinking";
}
