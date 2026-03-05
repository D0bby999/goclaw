import { useState } from "react";
import {
  FileText, Terminal, Pencil, FilePlus, FolderSearch,
  Search, Globe, Wrench, ChevronRight, CheckCircle2,
  XCircle, AlertCircle, Users, User,
} from "lucide-react";
import { MarkdownRenderer } from "@/components/shared/markdown-renderer";
import { cn } from "@/lib/utils";
import type { StreamEvent } from "@/types/claude-code";

// --- Tool icon + color mapping ---

const TOOL_META: Record<string, { icon: React.ElementType; color: string }> = {
  Read:      { icon: FileText,     color: "text-blue-400" },
  Glob:      { icon: FolderSearch, color: "text-blue-400" },
  Grep:      { icon: Search,       color: "text-cyan-400" },
  Edit:      { icon: Pencil,       color: "text-amber-400" },
  Write:     { icon: FilePlus,     color: "text-emerald-400" },
  Bash:      { icon: Terminal,     color: "text-purple-400" },
  WebSearch: { icon: Globe,        color: "text-teal-400" },
  WebFetch:  { icon: Globe,        color: "text-teal-400" },
  Agent:     { icon: Users,        color: "text-indigo-400" },
};

function shortenPath(path?: string): string {
  if (!path) return "file";
  const parts = path.split("/");
  return parts.length > 2 ? `.../${parts.slice(-2).join("/")}` : path;
}

function getToolSummary(name: string, input: unknown): string {
  const inp = input as Record<string, unknown> | undefined;
  switch (name) {
    case "Read":  return `Read ${shortenPath(inp?.file_path as string)}`;
    case "Write": return `Wrote ${shortenPath(inp?.file_path as string)}`;
    case "Edit":  return `Edited ${shortenPath(inp?.file_path as string)}`;
    case "Glob":  return `Searched for ${inp?.pattern ?? "files"}`;
    case "Grep":  return `Searched "${inp?.pattern ?? "..."}"`;
    case "Bash": {
      const cmd = String(inp?.command ?? "").slice(0, 60);
      return `Ran: ${cmd}${String(inp?.command ?? "").length > 60 ? "..." : ""}`;
    }
    case "WebSearch": return `Searched: ${inp?.query ?? "..."}`;
    case "WebFetch":  return `Fetched: ${(() => { try { return new URL(String(inp?.url)).hostname; } catch { return String(inp?.url ?? "").slice(0, 40); } })()}`;
    case "Agent":     return `Spawned ${inp?.description ?? "agent"}`;
    default:          return name;
  }
}

// --- Sub-components ---

function AssistantTextBlock({ text }: { text: string }) {
  return (
    <div className="text-slate-200 text-sm leading-relaxed">
      <MarkdownRenderer
        content={text}
        className="prose-invert prose-slate [&_*]:text-slate-200 [&_code]:bg-slate-800 [&_pre]:bg-slate-900 [&_a]:text-blue-400"
      />
    </div>
  );
}

function ToolCallBlock({ name, input }: { name: string; input: unknown }) {
  const [expanded, setExpanded] = useState(false);
  const meta = TOOL_META[name] ?? { icon: Wrench, color: "text-slate-400" };
  const Icon = meta.icon;
  const summary = getToolSummary(name, input);
  const inputStr = typeof input === "object" ? JSON.stringify(input, null, 2) : String(input ?? "");

  return (
    <div className="my-1">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className={cn(
          "flex items-center gap-2 text-xs font-mono hover:opacity-80 transition-opacity w-full text-left py-0.5",
          meta.color,
        )}
      >
        <ChevronRight className={cn("h-3 w-3 shrink-0 transition-transform duration-150", expanded && "rotate-90")} />
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span className="truncate">{summary}</span>
      </button>
      {expanded && inputStr && (
        <pre className="ml-[22px] mt-1 text-xs text-slate-500 font-mono whitespace-pre-wrap break-all max-h-60 overflow-y-auto bg-slate-900/50 rounded px-2 py-1">
          {inputStr.slice(0, 2000)}{inputStr.length > 2000 ? "\n..." : ""}
        </pre>
      )}
    </div>
  );
}

