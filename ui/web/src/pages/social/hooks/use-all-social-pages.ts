import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { SocialAccount, SocialPage } from "@/types/social";

export function useAllSocialPages(accounts: SocialAccount[]) {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const queryClient = useQueryClient();

  const accountIds = accounts.map((a) => a.id);
  const allPagesKey = ["social", "all-pages", accountIds] as const;

  const { data: pages = [], isLoading: loading } = useQuery({
    queryKey: allPagesKey,
    queryFn: async () => {
      const results = await Promise.all(
        accounts.map(async (acc) => {
          try {
            const res = await http.get<{ pages: SocialPage[]; count: number }>(
              `/v1/social/accounts/${acc.id}/pages`,
            );
            return (res.pages ?? []).map((p) => ({ ...p, _platform: acc.platform }));
          } catch {
            return [];
          }
        }),
      );
      return results.flat();
    },
    enabled: connected && accounts.length > 0,
  });

  const invalidate = useCallback(
    () => {
      queryClient.invalidateQueries({ queryKey: ["social", "all-pages"] });
      // Also invalidate per-account page queries so account-pages-panel stays in sync.
      for (const acc of accounts) {
        queryClient.invalidateQueries({ queryKey: queryKeys.social.pages(acc.id) });
      }
    },
    [queryClient, accounts],
  );

  const syncPages = useCallback(
    async (accountId: string) => {
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
    [http, invalidate],
  );

  const syncAll = useCallback(
    async () => {
      const syncable = accounts.filter((a) =>
        a.platform === "facebook" || a.platform === "instagram",
      );
      if (syncable.length === 0) {
        toast.info("No syncable accounts", "Only Facebook and Instagram accounts support page sync.");
        return;
      }
      let synced = 0;
      for (const acc of syncable) {
        try {
          await http.post(`/v1/social/accounts/${acc.id}/pages/sync`, {});
          synced++;
        } catch {
          // continue
        }
      }
      await invalidate();
      toast.success(`Synced pages from ${synced} account${synced !== 1 ? "s" : ""}`);
    },
    [http, accounts, invalidate],
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

  const createPage = useCallback(
    async (params: {
      accountId: string;
      pageId: string;
      pageName: string;
      pageToken: string;
      pageType: string;
    }) => {
      await http.post(`/v1/social/accounts/${params.accountId}/pages`, {
        page_id: params.pageId,
        page_name: params.pageName,
        page_token: params.pageToken,
        page_type: params.pageType,
      });
      await invalidate();
    },
    [http, invalidate],
  );

  return { pages, loading, syncPages, syncAll, setDefault, deletePage, createPage, refresh: invalidate };
}
