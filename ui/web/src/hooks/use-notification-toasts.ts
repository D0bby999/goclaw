import { useCallback } from "react";
import { useWsEvent } from "./use-ws-event";
import { Events } from "@/api/protocol";
import { toast } from "@/stores/use-toast-store";

interface CronEventPayload {
  type?: string;
  name?: string;
  status?: string;
  error?: string;
}

interface NotificationPayload {
  type?: string;
  title?: string;
  message?: string;
}

interface ApprovalPayload {
  command?: string;
  agent?: string;
}

/**
 * Listens to WS events and shows user-facing toast notifications.
 * Mount once at app level (alongside useWsQueryInvalidation).
 */
export function useNotificationToasts() {
  // Cron job completion → toast
  const handleCron = useCallback((raw: unknown) => {
    const payload = raw as CronEventPayload;
    if (payload.type === "completed") {
      toast.success("Cron completed", payload.name || "Job finished");
    } else if (payload.type === "failed") {
      toast.error("Cron failed", payload.error || payload.name || "Job failed");
    }
  }, []);

  // Exec approval requests → toast
  const handleApprovalReq = useCallback((raw: unknown) => {
    const payload = raw as ApprovalPayload;
    toast.warning(
      "Approval needed",
      payload.command
        ? `Command: ${payload.command}`
        : "An action requires your approval",
    );
  }, []);

  // Server-sent notification events → toast
  const handleNotification = useCallback((raw: unknown) => {
    const payload = raw as NotificationPayload;
    const title = payload.title || "Notification";
    const message = payload.message;
    switch (payload.type) {
      case "error":
        toast.error(title, message);
        break;
      case "warning":
        toast.warning(title, message);
        break;
      case "success":
        toast.success(title, message);
        break;
      default:
        toast.info(title, message);
    }
  }, []);

  useWsEvent(Events.CRON, handleCron);
  useWsEvent(Events.EXEC_APPROVAL_REQUESTED, handleApprovalReq);
  useWsEvent(Events.NOTIFICATION, handleNotification);
}
