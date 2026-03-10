import { useState, useRef, useCallback } from "react";
import {
  Database, Plus, RefreshCw, Trash2, Upload, Search,
  FileText, RotateCw, ChevronLeft,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useAgents } from "@/pages/agents/hooks/use-agents";
import { useMinLoading } from "@/hooks/use-min-loading";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import {
  useKBCollections,
  useKBDocuments,
  useKBUpload,
  type KBCollection,
  type KBDocument,
} from "./hooks/use-kb";
import { KBCollectionDialog } from "./kb-collection-dialog";
import { KBSearchDialog } from "./kb-search-dialog";

const statusColors: Record<string, string> = {
  pending: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400",
  processing: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400",
  ready: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  error: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
  active: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
};

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

function formatDate(ts: number): string {
  return new Date(ts).toLocaleString();
}

export function KBPage() {
  const { agents } = useAgents();
  const [agentId, setAgentId] = useState("");
  const [selectedCollection, setSelectedCollection] = useState<KBCollection | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<KBCollection | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);

  const {
    collections, loading, fetching, refresh, createCollection, deleteCollection,
  } = useKBCollections(agentId);

  const spinning = useMinLoading(fetching);
  const showSkeleton = useDeferredLoading(loading && collections.length === 0);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Knowledge Base"
        description="Manage document collections for RAG-powered agent responses"
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={refresh} disabled={!agentId}>
              <RefreshCw className={`h-4 w-4 mr-1 ${spinning ? "animate-spin" : ""}`} />
              Refresh
            </Button>
            {agentId && (
              <>
                <Button variant="outline" size="sm" onClick={() => setSearchOpen(true)}>
                  <Search className="h-4 w-4 mr-1" />
                  Search
                </Button>
                <Button size="sm" onClick={() => setCreateOpen(true)}>
                  <Plus className="h-4 w-4 mr-1" />
                  New Collection
                </Button>
              </>
            )}
          </div>
        }
      />

      {/* Agent selector */}
      <div className="flex items-center gap-3">
        <Label>Agent</Label>
        <select
          className="border rounded-md px-3 py-1.5 text-sm bg-background"
          value={agentId}
          onChange={(e) => { setAgentId(e.target.value); setSelectedCollection(null); }}
        >
          <option value="">Select agent...</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>{a.display_name || a.agent_key}</option>
          ))}
        </select>
      </div>

      {!agentId ? (
        <EmptyState icon={Database} title="Select an agent" description="Choose an agent to manage its knowledge base collections" />
      ) : selectedCollection ? (
        <CollectionDetailView
          agentId={agentId}
          collection={selectedCollection}
          onBack={() => setSelectedCollection(null)}
        />
      ) : showSkeleton ? (
        <TableSkeleton rows={3} />
      ) : collections.length === 0 ? (
        <EmptyState icon={Database} title="No collections" description="Create a collection to start uploading documents" />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {collections.map((col) => (
            <div
              key={col.id}
              className="border rounded-lg p-4 hover:border-primary/50 cursor-pointer transition-colors"
              onClick={() => setSelectedCollection(col)}
            >
              <div className="flex items-start justify-between">
                <div className="space-y-1">
                  <h3 className="font-medium">{col.name}</h3>
                  {col.description && (
                    <p className="text-sm text-muted-foreground line-clamp-2">{col.description}</p>
                  )}
                </div>
                <Badge variant="outline" className={statusColors[col.status] || ""}>
                  {col.status}
                </Badge>
              </div>
              <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
                <span>{col.doc_count} document{col.doc_count !== 1 ? "s" : ""}</span>
                <span>{formatDate(col.updated_at)}</span>
              </div>
              <div className="mt-2 flex justify-end">
                <Button
                  variant="ghost" size="sm"
                  onClick={(e) => { e.stopPropagation(); setDeleteTarget(col); }}
                >
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Dialogs */}
      <KBCollectionDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreate={createCollection}
      />
      <KBSearchDialog
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        agentId={agentId}
      />
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(v) => { if (!v) setDeleteTarget(null); }}
        title="Delete Collection"
        description={`Delete "${deleteTarget?.name}" and all its documents? This cannot be undone.`}
        variant="destructive"
        onConfirm={async () => {
          if (deleteTarget) await deleteCollection(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

// --- Collection Detail View ---

function CollectionDetailView({
  agentId, collection, onBack,
}: {
  agentId: string;
  collection: KBCollection;
  onBack: () => void;
}) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [deleteTarget, setDeleteTarget] = useState<KBDocument | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const {
    documents, loading, fetching, refresh, deleteDocument, reprocessDocument,
  } = useKBDocuments(agentId, collection.id);

  const { upload, uploading } = useKBUpload(agentId, collection.id);

  const spinning = useMinLoading(fetching);

  const handleFileSelect = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files?.length) return;
    for (const file of Array.from(files)) {
      await upload(file);
    }
    e.target.value = "";
  }, [upload]);

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    const files = e.dataTransfer.files;
    for (const file of Array.from(files)) {
      await upload(file);
    }
  }, [upload]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" onClick={onBack}>
          <ChevronLeft className="h-4 w-4 mr-1" />
          Back
        </Button>
        <h2 className="text-lg font-semibold">{collection.name}</h2>
        <Badge variant="outline">{collection.doc_count} docs</Badge>
        <div className="ml-auto flex gap-2">
          <Button variant="outline" size="sm" onClick={refresh}>
            <RefreshCw className={`h-4 w-4 mr-1 ${spinning ? "animate-spin" : ""}`} />
          </Button>
          <Button
            size="sm"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
          >
            <Upload className="h-4 w-4 mr-1" />
            {uploading ? "Uploading..." : "Upload"}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            accept=".pdf,.docx,.csv,.txt,.md"
            multiple
            onChange={handleFileSelect}
          />
        </div>
      </div>

      {collection.description && (
        <p className="text-sm text-muted-foreground">{collection.description}</p>
      )}

      {/* Drop zone */}
      <div
        className="border-2 border-dashed rounded-lg p-8 text-center text-muted-foreground hover:border-primary/50 transition-colors"
        onDragOver={(e) => e.preventDefault()}
        onDrop={handleDrop}
      >
        <Upload className="h-8 w-8 mx-auto mb-2 opacity-50" />
        <p className="text-sm">Drop files here or click Upload</p>
        <p className="text-xs mt-1">Supported: PDF, DOCX, CSV, TXT, MD (max 10MB)</p>
      </div>

      {/* Documents table */}
      {loading && documents.length === 0 ? (
        <TableSkeleton rows={3} />
      ) : documents.length === 0 ? (
        <EmptyState icon={FileText} title="No documents" description="Upload documents to build this collection's knowledge base" />
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th className="text-left p-3 font-medium">Filename</th>
                <th className="text-left p-3 font-medium">Type</th>
                <th className="text-left p-3 font-medium">Size</th>
                <th className="text-left p-3 font-medium">Status</th>
                <th className="text-left p-3 font-medium">Chunks</th>
                <th className="text-left p-3 font-medium">Updated</th>
                <th className="text-right p-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {documents.map((doc) => (
                <tr key={doc.id} className="border-t hover:bg-muted/30">
                  <td className="p-3 font-medium">{doc.filename}</td>
                  <td className="p-3 text-muted-foreground">{doc.mime_type.split("/").pop()}</td>
                  <td className="p-3 text-muted-foreground">{formatBytes(doc.file_size)}</td>
                  <td className="p-3">
                    <Badge variant="outline" className={`text-xs ${statusColors[doc.status] || ""}`}>
                      {doc.status}
                    </Badge>
                    {doc.error_message && (
                      <span className="ml-1 text-xs text-destructive" title={doc.error_message}>!</span>
                    )}
                  </td>
                  <td className="p-3 text-muted-foreground">
                    {doc.chunk_count} / {doc.embedded_count} emb
                  </td>
                  <td className="p-3 text-muted-foreground text-xs">{formatDate(doc.updated_at)}</td>
                  <td className="p-3 text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost" size="sm"
                        onClick={() => reprocessDocument(doc.id)}
                        title="Re-process"
                      >
                        <RotateCw className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost" size="sm"
                        onClick={() => setDeleteTarget(doc)}
                      >
                        <Trash2 className="h-3.5 w-3.5 text-destructive" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(v) => { if (!v) setDeleteTarget(null); }}
        title="Delete Document"
        description={`Delete "${deleteTarget?.filename}"? This cannot be undone.`}
        variant="destructive"
        onConfirm={async () => {
          if (!deleteTarget) return;
          setDeleteLoading(true);
          try { await deleteDocument(deleteTarget.id); } finally { setDeleteLoading(false); }
          setDeleteTarget(null);
        }}
        loading={deleteLoading}
      />
    </div>
  );
}
