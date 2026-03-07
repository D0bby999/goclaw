import { useState } from "react";
import { Newspaper, Plus, Trash2, Pencil, RefreshCw, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { Pagination } from "@/components/shared/pagination";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useNewsSources, type NewsSource } from "./hooks/use-news-sources";
import { useNewsItems } from "./hooks/use-news-items";
import { NewsSourceFormDialog } from "./news-source-form-dialog";
import { useAgents } from "@/pages/agents/hooks/use-agents";
import { useMinLoading } from "@/hooks/use-min-loading";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { usePagination } from "@/hooks/use-pagination";
import { formatDate } from "@/lib/format";

type Tab = "sources" | "items";

export function NewsDigestPage() {
  const { agents } = useAgents();

  const [agentId, setAgentId] = useState<string>("");
  const selectedAgentId = agentId || agents[0]?.id || "";

  const { sources, loading: sourcesLoading, refresh: refreshSources, createSource, updateSource, deleteSource } = useNewsSources(selectedAgentId);
  const { items, count, loading: itemsLoading, refresh: refreshItems } = useNewsItems(selectedAgentId);

  const [tab, setTab] = useState<Tab>("sources");
  const [showForm, setShowForm] = useState(false);
  const [editSource, setEditSource] = useState<NewsSource | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<NewsSource | null>(null);

  const loading = tab === "sources" ? sourcesLoading : itemsLoading;
  const spinning = useMinLoading(loading);
  const showSkeleton = useDeferredLoading(loading && (tab === "sources" ? sources.length === 0 : items.length === 0));

  const sourcePagination = usePagination(sources);
  const itemPagination = usePagination(items);

  const refresh = () => {
    refreshSources();
    refreshItems();
  };

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title="News Digest"
        description="Manage news sources and view collected items"
        actions={
          <div className="flex gap-2">
            {agents.length > 1 && (
              <select
                className="rounded-md border bg-background px-3 py-1.5 text-sm"
                value={selectedAgentId}
                onChange={(e) => setAgentId(e.target.value)}
              >
                {agents.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.display_name || a.agent_key}
                  </option>
                ))}
              </select>
            )}
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={"h-3.5 w-3.5" + (spinning ? " animate-spin" : "")} /> Refresh
            </Button>
            {tab === "sources" && (
              <Button size="sm" onClick={() => { setEditSource(null); setShowForm(true); }} className="gap-1">
                <Plus className="h-3.5 w-3.5" /> Add Source
              </Button>
            )}
          </div>
        }
      />

      {/* Tabs */}
      <div className="mt-4 flex gap-1 border-b">
        {(["sources", "items"] as const).map((t) => (
          <button
            key={t}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
            onClick={() => setTab(t)}
          >
            {t === "sources" ? `Sources (${sources.length})` : `Items (${count})`}
          </button>
        ))}
      </div>

      <div className="mt-4">
        {showSkeleton ? (
          <TableSkeleton rows={5} />
        ) : tab === "sources" ? (
          <SourcesTab
            sources={sourcePagination.pageItems}
            total={sources.length}
            pagination={sourcePagination.pagination}
            setPage={sourcePagination.setPage}
            setPageSize={sourcePagination.setPageSize}
            onEdit={(s) => { setEditSource(s); setShowForm(true); }}
            onDelete={setDeleteTarget}
            onAdd={() => { setEditSource(null); setShowForm(true); }}
          />
        ) : (
          <ItemsTab
            items={itemPagination.pageItems}
            total={items.length}
            pagination={itemPagination.pagination}
            setPage={itemPagination.setPage}
            setPageSize={itemPagination.setPageSize}
          />
        )}
      </div>

      <NewsSourceFormDialog
        open={showForm}
        onOpenChange={setShowForm}
        onSubmit={createSource}
        onUpdate={updateSource}
        editSource={editSource}
      />

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setDeleteTarget(null)}
          title="Delete Source"
          description={`Delete "${deleteTarget.name}"? This will also remove all items from this source.`}
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={async () => {
            await deleteSource(deleteTarget.id);
            setDeleteTarget(null);
          }}
        />
      )}
    </div>
  );
}

/* ---------- Sources Tab ---------- */

