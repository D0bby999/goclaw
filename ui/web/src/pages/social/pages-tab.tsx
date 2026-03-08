import { useState } from "react";
import { FileText, Plus, RefreshCw, Star, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { CardSkeleton } from "@/components/shared/loading-skeleton";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { PlatformIcon } from "./platform-icons";
import { PageConnectDialog } from "./page-connect-dialog";
import type { SocialAccount, SocialPage } from "@/types/social";

// SocialPage augmented with platform from parent account.
type PageWithPlatform = SocialPage & { _platform?: string };

interface PagesTabProps {
  pages: PageWithPlatform[];
  accounts: SocialAccount[];
  loading: boolean;
  onSyncAll: () => Promise<void>;
  onSetDefault: (pageId: string) => Promise<void>;
  onDelete: (pageId: string) => Promise<void>;
  onCreate: (params: {
    accountId: string;
    pageId: string;
    pageName: string;
    pageToken: string;
    pageType: string;
  }) => Promise<void>;
}

export function PagesTab({ pages, accounts, loading, onSyncAll, onSetDefault, onDelete, onCreate }: PagesTabProps) {
  const [showForm, setShowForm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PageWithPlatform | null>(null);
  const [syncing, setSyncing] = useState(false);
  const showSkeleton = useDeferredLoading(loading && pages.length === 0);

  const handleSyncAll = async () => {
    setSyncing(true);
    try {
      await onSyncAll();
    } finally {
      setSyncing(false);
    }
  };

  if (showSkeleton) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => <CardSkeleton key={i} />)}
      </div>
    );
  }

  if (pages.length === 0) {
    return (
      <>
        <EmptyState
          icon={FileText}
          title="No pages found"
          description="Add a page manually or sync from Facebook / Instagram accounts."
          action={
            <div className="flex gap-2">
              <Button size="sm" variant="outline" onClick={handleSyncAll} disabled={syncing} className="gap-1">
                <RefreshCw className={`h-3.5 w-3.5 ${syncing ? "animate-spin" : ""}`} /> Sync All
              </Button>
              <Button size="sm" onClick={() => setShowForm(true)} className="gap-1">
                <Plus className="h-3.5 w-3.5" /> Add Page
              </Button>
            </div>
          }
        />
        <PageConnectDialog
          open={showForm}
          onOpenChange={setShowForm}
          accounts={accounts}
          onSubmit={onCreate}
        />
      </>
    );
  }

  return (
    <>
      <div className="mb-4 flex justify-end gap-2">
        <Button size="sm" variant="outline" onClick={handleSyncAll} disabled={syncing} className="gap-1">
          <RefreshCw className={`h-3.5 w-3.5 ${syncing ? "animate-spin" : ""}`} /> Sync All
        </Button>
        <Button size="sm" onClick={() => setShowForm(true)} className="gap-1">
          <Plus className="h-3.5 w-3.5" /> Add Page
        </Button>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {pages.map((page) => (
          <PageCard
            key={page.id}
            page={page}
            onSetDefault={() => onSetDefault(page.id)}
            onDelete={() => setDeleteTarget(page)}
          />
        ))}
      </div>

      <PageConnectDialog
        open={showForm}
        onOpenChange={setShowForm}
        accounts={accounts}
        onSubmit={onCreate}
      />

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setDeleteTarget(null)}
          title="Remove Page"
          description={`Remove "${deleteTarget.page_name || deleteTarget.page_id}"? This won't affect published posts.`}
          confirmLabel="Remove"
          variant="destructive"
          onConfirm={async () => {
            await onDelete(deleteTarget.id);
            setDeleteTarget(null);
          }}
        />
      )}
    </>
  );
}

function PageCard({
  page,
  onSetDefault,
  onDelete,
}: {
  page: PageWithPlatform;
  onSetDefault: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-3">
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-3 min-w-0">
          {page.avatar_url ? (
            <img src={page.avatar_url} alt="" className="h-10 w-10 rounded-full shrink-0 object-cover" />
          ) : (
            <div className="h-10 w-10 rounded-full bg-muted shrink-0 flex items-center justify-center">
              <FileText className="h-4 w-4 text-muted-foreground" />
            </div>
          )}
          <div className="min-w-0">
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className="font-medium text-sm truncate">{page.page_name || page.page_id}</span>
              {page.is_default && (
                <Star className="h-3.5 w-3.5 fill-yellow-400 text-yellow-400 shrink-0" />
              )}
            </div>
            <div className="flex items-center gap-1.5 mt-0.5">
              <span className="text-xs rounded bg-muted px-1.5 py-0.5 text-muted-foreground">
                {page.page_type}
              </span>
              {page._platform && (
                <PlatformIcon platform={page._platform as Parameters<typeof PlatformIcon>[0]["platform"]} className="h-3.5 w-3.5" />
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Footer actions */}
      <div className="flex justify-end gap-1 pt-1 border-t">
        {!page.is_default && (
          <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={onSetDefault}>
            <Star className="h-3.5 w-3.5" /> Set default
          </Button>
        )}
        <Button variant="ghost" size="sm" className="h-7 text-destructive hover:text-destructive gap-1 text-xs" onClick={onDelete}>
          <Trash2 className="h-3.5 w-3.5" /> Remove
        </Button>
      </div>
    </div>
  );
}