function ToolResultBlock({ content, isError }: { content: unknown; isError: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const text = typeof content === "string" ? content : JSON.stringify(content, null, 2);
  const preview = text?.slice(0, 80) ?? "";
  const hasMore = (text?.length ?? 0) > 80;

  return (
    <div className="my-0.5">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className={cn(
          "flex items-center gap-2 text-xs font-mono hover:opacity-80 transition-opacity w-full text-left py-0.5",
          isError ? "text-red-400" : "text-slate-500",
        )}
      >
        <ChevronRight className={cn("h-3 w-3 shrink-0 transition-transform duration-150", expanded && "rotate-90")} />
        {isError ? <XCircle className="h-3 w-3 shrink-0" /> : <CheckCircle2 className="h-3 w-3 shrink-0" />}
        <span className="truncate">{isError ? "Error" : "Result"}: {preview}{hasMore ? "..." : ""}</span>
      </button>
      {expanded && text && (
        <pre className={cn(
          "ml-[22px] mt-1 text-xs font-mono whitespace-pre-wrap break-all max-h-60 overflow-y-auto rounded px-2 py-1",
          isError ? "bg-red-950/30 text-red-300" : "bg-slate-900/50 text-slate-400",
        )}>
          {text.slice(0, 5000)}{text.length > 5000 ? "\n..." : ""}
        </pre>
      )}
    </div>
  );
}

function ResultBlock({ isError, costUsd, inputTokens, outputTokens }: {
  isError: boolean; costUsd?: number; inputTokens?: number; outputTokens?: number;
}) {
  const cost = costUsd ? `$${costUsd.toFixed(4)}` : "";
  const tokens = (inputTokens || outputTokens)
    ? `${(inputTokens ?? 0).toLocaleString()}↑ ${(outputTokens ?? 0).toLocaleString()}↓`
    : "";

  return (
    <div className={cn(
      "flex items-center gap-2 text-xs font-mono py-1 border-t border-slate-800/50 mt-2",
      isError ? "text-red-400" : "text-emerald-500",
    )}>
      {isError ? <XCircle className="h-3.5 w-3.5" /> : <CheckCircle2 className="h-3.5 w-3.5" />}
      <span className="font-semibold">{isError ? "Failed" : "Completed"}</span>
      {tokens && <span className="text-slate-500">· {tokens}</span>}
      {cost && <span className="text-slate-500">· {cost}</span>}
    </div>
  );
}

function SystemBlock({ subtype }: { subtype?: string }) {
  return (
    <div className="flex items-center gap-1.5 text-xs text-slate-600 font-mono py-0.5">
      <AlertCircle className="h-3 w-3" />
      <span>{subtype ? `system:${subtype}` : "system"}</span>
    </div>
  );
}

// --- User message (local) ---

function UserMessageBlock({ text }: { text: string }) {
  return (
    <div className="flex items-start gap-2 py-2 mt-2 border-t border-slate-800/50">
      <div className="shrink-0 w-6 h-6 rounded-full bg-blue-600/20 flex items-center justify-center">
        <User className="h-3.5 w-3.5 text-blue-400" />
      </div>
      <div className="text-sm text-slate-100 whitespace-pre-wrap leading-relaxed">{text}</div>
    </div>
  );
}

// --- Main dispatcher ---

export function EventBlock({ event }: { event: StreamEvent }) {
  const { type, raw } = event;

  if (type === "user") {
    return <UserMessageBlock text={(raw?.text as string) ?? ""} />;
  }

  if (type === "assistant") {
    const content = (raw?.message as { content?: Array<{ type: string; text?: string; name?: string; input?: unknown }> })?.content ?? [];
    return (
      <div className="space-y-1">
        {content.map((block, i) => {
          if (block.type === "text" && block.text) {
            return <AssistantTextBlock key={i} text={block.text} />;
          }
          if (block.type === "tool_use") {
            return <ToolCallBlock key={i} name={block.name ?? "Tool"} input={block.input} />;
          }
          return null;
        })}
      </div>
    );
  }

  if (type === "tool_result") {
    return <ToolResultBlock content={raw?.content} isError={raw?.is_error === true} />;
  }

  if (type === "result") {
    return (
      <ResultBlock
        isError={raw?.is_error === true}
        costUsd={event.cost_usd}
        inputTokens={event.input_tokens}
        outputTokens={event.output_tokens}
      />
    );
  }

  if (type === "system") {
    return <SystemBlock subtype={raw?.subtype as string | undefined} />;
  }

  return null;
}
