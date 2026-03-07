import { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router";
import { ArrowLeft, Save, Send, Clock } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { PageHeader } from "@/components/shared/page-header";
import { DetailSkeleton } from "@/components/shared/loading-skeleton";
import { useSocialPosts, useSocialPost } from "./hooks/use-social-posts";
import { useSocialAccounts } from "./hooks/use-social-accounts";
import { TargetSelector } from "./target-selector";
import { MediaAttachments, type MediaItem } from "./media-attachments";
import { PlatformPreview } from "./platform-preview";
import type { SocialPlatform } from "@/types/social";

export function PostComposerPage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const isEdit = !!id && !window.location.pathname.endsWith("/new");

  const { createPost, updatePost, addTarget, removeTarget, addMedia, removeMedia, publishPost } = useSocialPosts();
  const { post, loading: postLoading } = useSocialPost(isEdit ? id! : "");
  const { accounts } = useSocialAccounts();

  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [selectedTargets, setSelectedTargets] = useState<Set<string>>(new Set());
  const [mediaItems, setMediaItems] = useState<MediaItem[]>([]);
  const [useSchedule, setUseSchedule] = useState(false);
  const [scheduledAt, setScheduledAt] = useState("");
  const [saving, setSaving] = useState(false);

  // Pre-fill in edit mode
  useEffect(() => {
    if (!post || !isEdit) return;
    setTitle(post.title ?? "");
    setContent(post.content);
    setSelectedTargets(new Set((post.targets ?? []).map((t) => t.account_id)));
    setMediaItems((post.media ?? []).map((m) => ({ url: m.url, media_type: m.media_type, filename: m.filename })));
    if (post.scheduled_at) {
      setUseSchedule(true);
      setScheduledAt(post.scheduled_at.slice(0, 16)); // datetime-local format
    }
  }, [post, isEdit]);

  // Platforms for preview (from selected targets)
  const previewPlatforms = Array.from(
    new Set(
      accounts
        .filter((a) => selectedTargets.has(a.id))
        .map((a) => a.platform),
    ),
  ) as SocialPlatform[];

  const toggleTarget = (accountId: string) => {
    setSelectedTargets((prev) => {
      const next = new Set(prev);
      if (next.has(accountId)) next.delete(accountId);
      else next.add(accountId);
      return next;
    });
  };

  const handleSave = async (publish: boolean) => {
    if (!content.trim()) return;
    setSaving(true);
    try {
      if (isEdit && post) {
        // Update post content
        const updates: Record<string, unknown> = { title: title || null, content };
        if (useSchedule && scheduledAt) {
          updates.scheduled_at = new Date(scheduledAt).toISOString();
          updates.status = "scheduled";
        }
        await updatePost(post.id, updates);

        // Sync targets: remove old, add new
        const existingTargetAccountIds = new Set((post.targets ?? []).map((t) => t.account_id));
        const existingTargetMap = new Map((post.targets ?? []).map((t) => [t.account_id, t.id]));
        for (const accountId of existingTargetAccountIds) {
          if (!selectedTargets.has(accountId)) {
            await removeTarget(post.id, existingTargetMap.get(accountId)!);
          }
        }
        for (const accountId of selectedTargets) {
          if (!existingTargetAccountIds.has(accountId)) {
            await addTarget(post.id, accountId);
          }
        }

        // Sync media: remove old, add new
        const existingMediaUrls = new Set((post.media ?? []).map((m) => m.url));
        const existingMediaMap = new Map((post.media ?? []).map((m) => [m.url, m.id]));
        for (const url of existingMediaUrls) {
          if (!mediaItems.some((m) => m.url === url)) {
            await removeMedia(post.id, existingMediaMap.get(url)!);
          }
        }
        for (const item of mediaItems) {
          if (!existingMediaUrls.has(item.url)) {
            await addMedia(post.id, item);
          }
        }

        if (publish) await publishPost(post.id);
        navigate(`/social/posts/${post.id}`);
      } else {
        // Create new post
        const newPost = await createPost({
          title: title || undefined,
          content,
          scheduled_at: useSchedule && scheduledAt ? new Date(scheduledAt).toISOString() : undefined,
        });
        if (newPost) {
          // Add targets and media
          for (const accountId of selectedTargets) {
            await addTarget(newPost.id, accountId);
          }
          for (const item of mediaItems) {
            await addMedia(newPost.id, item);
          }
          if (publish) {
            await publishPost(newPost.id);
          }
          navigate(`/social/posts/${newPost.id}`);
        }
      }
    } finally {
      setSaving(false);
    }
  };

  if (isEdit && postLoading) {
    return <div className="p-4 sm:p-6"><DetailSkeleton /></div>;
  }

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title={isEdit ? "Edit Post" : "New Post"}
        actions={
          <Button variant="ghost" size="sm" onClick={() => navigate("/social")} className="gap-1">
            <ArrowLeft className="h-3.5 w-3.5" /> Back
          </Button>
        }
      />

      <div className="mt-6 grid gap-6 lg:grid-cols-5">
        {/* Left column: editor */}
        <div className="space-y-4 lg:col-span-3">
          <div className="space-y-2">
            <Label>Title (optional)</Label>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Post title..." />
          </div>

          <div className="space-y-2">
            <Label>Content</Label>
            <Textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="What's on your mind?"
              rows={8}
              className="resize-y"
            />
            <div className="text-right text-xs text-muted-foreground">{content.length} chars</div>
          </div>

          <MediaAttachments items={mediaItems} onChange={setMediaItems} />

          <TargetSelector accounts={accounts} selected={selectedTargets} onToggle={toggleTarget} />

          {/* Schedule toggle */}
          <div className="space-y-2">
            <label className="flex cursor-pointer items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={useSchedule}
                onChange={(e) => setUseSchedule(e.target.checked)}
                className="h-4 w-4 rounded"
              />
              <Clock className="h-3.5 w-3.5 text-muted-foreground" />
              Schedule for later
            </label>
            {useSchedule && (
              <Input
                type="datetime-local"
                value={scheduledAt}
                onChange={(e) => setScheduledAt(e.target.value)}
                className="w-auto"
              />
            )}
          </div>

          {/* Action buttons */}
          <div className="flex gap-2 border-t pt-4">
            <Button variant="outline" onClick={() => handleSave(false)} disabled={!content.trim() || saving} className="gap-1">
              <Save className="h-3.5 w-3.5" /> {isEdit ? "Save" : "Save Draft"}
            </Button>
            {!useSchedule && (
              <Button onClick={() => handleSave(true)} disabled={!content.trim() || selectedTargets.size === 0 || saving} className="gap-1">
                <Send className="h-3.5 w-3.5" /> Publish Now
              </Button>
            )}
            {useSchedule && (
              <Button onClick={() => handleSave(false)} disabled={!content.trim() || !scheduledAt || saving} className="gap-1">
                <Clock className="h-3.5 w-3.5" /> Schedule
              </Button>
            )}
          </div>
        </div>

        {/* Right column: previews */}
        <div className="space-y-4 lg:col-span-2">
          <h3 className="text-sm font-medium">Platform Preview</h3>
          {previewPlatforms.length === 0 ? (
            <p className="text-sm text-muted-foreground">Select target accounts to see platform previews.</p>
          ) : (
            previewPlatforms.map((p) => (
              <PlatformPreview key={p} content={content} platform={p} />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
