import { useState } from "react";
import { useNavigate } from "react-router";
import { Share2, Plus, Pencil, Trash2, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/shared/empty-state";
import { Pagination } from "@/components/shared/pagination";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { usePagination } from "@/hooks/use-pagination";
import { formatDate } from "@/lib/format";
import { PostStatusBadge } from "./post-status-badge";
import { PlatformIcon } from "./platform-icons";
import type { SocialPost, SocialPlatform } from "@/types/social";

interface PostsTabProps {
  posts: SocialPost[];
  total: number;
  loading: boolean;
  onDelete: (id: string) => Promise<void>;
  onPublish: (id: string) => Promise<SocialPost>;
}

export function PostsTab({ posts, total, loading, onDelete, onPublish }: PostsTabProps) {
  const navigate = useNavigate();
  const [deleteTarget, setDeleteTarget] = useState<SocialPost | null>(null);
  const showSkeleton = useDeferredLoading(loading && posts.length === 0);
  const { pageItems, pagination, setPage, setPageSize } = usePagination(posts);

  if (showSkeleton) return <TableSkeleton rows={5} />;

  if (total === 0) {
    return (
      <EmptyState
        icon={Share2}
        title="No posts yet"
        description="Create your first social media post."
        action={
          <Button size="sm" onClick={() => navigate("/social/posts/new")} className="gap-1">
            <Plus className="h-3.5 w-3.5" /> New Post
          </Button>
        }
      />
    );
  }

  return (
    <>
      <div className="rounded-md border overflow-x-auto">
        <table className="w-full min-w-[700px] text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-4 py-3 text-left font-medium">Content</th>
              <th className="px-4 py-3 text-left font-medium">Status</th>
              <th className="px-4 py-3 text-left font-medium">Platforms</th>
              <th className="px-4 py-3 text-left font-medium">Scheduled</th>
              <th className="px-4 py-3 text-left font-medium">Created</th>
              <th className="px-4 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {pageItems.map((post) => (
              <tr
                key={post.id}
                className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                onClick={() => navigate(`/social/posts/${post.id}`)}
              >
                <td className="max-w-[250px] truncate px-4 py-3 font-medium" title={post.content}>
                  {post.title || post.content.slice(0, 80)}
                </td>
                <td className="px-4 py-3">
                  <PostStatusBadge status={post.status} />
                </td>
                <td className="px-4 py-3">
                  <div className="flex gap-1">
                    {(post.targets ?? []).map((t) => (
                      <PlatformIcon key={t.id} platform={t.platform as SocialPlatform} className="h-4 w-4" />
                    ))}
                    {(!post.targets || post.targets.length === 0) && (
                      <span className="text-muted-foreground">--</span>
                    )}
                  </div>
                </td>
                <td className="px-4 py-3 text-muted-foreground whitespace-nowrap">
                  {post.scheduled_at ? formatDate(post.scheduled_at) : "--"}
                </td>
                <td className="px-4 py-3 text-muted-foreground whitespace-nowrap">
                  {formatDate(post.created_at)}
                </td>
                <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                  <div className="flex justify-end gap-1">
                    {post.status === "draft" && (
                      <Button
                        variant="ghost"
                        size="icon"
                        title="Publish"
                        onClick={() => onPublish(post.id)}
                      >
                        <Send className="h-3.5 w-3.5" />
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      title="Edit"
                      onClick={() => navigate(`/social/posts/${post.id}/edit`)}
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      title="Delete"
                      onClick={() => setDeleteTarget(post)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <Pagination
          page={pagination.page}
          pageSize={pagination.pageSize}
          total={pagination.total}
          totalPages={pagination.totalPages}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />
      </div>

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setDeleteTarget(null)}
          title="Delete Post"
          description={`Delete this post? This cannot be undone.`}
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={async () => {
            await onDelete(deleteTarget.id);
            setDeleteTarget(null);
          }}
        />
      )}
    </>
  );
}
