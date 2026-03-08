import { useState } from "react";
import { Share2, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/shared/empty-state";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { CardSkeleton } from "@/components/shared/loading-skeleton";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { AccountCard } from "./account-card";
import { AccountConnectDialog } from "./account-connect-dialog";
import type { SocialAccount } from "@/types/social";

interface AccountsTabProps {
  accounts: SocialAccount[];
  loading: boolean;
  onCreate: (params: {
    platform: string;
    platform_user_id: string;
    access_token: string;
    platform_username?: string;
    display_name?: string;
  }) => Promise<void>;
  onUpdate: (id: string, updates: Record<string, unknown>) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
  onRefresh?: () => void;
}

export function AccountsTab({ accounts, loading, onCreate, onUpdate, onDelete, onRefresh }: AccountsTabProps) {
  const [showForm, setShowForm] = useState(false);
  const [editAccount, setEditAccount] = useState<SocialAccount | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SocialAccount | null>(null);
  const showSkeleton = useDeferredLoading(loading && accounts.length === 0);

  if (showSkeleton) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => <CardSkeleton key={i} />)}
      </div>
    );
  }

  if (accounts.length === 0) {
    return (
      <>
        <EmptyState
          icon={Share2}
          title="No accounts connected"
          description="Connect a social media account to start publishing."
          action={
            <Button size="sm" onClick={() => { setEditAccount(null); setShowForm(true); }} className="gap-1">
              <Plus className="h-3.5 w-3.5" /> Connect Account
            </Button>
          }
        />
        <AccountConnectDialog
          open={showForm}
          onOpenChange={setShowForm}
          onSubmit={onCreate}
          onUpdate={onUpdate}
          editAccount={editAccount}
          onOAuthComplete={onRefresh}
        />
      </>
    );
  }

  return (
    <>
      <div className="mb-4 flex justify-end">
        <Button size="sm" onClick={() => { setEditAccount(null); setShowForm(true); }} className="gap-1">
          <Plus className="h-3.5 w-3.5" /> Connect Account
        </Button>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {accounts.map((a) => (
          <AccountCard
            key={a.id}
            account={a}
            onEdit={(acc) => { setEditAccount(acc); setShowForm(true); }}
            onDelete={setDeleteTarget}
          />
        ))}
      </div>

      <AccountConnectDialog
        open={showForm}
        onOpenChange={setShowForm}
        onSubmit={onCreate}
        onUpdate={onUpdate}
        editAccount={editAccount}
      />

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setDeleteTarget(null)}
          title="Disconnect Account"
          description={`Disconnect "${deleteTarget.display_name || deleteTarget.platform_username || deleteTarget.platform}"? This won't delete published posts.`}
          confirmLabel="Disconnect"
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
