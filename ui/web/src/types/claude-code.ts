export interface CCProject {
  id: string;
  name: string;
  slug: string;
  work_dir: string;
  description?: string;
  allowed_tools?: string[];
  claude_config?: Record<string, unknown>;
  max_sessions: number;
  owner_id: string;
  team_id?: string;
  status: "active" | "archived";
  created_at?: string;
  updated_at?: string;
}

export interface CCSession {
  id: string;
  project_id: string;
  claude_session_id?: string;
  label?: string;
  status: "starting" | "running" | "stopped" | "failed" | "completed";
  pid?: number;
  started_by: string;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
  error?: string;
  started_at: string;
  stopped_at?: string;
  project_name?: string;
  project_slug?: string;
}

export interface CCSessionLog {
  id: string;
  session_id: string;
  event_type: string;
  content: Record<string, unknown>;
  seq: number;
  created_at: string;
}

export interface StreamEvent {
  type: string;
  subtype?: string;
  session_id?: string;
  raw: Record<string, unknown>;
  input_tokens?: number;
  output_tokens?: number;
  cost_usd?: number;
  /** Parsed message content for assistant events */
  message?: { content?: Array<{ type: string; text?: string; name?: string; input?: unknown }> };
}

export type AgentStatus =
  | "writing"
  | "reading"
  | "editing"
  | "running_cmd"
  | "searching"
  | "thinking"
  | "completed"
  | "failed"
  | "idle";
