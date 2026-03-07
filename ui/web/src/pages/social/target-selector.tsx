import { PlatformIcon, PLATFORM_META } from "./platform-icons";
import type { SocialAccount, SocialPlatform } from "@/types/social";

interface TargetSelectorProps {
  accounts: SocialAccount[];
  selected: Set<string>;
  onToggle: (accountId: string) => void;
}

export function TargetSelector({ accounts, selected, onToggle }: TargetSelectorProps) {
  if (accounts.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        No accounts connected. Connect accounts in the Accounts tab first.
      </p>
    );
  }

  // Group by platform
  const grouped = new Map<SocialPlatform, SocialAccount[]>();
  for (const a of accounts) {
    if (a.status !== "active") continue;
    const list = grouped.get(a.platform) ?? [];
    list.push(a);
    grouped.set(a.platform, list);
  }

  return (
    <div className="space-y-3">
      <label className="text-sm font-medium">Publish to</label>
      {Array.from(grouped.entries()).map(([platform, accs]) => (
        <div key={platform} className="space-y-1">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <PlatformIcon platform={platform} className="h-3.5 w-3.5" />
            {PLATFORM_META[platform]?.label}
          </div>
          {accs.map((a) => (
            <label
              key={a.id}
              className="flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm hover:bg-muted/30 transition-colors"
            >
              <input
                type="checkbox"
                checked={selected.has(a.id)}
                onChange={() => onToggle(a.id)}
                className="h-4 w-4 rounded border-muted-foreground"
              />
              <span>{a.display_name || a.platform_username || a.platform_user_id}</span>
            </label>
          ))}
        </div>
      ))}
    </div>
  );
}
