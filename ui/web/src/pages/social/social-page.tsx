import { useState } from "react";
import { useNavigate } from "react-router";
import { Plus, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { useSocialAccounts } from "./hooks/use-social-accounts";
import { useSocialPosts } from "./hooks/use-social-posts";
import { useAllSocialPages } from "./hooks/use-all-social-pages";
import { useContentSchedules } from "./hooks/use-content-schedules";
import { useMinLoading } from "@/hooks/use-min-loading";
import { PostsTab } from "./posts-tab";
import { AccountsTab } from "./accounts-tab";
import { PagesTab } from "./pages-tab";
import { SchedulesTab } from "./schedules-tab";

type Tab = "posts" | "accounts" | "pages" | "schedules";

export function SocialPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>("posts");

  const { accounts, loading: accountsLoading, refresh: refreshAccounts, createAccount, updateAccount, deleteAccount } = useSocialAccounts();
  const { posts, total, loading: postsLoading, refresh: refreshPosts, deletePost, publishPost } = useSocialPosts();
  const { pages, loading: pagesLoading, syncAll, setDefault, deletePage, createPage } = useAllSocialPages(accounts);
  const { schedules, loading: schedulesLoading } = useContentSchedules();

  const loading = tab === "posts" ? postsLoading : tab === "pages" ? pagesLoading : tab === "schedules" ? schedulesLoading : accountsLoading;
  const spinning = useMinLoading(loading);

  const refresh = () => {
    refreshAccounts();
    refreshPosts();
  };

  const tabLabel = (t: Tab) => {
    if (t === "posts") return `Posts (${total})`;
    if (t === "accounts") return `Accounts (${accounts.length})`;
    if (t === "pages") return `Pages (${pages.length})`;
    return `Schedules (${schedules.length})`;
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
        {(["posts", "accounts", "pages", "schedules"] as const).map((t) => (
          <button
            key={t}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
            onClick={() => setTab(t)}
          >
            {tabLabel(t)}
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
        ) : tab === "accounts" ? (
          <AccountsTab
            accounts={accounts}
            loading={accountsLoading}
            onCreate={createAccount}
            onUpdate={updateAccount}
            onDelete={deleteAccount}
            onRefresh={refreshAccounts}
          />
        ) : tab === "pages" ? (
          <PagesTab
            pages={pages}
            accounts={accounts}
            loading={pagesLoading}
            onSyncAll={syncAll}
            onSetDefault={setDefault}
            onDelete={deletePage}
            onCreate={createPage}
          />
        ) : (
          <SchedulesTab />
        )}
      </div>
    </div>
  );
}
