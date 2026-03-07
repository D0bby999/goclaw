import { useState, useEffect } from "react";
import { useHttp } from "@/hooks/use-ws";
import { useDebounce } from "@/hooks/use-debounce";
import { PlatformIcon, PLATFORM_META } from "./platform-icons";
import { usePlatformLimits } from "./hooks/use-platform-limits";
import { cn } from "@/lib/utils";
import type { SocialPlatform, AdaptResult } from "@/types/social";

interface PlatformPreviewProps {
  content: string;
  platform: SocialPlatform;
}

export function PlatformPreview({ content, platform }: PlatformPreviewProps) {
  const http = useHttp();
  const { platforms } = usePlatformLimits();
  const limits = platforms[platform];
  const debouncedContent = useDebounce(content, 500);
  const [adapted, setAdapted] = useState<AdaptResult | null>(null);

  useEffect(() => {
    if (!debouncedContent.trim()) {
      setAdapted(null);
      return;
    }
    let cancelled = false;
    http.post<{ results: Record<string, AdaptResult> }>("/v1/social/adapt", {
      content: debouncedContent,
      platforms: [platform],
    }).then((res) => {
      if (!cancelled && res.results?.[platform]) {
        setAdapted(res.results[platform]);
      }
    }).catch(() => {
      // silently ignore preview errors
    });
    return () => { cancelled = true; };
  }, [debouncedContent, platform, http]);

  const meta = PLATFORM_META[platform];
  const displayContent = adapted?.adapted ?? content;
  const charCount = displayContent.length;
  const maxChars = limits?.max_chars ?? 0;
  const pct = maxChars > 0 ? charCount / maxChars : 0;
  const counterColor = pct > 1 ? "text-red-500" : pct > 0.9 ? "text-yellow-500" : "text-green-500";

  return (
    <div className="rounded-lg border">
      {/* Header */}
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <PlatformIcon platform={platform} className="h-4 w-4" />
        <span className="text-sm font-medium">{meta?.label}</span>
        {maxChars > 0 && (
          <span className={cn("ml-auto text-xs font-mono", counterColor)}>
            {charCount}/{maxChars}
          </span>
        )}
      </div>

      {/* Content */}
      <div className="p-3">
        {!content.trim() ? (
          <p className="text-sm text-muted-foreground italic">Start typing to see preview...</p>
        ) : (
          <p className="whitespace-pre-wrap text-sm leading-relaxed">{displayContent}</p>
        )}
      </div>

      {/* Warnings */}
      {adapted?.warnings && adapted.warnings.length > 0 && (
        <div className="border-t px-3 py-2 space-y-1">
          {adapted.warnings.map((w, i) => (
            <p key={i} className="text-xs text-yellow-600 dark:text-yellow-400">{w}</p>
          ))}
        </div>
      )}
    </div>
  );
}
