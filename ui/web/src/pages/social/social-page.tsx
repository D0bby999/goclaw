import { useState } from "react";
import { useNavigate } from "react-router";
import { Plus, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { useSocialAccounts } from "./hooks/use-social-accounts";
import { useSocialPosts } from "./hooks/use-social-posts";
import { useMinLoading } from "@/hooks/use-min-loading";
import { PostsTab } from "./posts-tab";
import { AccountsTab } from "./accounts-tab";

type Tab = "posts" | "accounts";

export function SocialPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>("posts");

  const { accounts, loading: accountsLoading, refresh: refreshAccounts, createAccount, updateAccount, deleteAccount } = useSocialAccounts();
  const { posts, total, loading: postsLoading, refresh: refreshPosts, deletePost, publishPost } = useSocialPosts();

  const loading = tab === "posts" ? postsLoading : accountsLoading;
  const spinning = useMinLoading(loading);

  const refresh = () => {
    refreshAccounts();
    refreshPosts();
  };

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title="Social"
        description="Manage social accounts and publish content across platforms"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={"h-3.5 w-3.5" + (spinning ? " animate-spin" : "")} /> Refresh
            </Button>
            {tab === "posts" && (
              <Button size="sm" onClick={() => navigate("/social/posts/new")} className="gap-1">
                <Plus className="h-3.5 w-3.5" /> New Post
              </Button>
            )}
          </div>
        }
      />

      {/* Tabs */}
      <div className="mt-4 flex gap-1 border-b">
        {(["posts", "accounts"] as const).map((t) => (
          <button
            key={t}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
            onClick={() => setTab(t)}
          >
            {t === "posts" ? `Posts (${total})` : `Accounts (${accounts.length})`}
          </button>
        ))}
      </div>

      <div className="mt-4">
        {tab === "posts" ? (
          <PostsTab
            posts={posts}
            total={total}
            loading={postsLoading}
            onDelete={deletePost}
            onPublish={publishPost}
          />
        ) : (
          <AccountsTab
            accounts={accounts}
            loading={accountsLoading}
            onCreate={createAccount}
            onUpdate={updateAccount}
            onDelete={deleteAccount}
          />
        )}
      </div>
    </div>
  );
}
