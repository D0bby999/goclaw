import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWs, useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods } from "@/api/protocol";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { SocialAccount } from "@/types/social";

export function useSocialAccounts() {
  const ws = useWs();
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const queryClient = useQueryClient();

  const { data: accounts = [], isLoading: loading } = useQuery({
    queryKey: queryKeys.social.accounts,
    queryFn: async () => {
      try {
        const res = await http.get<{ accounts: SocialAccount[]; count: number }>("/v1/social/accounts");
        if (res.accounts) return res.accounts;
      } catch {
        // fallback to WS
      }
      if (!ws.isConnected) return [];
      const res = await ws.call<{ accounts: SocialAccount[] }>(Methods.SOCIAL_ACCOUNTS_LIST);
      return res.accounts ?? [];
    },
    enabled: connected,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.social.accounts }),
    [queryClient],
  );

  const createAccount = useCallback(
    async (params: {
      platform: string;
      platform_user_id: string;
      access_token: string;
      platform_username?: string;
      display_name?: string;
    }) => {
      try {
        await ws.call(Methods.SOCIAL_ACCOUNTS_CREATE, params);
        await invalidate();
        toast.success("Account connected", `${params.platform} account added`);
      } catch (err) {
        toast.error("Failed to connect account", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidate],
  );

  const updateAccount = useCallback(
    async (id: string, updates: Record<string, unknown>) => {
      try {
        await ws.call(Methods.SOCIAL_ACCOUNTS_UPDATE, { id, updates });
        await invalidate();
        toast.success("Account updated");
      } catch (err) {
        toast.error("Failed to update account", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidate],
  );

  const deleteAccount = useCallback(
    async (id: string) => {
      try {
        await ws.call(Methods.SOCIAL_ACCOUNTS_DELETE, { id });
        await invalidate();
        toast.success("Account disconnected");
      } catch (err) {
        toast.error("Failed to disconnect account", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [ws, invalidate],
  );

  return { accounts, loading, refresh: invalidate, createAccount, updateAccount, deleteAccount };
}
