// Wire format types matching Go pkg/protocol/ exactly.

export const PROTOCOL_VERSION = 3;

// --- Frame types ---

export interface RequestFrame {
  type: "req";
  id: string;
  method: string;
  params?: Record<string, unknown>;
}

export interface ResponseFrame {
  type: "res";
  id: string;
  ok: boolean;
  payload?: unknown;
  error?: ErrorShape;
}

export interface EventFrame {
  type: "event";
  event: string;
  payload?: unknown;
  seq?: number;
  stateVersion?: { presence: number; health: number };
}

export interface ErrorShape {
  code: string;
  message: string;
  details?: unknown;
  retryable?: boolean;
  retryAfterMs?: number;
}

// --- RPC method names (from pkg/protocol/methods.go) ---

// Phase 1 - CRITICAL
export const Methods = {
  // System
  CONNECT: "connect",
  HEALTH: "health",
  STATUS: "status",

  // Agent
  AGENT: "agent",
  AGENT_WAIT: "agent.wait",
  AGENT_IDENTITY_GET: "agent.identity.get",

  // Chat
  CHAT_SEND: "chat.send",
  CHAT_HISTORY: "chat.history",
  CHAT_ABORT: "chat.abort",
  CHAT_INJECT: "chat.inject",

  // Agents management
  AGENTS_LIST: "agents.list",
  AGENTS_CREATE: "agents.create",
  AGENTS_UPDATE: "agents.update",
  AGENTS_DELETE: "agents.delete",
  AGENTS_FILES_LIST: "agents.files.list",
  AGENTS_FILES_GET: "agents.files.get",
  AGENTS_FILES_SET: "agents.files.set",

  // Config
  CONFIG_GET: "config.get",
  CONFIG_APPLY: "config.apply",
  CONFIG_PATCH: "config.patch",
  CONFIG_SCHEMA: "config.schema",

  // Sessions
  SESSIONS_LIST: "sessions.list",
  SESSIONS_PREVIEW: "sessions.preview",
  SESSIONS_PATCH: "sessions.patch",
  SESSIONS_DELETE: "sessions.delete",
  SESSIONS_RESET: "sessions.reset",

  // Phase 2 - NEEDED
  SKILLS_LIST: "skills.list",
  SKILLS_GET: "skills.get",
  SKILLS_UPDATE: "skills.update",

  CRON_LIST: "cron.list",
  CRON_CREATE: "cron.create",
  CRON_UPDATE: "cron.update",
  CRON_DELETE: "cron.delete",
  CRON_TOGGLE: "cron.toggle",
  CRON_STATUS: "cron.status",
  CRON_RUN: "cron.run",
  CRON_RUNS: "cron.runs",

  CHANNELS_LIST: "channels.list",
  CHANNELS_STATUS: "channels.status",
  CHANNELS_TOGGLE: "channels.toggle",

  // Channel instances
  CHANNEL_INSTANCES_LIST: "channels.instances.list",
  CHANNEL_INSTANCES_CREATE: "channels.instances.create",
  CHANNEL_INSTANCES_UPDATE: "channels.instances.update",
  CHANNEL_INSTANCES_DELETE: "channels.instances.delete",

  PAIRING_REQUEST: "device.pair.request",
  PAIRING_APPROVE: "device.pair.approve",
  PAIRING_DENY: "device.pair.deny",
  PAIRING_LIST: "device.pair.list",
  PAIRING_REVOKE: "device.pair.revoke",

  BROWSER_PAIRING_STATUS: "browser.pairing.status",

  APPROVALS_LIST: "exec.approval.list",
  APPROVALS_APPROVE: "exec.approval.approve",
  APPROVALS_DENY: "exec.approval.deny",

  USAGE_GET: "usage.get",
  USAGE_SUMMARY: "usage.summary",

  QUOTA_USAGE: "quota.usage",

  SEND: "send",

  // Agent links (delegation)
  AGENTS_LINKS_LIST: "agents.links.list",
  AGENTS_LINKS_CREATE: "agents.links.create",
  AGENTS_LINKS_UPDATE: "agents.links.update",
  AGENTS_LINKS_DELETE: "agents.links.delete",

  // Agent teams
  TEAMS_LIST: "teams.list",
  TEAMS_CREATE: "teams.create",
  TEAMS_GET: "teams.get",
  TEAMS_DELETE: "teams.delete",
  TEAMS_TASK_LIST: "teams.tasks.list",
  TEAMS_MEMBERS_ADD: "teams.members.add",
  TEAMS_MEMBERS_REMOVE: "teams.members.remove",
  TEAMS_UPDATE: "teams.update",
  TEAMS_KNOWN_USERS: "teams.known_users",

  // Delegation history
  DELEGATIONS_LIST: "delegations.list",
  DELEGATIONS_GET: "delegations.get",

  // Scraper cookies
  SCRAPER_COOKIES_LIST: "scraper.cookies.list",
  SCRAPER_COOKIES_SET: "scraper.cookies.set",
  SCRAPER_COOKIES_DELETE: "scraper.cookies.delete",
  SCRAPER_COOKIES_LOGIN: "scraper.cookies.login",

  // News digest
  NEWS_SOURCES_LIST: "news.sources.list",
  NEWS_SOURCES_CREATE: "news.sources.create",
  NEWS_SOURCES_UPDATE: "news.sources.update",
  NEWS_SOURCES_DELETE: "news.sources.delete",
  NEWS_ITEMS_LIST: "news.items.list",
  NEWS_ITEMS_STATS: "news.items.stats",

  // Social management
  SOCIAL_ACCOUNTS_LIST: "social.accounts.list",
  SOCIAL_ACCOUNTS_CREATE: "social.accounts.create",
  SOCIAL_ACCOUNTS_UPDATE: "social.accounts.update",
  SOCIAL_ACCOUNTS_DELETE: "social.accounts.delete",
  SOCIAL_POSTS_LIST: "social.posts.list",
  SOCIAL_POSTS_CREATE: "social.posts.create",
  SOCIAL_POSTS_GET: "social.posts.get",
  SOCIAL_POSTS_UPDATE: "social.posts.update",
  SOCIAL_POSTS_DELETE: "social.posts.delete",
  SOCIAL_POSTS_PUBLISH: "social.posts.publish",
  SOCIAL_TARGETS_ADD: "social.posts.targets.add",
  SOCIAL_TARGETS_REMOVE: "social.posts.targets.remove",
  SOCIAL_MEDIA_ADD: "social.posts.media.add",
  SOCIAL_MEDIA_REMOVE: "social.posts.media.remove",

  // Phase 3+ - NICE TO HAVE
  LOGS_TAIL: "logs.tail",
  HEARTBEAT: "heartbeat",

  // Projects orchestration
  PROJECTS_LIST: "projects.list",
  PROJECTS_CREATE: "projects.create",
  PROJECTS_GET: "projects.get",
  PROJECTS_UPDATE: "projects.update",
  PROJECTS_DELETE: "projects.delete",
  PROJECT_SESSIONS_LIST: "projects.sessions.list",
  PROJECT_SESSIONS_START: "projects.sessions.start",
  PROJECT_SESSIONS_GET: "projects.sessions.get",
  PROJECT_SESSIONS_PROMPT: "projects.sessions.prompt",
  PROJECT_SESSIONS_STOP: "projects.sessions.stop",
  PROJECT_SESSIONS_DELETE: "projects.sessions.delete",
  PROJECT_SESSIONS_UPDATE: "projects.sessions.update",
  PROJECT_SESSIONS_LOGS: "projects.sessions.logs",
} as const;

