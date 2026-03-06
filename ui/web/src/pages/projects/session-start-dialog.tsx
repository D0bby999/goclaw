import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

interface SessionStartDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onStart: (prompt: string, label?: string) => Promise<void>;
}

export function SessionStartDialog({ open, onOpenChange, onStart }: SessionStartDialogProps) {
  const [prompt, setPrompt] = useState("");
  const [label, setLabel] = useState("");
  const [loading, setLoading] = useState(false);

  const reset = () => { setPrompt(""); setLabel(""); };

  const handleStart = async () => {
    if (!prompt.trim()) return;
    setLoading(true);
    try {
      await onStart(prompt.trim(), label.trim() || undefined);
      reset();
      onOpenChange(false);
    } catch {
      // error handled upstream
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) handleStart();
  };

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) reset(); onOpenChange(v); }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>New Session</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="ss-label">Label (optional)</Label>
            <Input
              id="ss-label"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="e.g. fix-auth-bug"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="ss-prompt">Initial Prompt *</Label>
            <Textarea
              id="ss-prompt"
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Describe what you want to do..."
              rows={5}
              className="resize-none font-mono text-sm"
            />
            <p className="text-xs text-muted-foreground">Ctrl+Enter to submit</p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleStart} disabled={!prompt.trim() || loading}>
            {loading ? "Starting..." : "Start Session"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
