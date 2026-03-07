import { StatusBadge } from "@/components/shared/status-badge";
import type { SocialPostStatus, SocialTargetStatus } from "@/types/social";

const POST_STATUS_MAP: Record<SocialPostStatus, { status: "success" | "warning" | "error" | "info" | "default"; label: string }> = {
  draft:      { status: "default", label: "Draft" },
  scheduled:  { status: "info",    label: "Scheduled" },
  publishing: { status: "warning", label: "Publishing" },
  published:  { status: "success", label: "Published" },
  partial:    { status: "warning", label: "Partial" },
  failed:     { status: "error",   label: "Failed" },
};

const TARGET_STATUS_MAP: Record<SocialTargetStatus, { status: "success" | "warning" | "error" | "info" | "default"; label: string }> = {
  pending:    { status: "default", label: "Pending" },
  publishing: { status: "warning", label: "Publishing" },
  published:  { status: "success", label: "Published" },
  failed:     { status: "error",   label: "Failed" },
};

export function PostStatusBadge({ status }: { status: SocialPostStatus }) {
  const mapped = POST_STATUS_MAP[status] ?? { status: "default" as const, label: status };
  return <StatusBadge status={mapped.status} label={mapped.label} />;
}

export function TargetStatusBadge({ status }: { status: SocialTargetStatus }) {
  const mapped = TARGET_STATUS_MAP[status] ?? { status: "default" as const, label: status };
  return <StatusBadge status={mapped.status} label={mapped.label} />;
}