// --- Event names (from pkg/protocol/events.go) ---

export const Events = {
  AGENT: "agent",
  CHAT: "chat",
  HEALTH: "health",
  CRON: "cron",
  EXEC_APPROVAL_REQUESTED: "exec.approval.requested",
  EXEC_APPROVAL_RESOLVED: "exec.approval.resolved",
  PRESENCE: "presence",
  TICK: "tick",
  SHUTDOWN: "shutdown",
  NODE_PAIR_REQUESTED: "node.pair.requested",
  NODE_PAIR_RESOLVED: "node.pair.resolved",
  DEVICE_PAIR_REQUESTED: "device.pair.requested",
  DEVICE_PAIR_RESOLVED: "device.pair.resolved",
  VOICEWAKE_CHANGED: "voicewake.changed",
  CONNECT_CHALLENGE: "connect.challenge",
  TALK_MODE: "talk.mode",
  HANDOFF: "handoff",

  // Delegation lifecycle
  DELEGATION_STARTED: "delegation.started",
  DELEGATION_COMPLETED: "delegation.completed",
  DELEGATION_FAILED: "delegation.failed",
  DELEGATION_CANCELLED: "delegation.cancelled",
  DELEGATION_PROGRESS: "delegation.progress",
  DELEGATION_ACCUMULATED: "delegation.accumulated",
  DELEGATION_ANNOUNCE: "delegation.announce",
  DELEGATION_QUALITY_GATE_RETRY: "delegation.quality_gate.retry",

  // Team tasks
  TEAM_TASK_CREATED: "team.task.created",
  TEAM_TASK_CLAIMED: "team.task.claimed",
  TEAM_TASK_COMPLETED: "team.task.completed",
  TEAM_TASK_CANCELLED: "team.task.cancelled",

  // Team messages
  TEAM_MESSAGE_SENT: "team.message.sent",

  // Team CRUD
  TEAM_CREATED: "team.created",
  TEAM_UPDATED: "team.updated",
  TEAM_DELETED: "team.deleted",
  TEAM_MEMBER_ADDED: "team.member.added",
  TEAM_MEMBER_REMOVED: "team.member.removed",

  // Agent links
  AGENT_LINK_CREATED: "agent_link.created",
  AGENT_LINK_UPDATED: "agent_link.updated",
  AGENT_LINK_DELETED: "agent_link.deleted",

  // Projects orchestration events
  PROJECT_OUTPUT: "project.output",
  PROJECT_SESSION_STATUS: "project.session.status",
} as const;

