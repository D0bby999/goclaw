import { useRef, useCallback, useEffect } from "react";
import { Send, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

interface TerminalInputBarProps {
  prompt: string;
  onPromptChange: (value: string) => void;
  onSend: () => void;
  onStop: () => void;
  isRunning: boolean;
  canSend: boolean;
  sending: boolean;
}

export function TerminalInputBar({
  prompt, onPromptChange, onSend, onStop,
  isRunning, canSend, sending,
}: TerminalInputBarProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-focus input when Claude finishes responding
  useEffect(() => {
    if (!isRunning && canSend && !sending) {
      textareaRef.current?.focus();
    }
  }, [isRunning, canSend, sending]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  }, [onSend]);

  const placeholder = isRunning
    ? "Claude is working... send a follow-up"
    : canSend
      ? "Send a message..."
      : "Waiting for session to initialize...";

  return (
    <div className="border-t border-slate-800 px-4 py-3 shrink-0">
      <div className="flex items-end gap-2">
        <Textarea
          ref={textareaRef}
          value={prompt}
          onChange={(e) => onPromptChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={sending || !canSend}
          rows={1}
          className={cn(
            "flex-1 min-h-[36px] max-h-32 resize-none",
            "bg-slate-900 border-slate-700 text-slate-100 placeholder:text-slate-600",
            "font-mono text-sm",
          )}
        />
        <Button
          onClick={onSend}
          disabled={!prompt.trim() || sending || !canSend}
          size="icon"
          className="shrink-0 h-9 w-9"
        >
          <Send className="h-4 w-4" />
        </Button>
        <Button
          onClick={onStop}
          disabled={!isRunning}
          variant="destructive"
          size="icon"
          className="shrink-0 h-9 w-9"
        >
          <Square className="h-4 w-4" />
        </Button>
      </div>
      <div className="flex items-center gap-3 mt-1.5 text-[10px] text-slate-600 font-mono">
        <span>↵ Send</span>
        <span>⇧↵ Newline</span>
      </div>
    </div>
  );
}
