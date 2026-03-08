import { RefreshCw, Star, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSocialPages } from "./hooks/use-social-pages";
import type { SocialPage } from "@/types/social";

interface AccountPagesPanelProps {
  accountId: string;
  platform: string;
}

const PAGE_SUPPORTED_PLATFORMS = ["facebook", "instagram"];

export function AccountPagesPanel({ accountId, platform }: AccountPagesPanelProps) {
  const { pages, loading, syncPages, setDefault, deletePage } = useSocialPages(accountId);

  if (!PAGE_SUPPORTED_PLATFORMS.includes(platform)) return null;

  return (
    <div className="mt-3 border-t pt-3 space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">
          Pages {pages.length > 0 && `(${pages.length})`}
        </span>
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          title="Sync pages"
          onClick={syncPages}
          disabled={loading}
        >
          <RefreshCw className={`h-3 w-3 ${loading ? "animate-spin" : ""}`} />
        </Button>
      </div>

      {pages.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground">No pages found. Click sync to fetch.</p>
      )}

      {pages.map((page) => (
        <PageRow
          key={page.id}
          page={page}
          onSetDefault={() => setDefault(page.id)}
          onDelete={() => deletePage(page.id)}
        />
      ))}
    </div>
  );
}

function PageRow({
  page,
  onSetDefault,
  onDelete,
}: {
  page: SocialPage;
  onSetDefault: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-center justify-between rounded-md border px-2.5 py-1.5 text-xs">
      <div className="flex items-center gap-2 min-w-0">
        {page.avatar_url ? (
          <img src={page.avatar_url} alt="" className="h-5 w-5 rounded-full shrink-0" />
        ) : (
          <div className="h-5 w-5 rounded-full bg-muted shrink-0" />
        )}
        <span className="truncate">{page.page_name || page.page_id}</span>
        <span className="shrink-0 rounded bg-muted px-1 text-[10px] text-muted-foreground">
          {page.page_type}
        </span>
        {page.is_default && (
          <Star className="h-3 w-3 shrink-0 fill-yellow-400 text-yellow-400" />
        )}
      </div>
      <div className="flex gap-0.5 shrink-0">
        {!page.is_default && (
          <Button variant="ghost" size="icon" className="h-6 w-6" title="Set as default" onClick={onSetDefault}>
            <Star className="h-3 w-3" />
          </Button>
        )}
        <Button variant="ghost" size="icon" className="h-6 w-6" title="Remove" onClick={onDelete}>
          <Trash2 className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}
