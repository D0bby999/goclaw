import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";

export interface NewsSource {
  id: string;
  agentId: string;
  name: string;
  sourceType: string;
  config: Record<string, unknown>;
  enabled: boolean;
  scrapeInterval: string;
  lastScrapedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export function useNewsSources(agentId: string) {
  const ws = useWs();
  const queryClient = useQueryClient();

  const { data: sources = [], isLoading: loading } = useQuery({
    queryKey: queryKeys.newsDigest.sources(agentId),
    queryFn: async () => {
      if (!ws.isConnected || !agentId) return [];
      const res = await ws.call<{ sources: NewsSource[] }>(Methods.NEWS_SOURCES_LIST, {
        agentId,
        enabledOnly: false,
      });
      return res.sources ?? [];
    },
    enabled: !!agentId,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.newsDigest.sources(agentId) }),
    [queryClient, agentId],
  );

  const createSource = useCallback(
    async (params: {
      name: string;
      sourceType: string;
      config?: Record<string, unknown>;
      scrapeInterval?: string;
    }) => {
      try {
        await ws.call(Methods.NEWS_SOURCES_CREATE, { agentId, ...params });
        await invalidate();
        toast.success("Source created", `${params.name} has been added`);
      } catch (err) {
        toast.error("Failed to create source", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, agentId, invalidate],
  );

  const updateSource = useCallback(
    async (id: string, patch: Record<string, unknown>) => {
      try {
        await ws.call(Methods.NEWS_SOURCES_UPDATE, { id, patch });
        await invalidate();
        toast.success("Source updated");
      } catch (err) {
        toast.error("Failed to update source", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidate],
  );

  const deleteSource = useCallback(
    async (id: string) => {
      try {
        await ws.call(Methods.NEWS_SOURCES_DELETE, { id });
        await invalidate();
        toast.success("Source deleted");
      } catch (err) {
        toast.error("Failed to delete source", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidate],
  );

  return { sources, loading, refresh: invalidate, createSource, updateSource, deleteSource };
}
