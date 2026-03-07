import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import { queryKeys } from "@/lib/query-keys";

export interface NewsItem {
  id: string;
  sourceId?: string;
  agentId: string;
  url: string;
  title: string;
  content?: string;
  summary?: string;
  categories: string[];
  tags: string[];
  sentiment?: string;
  sourceType?: string;
  sourceName?: string;
  publishedAt?: string;
  scrapedAt: string;
  createdAt: string;
}

export function useNewsItems(agentId: string) {
  const ws = useWs();
  const queryClient = useQueryClient();

  const { data, isLoading: loading } = useQuery({
    queryKey: queryKeys.newsDigest.items(agentId),
    queryFn: async () => {
      if (!ws.isConnected || !agentId) return { items: [] as NewsItem[], count: 0 };
      const res = await ws.call<{ items: NewsItem[]; count: number }>(Methods.NEWS_ITEMS_LIST, {
        agentId,
        limit: 100,
        offset: 0,
      });
      return { items: res.items ?? [], count: res.count ?? 0 };
    },
    enabled: !!agentId,
  });

  const items = data?.items ?? [];
  const count = data?.count ?? 0;

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.newsDigest.items(agentId) }),
    [queryClient, agentId],
  );

  return { items, count, loading, refresh: invalidate };
}
