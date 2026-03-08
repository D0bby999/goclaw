import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { SocialPage } from "@/types/social";

export function useSocialPages(accountId: string | null) {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const queryClient = useQueryClient();

  const { data: pages = [], isLoading: loading } = useQuery({
    queryKey: queryKeys.social.pages(accountId ?? ""),
    queryFn: async () => {
      if (!accountId) return [];
      const res = await http.get<{ pages: SocialPage[]; count: number }>(
        `/v1/social/accounts/${accountId}/pages`,
      );
      return res.pages ?? [];
    },
    enabled: connected && !!accountId,
  });

  const invalidate = useCallback(
    () => {
      if (accountId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.social.pages(accountId) });
      }
    },
    [queryClient, accountId],
  );

  const syncPages = useCallback(
    async () => {
      if (!accountId) return;
      try {
        await http.post<{ pages: SocialPage[] }>(
          `/v1/social/accounts/${accountId}/pages/sync`,
          {},
        );
        await invalidate();
        toast.success("Pages synced");
      } catch (err) {
        toast.error("Failed to sync pages", err instanceof Error ? err.message : "Unknown error");
      }
    },
    [http, accountId, invalidate],
  );

  const setDefault = useCallback(
    async (pageId: string) => {
      try {
        await http.put(`/v1/social/pages/${pageId}/default`, {});
        await invalidate();
        toast.success("Default page updated");
      } catch (err) {
        toast.error("Failed to set default", err instanceof Error ? err.message : "Unknown error");
      }
    },
    [http, invalidate],
  );

  const deletePage = useCallback(
    async (pageId: string) => {
      try {
        await http.delete(`/v1/social/pages/${pageId}`);
        await invalidate();
        toast.success("Page removed");
      } catch (err) {
        toast.error("Failed to remove page", err instanceof Error ? err.message : "Unknown error");
      }
    },
    [http, invalidate],
  );

  return { pages, loading, syncPages, setDefault, deletePage, refresh: invalidate };
}
