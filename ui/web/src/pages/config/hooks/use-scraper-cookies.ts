import { useState, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import { queryKeys } from "@/lib/query-keys";

export interface ScraperCookieEntry {
  label: string;
  platform: string;
  cookies: string;
  created_at: string;
  updated_at: string;
}

export function useScraperCookies() {
  const ws = useWs();
  const qc = useQueryClient();
  const [loginPending, setLoginPending] = useState<string | null>(null);

  const { data, isLoading: loading } = useQuery({
    queryKey: queryKeys.scraperCookies.all,
    queryFn: async () => {
      if (!ws.isConnected) return [];
      const res = await ws.call<{ entries: ScraperCookieEntry[] }>(Methods.SCRAPER_COOKIES_LIST);
      return res.entries ?? [];
    },
  });

  const cookies = data ?? [];
  const invalidate = useCallback(() => qc.invalidateQueries({ queryKey: queryKeys.scraperCookies.all }), [qc]);

  const addManual = useCallback(
    async (platform: string, label: string, cookieValue: string) => {
      await ws.call(Methods.SCRAPER_COOKIES_SET, { platform, label, cookies: cookieValue });
      await invalidate();
    },
    [ws, invalidate],
  );

  const remove = useCallback(
    async (platform: string, label: string) => {
      await ws.call(Methods.SCRAPER_COOKIES_DELETE, { platform, label });
      await invalidate();
    },
    [ws, invalidate],
  );

  const startLogin = useCallback(
    async (platform: string, label?: string) => {
      setLoginPending(platform);
      try {
        await ws.call(Methods.SCRAPER_COOKIES_LOGIN, { platform, label }, 130_000);
        await invalidate();
      } finally {
        setLoginPending(null);
      }
    },
    [ws, invalidate],
  );

  return { cookies, loading, loginPending, addManual, remove, startLogin };
}
