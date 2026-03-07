import { Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/shared/status-badge";
import { PlatformIcon, PLATFORM_META } from "./platform-icons";
import { formatDate } from "@/lib/format";
import type { SocialAccount } from "@/types/social";

interface AccountCardProps {
  account: SocialAccount;
  onEdit: (a: SocialAccount) => void;
  onDelete: (a: SocialAccount) => void;
}

export function AccountCard({ account, onEdit, onDelete }: AccountCardProps) {
  const meta = PLATFORM_META[account.platform];
  const statusVariant = account.status === "active" ? "success"
    : account.status === "expired" ? "warning" : "error";

  return (
    <div className="group rounded-lg border p-4 hover:bg-muted/30 transition-colors">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
            <PlatformIcon platform={account.platform} className="h-5 w-5" />
          </div>
          <div>
            <div className="font-medium text-sm">
              {account.display_name || account.platform_username || account.platform_user_id}
            </div>
            <div className="text-xs text-muted-foreground">
              {meta?.label ?? account.platform}
              {account.platform_username && ` @${account.platform_username}`}
            </div>
          </div>
        </div>
        <StatusBadge status={statusVariant} label={account.status} />
      </div>

      <div className="mt-3 flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          Connected {formatDate(account.connected_at)}
        </span>
        <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <Button variant="ghost" size="icon" className="h-7 w-7" title="Edit" onClick={() => onEdit(account)}>
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" title="Disconnect" onClick={() => onDelete(account)}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
