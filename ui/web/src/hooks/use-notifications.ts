import { useState, useEffect, useCallback } from "react";
import { useWs } from "./use-ws";
import { useWsEvent } from "./use-ws-event";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods, Events } from "@/api/protocol";

export interface Notification {
  id: string;
  userId: string;
  agentId?: string;
  type: string;
  title: string;
  message: string;
  metadata?: Record<string, unknown>;
  read: boolean;
  createdAt: string;
}

export function useNotifications() {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);

  const fetchNotifications = useCallback(async () => {
    if (!connected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ notifications: Notification[] }>(
        Methods.NOTIFICATIONS_LIST,
        { limit: 50 },
      );
      setNotifications(res.notifications ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws, connected]);

  const fetchUnreadCount = useCallback(async () => {
    if (!connected) return;
    try {
      const res = await ws.call<{ count: number }>(
        Methods.NOTIFICATIONS_UNREAD,
      );
      setUnreadCount(res.count ?? 0);
    } catch {
      // ignore
    }
  }, [ws, connected]);

  const markRead = useCallback(
    async (id: string) => {
      try {
        await ws.call(Methods.NOTIFICATIONS_MARK_READ, { id });
        setNotifications((prev) =>
          prev.map((n) => (n.id === id ? { ...n, read: true } : n)),
        );
        setUnreadCount((c) => Math.max(0, c - 1));
      } catch {
        // ignore
      }
    },
    [ws],
  );

  const markAllRead = useCallback(async () => {
    try {
      await ws.call(Methods.NOTIFICATIONS_MARK_ALL_READ);
      setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
      setUnreadCount(0);
    } catch {
      // ignore
    }
  }, [ws]);

  // Initial fetch
  useEffect(() => {
    fetchNotifications();
    fetchUnreadCount();
  }, [fetchNotifications, fetchUnreadCount]);

  // Refresh on new notification events
  useWsEvent(
    Events.NOTIFICATION,
    useCallback(() => {
      fetchNotifications();
      fetchUnreadCount();
    }, [fetchNotifications, fetchUnreadCount]),
  );

  return {
    notifications,
    unreadCount,
    loading,
    markRead,
    markAllRead,
    refresh: fetchNotifications,
  };
}
