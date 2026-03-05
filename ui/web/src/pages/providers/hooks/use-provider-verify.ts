import { useState, useCallback } from "react";
import { useHttp } from "@/hooks/use-ws";
import { OAUTH_PROVIDER_ID } from "./use-providers";

interface VerifyResult {
  valid: boolean;
  error?: string;
}

export function useProviderVerify() {
  const http = useHttp();
  const [verifying, setVerifying] = useState(false);
  const [result, setResult] = useState<VerifyResult | null>(null);

  const verify = useCallback(
    async (providerId: string, model: string) => {
      // OAuth provider can't be verified via backend — auto-accept
      if (providerId === OAUTH_PROVIDER_ID) {
        const r: VerifyResult = { valid: true };
        setResult(r);
        return r;
      }

      setVerifying(true);
      setResult(null);
      try {
        const res = await http.post<VerifyResult>(
          `/v1/providers/${providerId}/verify`,
          { model },
        );
        setResult(res);
        return res;
      } catch (err) {
        const r: VerifyResult = {
          valid: false,
          error: err instanceof Error ? err.message : "Verification failed",
        };
        setResult(r);
        return r;
      } finally {
        setVerifying(false);
      }
    },
    [http],
  );

  const reset = useCallback(() => setResult(null), []);

  return { verify, verifying, result, reset };
}
