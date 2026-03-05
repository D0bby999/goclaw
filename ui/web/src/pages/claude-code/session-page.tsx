import { useParams, useNavigate } from "react-router";
import { SessionTerminalPage } from "./session-terminal-page";
import { ROUTES } from "@/lib/constants";

export function SessionPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  if (!id) {
    navigate(ROUTES.CC_PROJECTS, { replace: true });
    return null;
  }

  return <SessionTerminalPage sessionId={id} />;
}
