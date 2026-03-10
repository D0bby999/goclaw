import { useCallback, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useHttp } from "@/hooks/use-ws";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";

// --- Types ---

export interface KBCollection {
  id: string;
  agent_id: string;
  name: string;
  description: string;
  doc_count: number;
  status: string;
  created_at: number;
  updated_at: number;
}

export interface KBDocument {
  id: string;
  collection_id: string;
  agent_id: string;
  filename: string;
  mime_type: string;
  file_size: number;
  version: number;
  status: string;
  error_message?: string;
  chunk_count: number;
  embedded_count: number;
  created_at: number;
  updated_at: number;
}

export interface KBChunk {
  id: string;
  document_id: string;
  chunk_index: number;
  text: string;
  start_line: number;
  end_line: number;
  has_embedding: boolean;
}

export interface KBSearchResult {
  document_id: string;
  filename: string;
  collection_id: string;
  chunk_index: number;
  start_line: number;
  end_line: number;
  text: string;
  score: number;
}

// --- Collections Hook ---

export function useKBCollections(agentId: string) {
  const http = useHttp();
  const queryClient = useQueryClient();

  const { data, isLoading, isFetching } = useQuery({
    queryKey: queryKeys.kb.collections(agentId),
    queryFn: async () => {
      if (!agentId) return [];
      const res = await http.get<{ collections: KBCollection[] }>(
        `/v1/agents/${agentId}/kb/collections`,
      );
      return res?.collections ?? [];
    },
    enabled: !!agentId,
    placeholderData: (prev) => prev,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.kb.collections(agentId) }),
    [queryClient, agentId],
  );

  const createCollection = useCallback(
    async (name: string, description: string) => {
      try {
        await http.post(`/v1/agents/${agentId}/kb/collections`, { name, description });
        await invalidate();
        toast.success("Collection created", name);
      } catch (err) {
        toast.error("Failed to create collection", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, agentId, invalidate],
  );

  const deleteCollection = useCallback(
    async (id: string) => {
      try {
        await http.delete(`/v1/agents/${agentId}/kb/collections/${id}`);
        await invalidate();
        toast.success("Collection deleted");
      } catch (err) {
        toast.error("Failed to delete collection", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, agentId, invalidate],
  );

  return {
    collections: data ?? [],
    loading: isLoading,
    fetching: isFetching,
    refresh: invalidate,
    createCollection,
    deleteCollection,
  };
}

// --- Documents Hook ---

export function useKBDocuments(agentId: string, collectionId: string) {
  const http = useHttp();
  const queryClient = useQueryClient();

  const { data, isLoading, isFetching } = useQuery({
    queryKey: queryKeys.kb.documents(collectionId),
    queryFn: async () => {
      if (!collectionId) return [];
      const res = await http.get<{ documents: KBDocument[] }>(
        `/v1/agents/${agentId}/kb/collections/${collectionId}/documents`,
      );
      return res?.documents ?? [];
    },
    enabled: !!collectionId && !!agentId,
    placeholderData: (prev) => prev,
    refetchInterval: (query) => {
      // Poll if any doc is processing
      const docs = query.state.data;
      if (docs?.some((d: KBDocument) => d.status === "pending" || d.status === "processing")) {
        return 3000;
      }
      return false;
    },
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.kb.documents(collectionId) }),
    [queryClient, collectionId],
  );

  const deleteDocument = useCallback(
    async (docId: string) => {
      try {
        await http.delete(`/v1/agents/${agentId}/kb/documents/${docId}`);
        await invalidate();
        toast.success("Document deleted");
      } catch (err) {
        toast.error("Failed to delete document", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, agentId, invalidate],
  );

  const reprocessDocument = useCallback(
    async (docId: string) => {
      try {
        await http.post(`/v1/agents/${agentId}/kb/documents/${docId}/reprocess`, {});
        await invalidate();
        toast.success("Re-processing started");
      } catch (err) {
        toast.error("Failed to reprocess", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, agentId, invalidate],
  );

  return {
    documents: data ?? [],
    loading: isLoading,
    fetching: isFetching,
    refresh: invalidate,
    deleteDocument,
    reprocessDocument,
  };
}

// --- Upload Hook ---

export function useKBUpload(agentId: string, collectionId: string) {
  const [uploading, setUploading] = useState(false);
  const queryClient = useQueryClient();

  const upload = useCallback(
    async (file: File) => {
      setUploading(true);
      try {
        const formData = new FormData();
        formData.append("file", file);
        const token = localStorage.getItem("goclaw:token") || "";
        const res = await fetch(
          `/v1/agents/${agentId}/kb/collections/${collectionId}/documents`,
          {
            method: "POST",
            headers: { Authorization: `Bearer ${token}` },
            body: formData,
          },
        );
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: "upload failed" }));
          throw new Error(err.error || "upload failed");
        }
        queryClient.invalidateQueries({ queryKey: queryKeys.kb.documents(collectionId) });
        toast.success("Document uploaded", file.name);
      } catch (err) {
        toast.error("Upload failed", err instanceof Error ? err.message : "Unknown error");
        throw err;
      } finally {
        setUploading(false);
      }
    },
    [agentId, collectionId, queryClient],
  );

  return { upload, uploading };
}

// --- Search Hook ---

export function useKBSearch(agentId: string) {
  const http = useHttp();
  const [results, setResults] = useState<KBSearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  const search = useCallback(
    async (query: string, collectionIds?: string[], maxResults?: number) => {
      setSearching(true);
      try {
        const res = await http.post<{ results: KBSearchResult[]; count: number }>(
          `/v1/agents/${agentId}/kb/search`,
          {
            query,
            collection_ids: collectionIds || [],
            max_results: maxResults || 10,
          },
        );
        setResults(res?.results ?? []);
        return res?.results ?? [];
      } catch (err) {
        toast.error("Search failed", err instanceof Error ? err.message : "Unknown error");
        setResults([]);
        return [];
      } finally {
        setSearching(false);
      }
    },
    [http, agentId],
  );

  return { results, searching, search, setResults };
}
