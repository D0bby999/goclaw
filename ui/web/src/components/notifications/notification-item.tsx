import { CheckCheck } from "lucide-react";
import type { Notification } from "@/hooks/use-notifications";

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

const typeColors: Record<string, string> = {
  error: "bg-destructive",
  warning: "bg-yellow-500",
  success: "bg-green-500",
  info: "bg-blue-500",
};

interface NotificationItemProps {
  notification: Notification;
  onMarkRead: (id: string) => void;
}

export function NotificationItem({ notification, onMarkRead }: NotificationItemProps) {
  const dotColor = typeColors[notification.type] || "bg-blue-500";

  return (
    <div
      className={`flex items-start gap-3 px-4 py-3 border-b last:border-b-0 transition-colors ${
        notification.read
          ? "opacity-60"
          : "bg-accent/30"
      }`}
    >
      <div className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${dotColor}`} />

      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium leading-tight truncate">{notification.title}</p>
        {notification.message && (
          <p className="mt-0.5 text-xs text-muted-foreground line-clamp-2">
            {notification.message}
          </p>
        )}
        <p className="mt-1 text-[10px] text-muted-foreground">
          {timeAgo(notification.createdAt)}
        </p>
      </div>

      {!notification.read && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onMarkRead(notification.id);
          }}
          className="mt-1 shrink-0 rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
          title="Mark as read"
        >
          <CheckCheck className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  );
}