/** All event names relevant to team debug view */
export const TEAM_RELATED_EVENTS: Set<string> = new Set([
  Events.DELEGATION_STARTED, Events.DELEGATION_COMPLETED,
  Events.DELEGATION_FAILED, Events.DELEGATION_CANCELLED,
  Events.DELEGATION_PROGRESS, Events.DELEGATION_ACCUMULATED,
  Events.DELEGATION_ANNOUNCE, Events.DELEGATION_QUALITY_GATE_RETRY,
  Events.TEAM_TASK_CREATED, Events.TEAM_TASK_CLAIMED,
  Events.TEAM_TASK_COMPLETED, Events.TEAM_TASK_CANCELLED,
  Events.TEAM_MESSAGE_SENT,
  Events.TEAM_CREATED, Events.TEAM_UPDATED, Events.TEAM_DELETED,
  Events.TEAM_MEMBER_ADDED, Events.TEAM_MEMBER_REMOVED,
  Events.AGENT_LINK_CREATED, Events.AGENT_LINK_UPDATED,
  Events.AGENT_LINK_DELETED,
  Events.AGENT,
]);

// Agent event subtypes (in payload.type)
export const AgentEventTypes = {
  RUN_STARTED: "run.started",
  RUN_COMPLETED: "run.completed",
  RUN_FAILED: "run.failed",
  TOOL_CALL: "tool.call",
  TOOL_RESULT: "tool.result",
  BLOCK_REPLY: "block.reply",
} as const;

// Chat event subtypes (in payload.type)
export const ChatEventTypes = {
  CHUNK: "chunk",
  MESSAGE: "message",
  THINKING: "thinking",
} as const;
