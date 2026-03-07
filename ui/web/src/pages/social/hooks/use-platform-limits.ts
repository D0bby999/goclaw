import { useQuery } from "@tanstack/react-query";
import { useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { queryKeys } from "@/lib/query-keys";
import type { PlatformLimits } from "@/types/social";

export function usePlatformLimits() {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);

  const { data: platforms = {} } = useQuery({
    queryKey: queryKeys.social.platforms,
    queryFn: async () => {
      const res = await http.get<{ platforms: Record<string, PlatformLimits> }>("/v1/social/platforms");
      return res.platforms ?? {};
    },
    enabled: connected,
    staleTime: 60 * 60 * 1000, // 1 hour
  });

  return { platforms };
}
