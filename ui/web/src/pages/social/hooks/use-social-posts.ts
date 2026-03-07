import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWs, useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods } from "@/api/protocol";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { SocialPost } from "@/types/social";

interface ListParams {
  status?: string;
  limit?: number;
  offset?: number;
}

export function useSocialPosts(params: ListParams = {}) {
  const ws = useWs();
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const queryClient = useQueryClient();

  const queryParams: Record<string, unknown> = { status: params.status, limit: params.limit, offset: params.offset };

  const { data, isLoading: loading } = useQuery({
    queryKey: queryKeys.social.posts(queryParams),
    queryFn: async () => {
      try {
        const httpParams: Record<string, string> = {};
        if (params.status) httpParams.status = params.status;
        if (params.limit) httpParams.limit = String(params.limit);
        if (params.offset) httpParams.offset = String(params.offset);
        const res = await http.get<{ posts: SocialPost[]; total: number }>("/v1/social/posts", httpParams);
        return { posts: res.posts ?? [], total: res.total ?? 0 };
      } catch {
        // fallback to WS
      }
      if (!ws.isConnected) return { posts: [], total: 0 };
      const res = await ws.call<{ posts: SocialPost[]; total: number }>(Methods.SOCIAL_POSTS_LIST, queryParams);
      return { posts: res.posts ?? [], total: res.total ?? 0 };
    },
    enabled: connected,
  });

  const posts = data?.posts ?? [];
  const total = data?.total ?? 0;

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: ["social", "posts"] }),
    [queryClient],
  );

  const invalidatePost = useCallback(
    (id: string) => queryClient.invalidateQueries({ queryKey: queryKeys.social.post(id) }),
    [queryClient],
  );

  const getPost = useCallback(
    async (id: string) => {
      const res = await http.get<{ post: SocialPost }>(`/v1/social/posts/${id}`);
      return res.post;
    },
    [http],
  );

  const createPost = useCallback(
    async (params: { title?: string; content: string; post_type?: string; scheduled_at?: string }) => {
      try {
        const body = { ...params, post_type: params.post_type || "standard" };
        const res = await http.post<{ post: SocialPost }>("/v1/social/posts", body);
        await invalidate();
        toast.success("Post created");
        return res.post;
      } catch (err) {
        toast.error("Failed to create post", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate],
  );

  const updatePost = useCallback(
    async (id: string, updates: Record<string, unknown>) => {
      try {
        await http.put(`/v1/social/posts/${id}`, updates);
        await invalidate();
        await invalidatePost(id);
        toast.success("Post updated");
      } catch (err) {
        toast.error("Failed to update post", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate, invalidatePost],
  );

  const deletePost = useCallback(
    async (id: string) => {
      try {
        await http.delete(`/v1/social/posts/${id}`);
        await invalidate();
        toast.success("Post deleted");
      } catch (err) {
        toast.error("Failed to delete post", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate],
  );

  const publishPost = useCallback(
    async (id: string) => {
      try {
        const res = await http.post<{ post: SocialPost }>(`/v1/social/posts/${id}/publish`);
        await invalidate();
        await invalidatePost(id);
        toast.success("Publishing started");
        return res.post;
      } catch (err) {
        toast.error("Failed to publish", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate, invalidatePost],
  );

  const addTarget = useCallback(
    async (postId: string, accountId: string) => {
      try {
        await http.post(`/v1/social/posts/${postId}/targets`, { account_id: accountId });
        await invalidatePost(postId);
        await invalidate();
      } catch (err) {
        toast.error("Failed to add target", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate, invalidatePost],
  );

  const removeTarget = useCallback(
    async (postId: string, targetId: string) => {
      try {
        await http.delete(`/v1/social/posts/${postId}/targets/${targetId}`);
        await invalidatePost(postId);
        await invalidate();
      } catch (err) {
        toast.error("Failed to remove target", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidate, invalidatePost],
  );

  const addMedia = useCallback(
    async (postId: string, media: { url: string; media_type: string; filename?: string }) => {
      try {
        await http.post(`/v1/social/posts/${postId}/media`, media);
        await invalidatePost(postId);
      } catch (err) {
        toast.error("Failed to add media", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidatePost],
  );

  const removeMedia = useCallback(
    async (postId: string, mediaId: string) => {
      try {
        await http.delete(`/v1/social/posts/${postId}/media/${mediaId}`);
        await invalidatePost(postId);
      } catch (err) {
        toast.error("Failed to remove media", err instanceof Error ? err.message : "Unknown error");
        throw err;
      }
    },
    [http, invalidatePost],
  );

  return {
    posts,
    total,
    loading,
    refresh: invalidate,
    getPost,
    createPost,
    updatePost,
    deletePost,
    publishPost,
    addTarget,
    removeTarget,
    addMedia,
    removeMedia,
  };
}

/** Fetch a single post by ID. */
export function useSocialPost(id: string) {
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);

  const { data: post, isLoading: loading, refetch } = useQuery({
    queryKey: queryKeys.social.post(id),
    queryFn: async () => {
      const res = await http.get<{ post: SocialPost }>(`/v1/social/posts/${id}`);
      return res.post;
    },
    enabled: connected && !!id,
  });

  return { post, loading, refresh: refetch };
}
