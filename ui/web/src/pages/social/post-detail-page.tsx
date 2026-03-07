import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { ArrowLeft, Pencil, Send, Trash2, ExternalLink, Image } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { DetailSkeleton } from "@/components/shared/loading-skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useSocialPost, useSocialPosts } from "./hooks/use-social-posts";
import { PostStatusBadge, TargetStatusBadge } from "./post-status-badge";
import { PlatformIcon, PLATFORM_META } from "./platform-icons";
import { formatDate } from "@/lib/format";
import type { SocialPlatform } from "@/types/social";

export function PostDetailPage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const { post, loading, refresh } = useSocialPost(id ?? "");
  const { deletePost, publishPost } = useSocialPosts();
  const [showDelete, setShowDelete] = useState(false);

  if (loading) {
    return <div className="p-4 sm:p-6"><DetailSkeleton /></div>;
  }

  if (!post) {
    return (
      <div className="p-4 sm:p-6">
        <EmptyState title="Post not found" description="This post may have been deleted." />
      </div>
    );
  }

  const handlePublish = async () => {
    await publishPost(post.id);
    refresh();
  };

  const handleDelete = async () => {
    await deletePost(post.id);
    navigate("/social");
  };

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title={post.title || "Post"}
        actions={
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => navigate("/social")} className="gap-1">
              <ArrowLeft className="h-3.5 w-3.5" /> Back
            </Button>
            {(post.status === "draft" || post.status === "failed") && (
              <Button
                variant="outline"
                size="sm"
                onClick={handlePublish}
                disabled={!post.targets?.length}
                className="gap-1"
              >
                <Send className="h-3.5 w-3.5" /> Publish
              </Button>
            )}
            <Button variant="outline" size="sm" onClick={() => navigate(`/social/posts/${post.id}/edit`)} className="gap-1">
              <Pencil className="h-3.5 w-3.5" /> Edit
            </Button>
            <Button variant="outline" size="sm" onClick={() => setShowDelete(true)} className="gap-1 text-destructive">
              <Trash2 className="h-3.5 w-3.5" /> Delete
            </Button>
          </div>
        }
      />

      <div className="mt-6 grid gap-6 lg:grid-cols-3">
        {/* Main content */}
        <div className="space-y-4 lg:col-span-2">
          {/* Status + meta */}
          <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
            <PostStatusBadge status={post.status} />
            <span>Created {formatDate(post.created_at)}</span>
            {post.scheduled_at && <span>Scheduled {formatDate(post.scheduled_at)}</span>}
            {post.published_at && <span>Published {formatDate(post.published_at)}</span>}
          </div>

          {/* Content */}
          <div className="rounded-lg border p-4">
            <p className="whitespace-pre-wrap text-sm leading-relaxed">{post.content}</p>
          </div>

          {post.error && (
            <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
              {post.error}
            </div>
          )}

          {/* Media gallery */}
          {post.media && post.media.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-sm font-medium">Media ({post.media.length})</h3>
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {post.media.map((m) => (
                  <div key={m.id} className="group relative rounded-lg border overflow-hidden">
                    {m.media_type === "image" || m.media_type === "gif" ? (
                      <img src={m.url} alt={m.filename ?? ""} className="h-40 w-full object-cover" />
                    ) : (
                      <div className="flex h-40 items-center justify-center bg-muted">
                        <Image className="h-8 w-8 text-muted-foreground" />
                      </div>
                    )}
                    <div className="absolute bottom-0 left-0 right-0 bg-black/60 px-2 py-1 text-xs text-white">
                      {m.media_type}{m.filename ? ` - ${m.filename}` : ""}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Sidebar: Publishing status per target */}
        <div className="space-y-4">
          <h3 className="text-sm font-medium">Publishing Status</h3>
          {(!post.targets || post.targets.length === 0) ? (
            <p className="text-sm text-muted-foreground">No target accounts selected.</p>
          ) : (
            <div className="space-y-3">
              {post.targets.map((t) => (
                <div key={t.id} className="rounded-lg border p-3 space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <PlatformIcon platform={(t.platform as SocialPlatform) ?? "twitter"} className="h-4 w-4" />
                      <span className="text-sm font-medium">
                        {PLATFORM_META[t.platform as SocialPlatform]?.label ?? t.platform}
                      </span>
                    </div>
                    <TargetStatusBadge status={t.status} />
                  </div>

                  {t.platform_username && (
                    <p className="text-xs text-muted-foreground">@{t.platform_username}</p>
                  )}

                  {t.error && (
                    <p className="text-xs text-red-500">{t.error}</p>
                  )}

                  {t.platform_url && (
                    <a
                      href={t.platform_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                    >
                      <ExternalLink className="h-3 w-3" /> View on platform
                    </a>
                  )}

                  {t.published_at && (
                    <p className="text-xs text-muted-foreground">Published {formatDate(t.published_at)}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {showDelete && (
        <ConfirmDialog
          open
          onOpenChange={() => setShowDelete(false)}
          title="Delete Post"
          description="Delete this post? This cannot be undone."
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={handleDelete}
        />
      )}
    </div>
  );
}