function SourcesTab({
  sources,
  total,
  pagination,
  setPage,
  setPageSize,
  onEdit,
  onDelete,
  onAdd,
}: {
  sources: NewsSource[];
  total: number;
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
  setPage: (p: number) => void;
  setPageSize: (s: number) => void;
  onEdit: (s: NewsSource) => void;
  onDelete: (s: NewsSource) => void;
  onAdd: () => void;
}) {
  if (total === 0) {
    return (
      <EmptyState
        icon={Newspaper}
        title="No sources"
        description="Add a news source to start collecting items."
        action={
          <Button size="sm" onClick={onAdd} className="gap-1">
            <Plus className="h-3.5 w-3.5" /> Add Source
          </Button>
        }
      />
    );
  }

  return (
    <div className="rounded-md border overflow-x-auto">
      <table className="w-full min-w-[600px] text-sm">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="px-4 py-3 text-left font-medium">Name</th>
            <th className="px-4 py-3 text-left font-medium">Type</th>
            <th className="px-4 py-3 text-left font-medium">Interval</th>
            <th className="px-4 py-3 text-left font-medium">Status</th>
            <th className="px-4 py-3 text-left font-medium">Last Scraped</th>
            <th className="px-4 py-3 text-right font-medium">Actions</th>
          </tr>
        </thead>
        <tbody>
          {sources.map((s) => (
            <tr key={s.id} className="border-b last:border-0 hover:bg-muted/30">
              <td className="px-4 py-3 font-medium">{s.name}</td>
              <td className="px-4 py-3">
                <Badge variant="outline">{s.sourceType}</Badge>
              </td>
              <td className="px-4 py-3">
                <Badge variant="secondary">{s.scrapeInterval}</Badge>
              </td>
              <td className="px-4 py-3">
                <Badge variant={s.enabled ? "success" : "destructive"}>
                  {s.enabled ? "enabled" : "disabled"}
                </Badge>
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {s.lastScrapedAt ? formatDate(s.lastScrapedAt) : "never"}
              </td>
              <td className="px-4 py-3">
                <div className="flex justify-end gap-1">
                  <Button variant="ghost" size="icon" title="Edit" onClick={() => onEdit(s)}>
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" title="Delete" onClick={() => onDelete(s)}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <Pagination
        page={pagination.page}
        pageSize={pagination.pageSize}
        total={pagination.total}
        totalPages={pagination.totalPages}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </div>
  );
}

/* ---------- Items Tab ---------- */

function ItemsTab({
  items,
  total,
  pagination,
  setPage,
  setPageSize,
}: {
  items: import("./hooks/use-news-items").NewsItem[];
  total: number;
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
  setPage: (p: number) => void;
  setPageSize: (s: number) => void;
}) {
  if (total === 0) {
    return (
      <EmptyState
        icon={Newspaper}
        title="No items yet"
        description="Items will appear here once sources are scraped."
      />
    );
  }

  return (
    <div className="rounded-md border overflow-x-auto">
      <table className="w-full min-w-[700px] text-sm">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="px-4 py-3 text-left font-medium">Title</th>
            <th className="px-4 py-3 text-left font-medium">Source</th>
            <th className="px-4 py-3 text-left font-medium">Categories</th>
            <th className="px-4 py-3 text-left font-medium">Date</th>
            <th className="px-4 py-3 text-right font-medium">Link</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.id} className="border-b last:border-0 hover:bg-muted/30">
              <td className="max-w-[300px] truncate px-4 py-3 font-medium" title={item.title}>
                {item.title}
              </td>
              <td className="px-4 py-3">
                {item.sourceName ? (
                  <Badge variant="outline">{item.sourceName}</Badge>
                ) : item.sourceType ? (
                  <Badge variant="outline">{item.sourceType}</Badge>
                ) : (
                  <span className="text-muted-foreground">—</span>
                )}
              </td>
              <td className="px-4 py-3">
                <div className="flex flex-wrap gap-1">
                  {(item.categories || []).slice(0, 3).map((cat) => (
                    <Badge key={cat} variant="secondary">{cat}</Badge>
                  ))}
                  {(item.categories || []).length > 3 && (
                    <Badge variant="secondary">+{item.categories.length - 3}</Badge>
                  )}
                </div>
              </td>
              <td className="px-4 py-3 text-muted-foreground whitespace-nowrap">
                {formatDate(item.publishedAt || item.scrapedAt)}
              </td>
              <td className="px-4 py-3">
                <div className="flex justify-end">
                  {item.url && (
                    <a href={item.url} target="_blank" rel="noopener noreferrer">
                      <Button variant="ghost" size="icon" title="Open link">
                        <ExternalLink className="h-3.5 w-3.5" />
                      </Button>
                    </a>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <Pagination
        page={pagination.page}
        pageSize={pagination.pageSize}
        total={pagination.total}
        totalPages={pagination.totalPages}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </div>
  );
}
