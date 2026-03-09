// Social management types — mirrors Go store.Social*Data structs.

export type SocialPlatform =
  | "facebook"
  | "instagram"
  | "twitter"
  | "youtube"
  | "tiktok"
  | "threads"
  | "linkedin"
  | "bluesky";

export type SocialAccountStatus = "active" | "expired" | "revoked";

export type SocialPostStatus =
  | "draft"
  | "scheduled"
  | "publishing"
  | "published"
  | "partial"
  | "failed";

export type SocialTargetStatus = "pending" | "publishing" | "published" | "failed";

export interface SocialAccount {
  id: string;
  owner_id: string;
  platform: SocialPlatform;
  platform_user_id: string;
  platform_username?: string;
  display_name?: string;
  avatar_url?: string;
  token_expires_at?: string;
  scopes?: string[];
  metadata?: Record<string, unknown>;
  status: SocialAccountStatus;
  connected_at: string;
  created_at: string;
  updated_at: string;
}

export interface SocialPage {
  id: string;
  account_id: string;
  page_id: string;
  page_name?: string;
  page_type: string;
  avatar_url?: string;
  is_default: boolean;
  metadata?: Record<string, unknown>;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface SocialPostTarget {
  id: string;
  post_id: string;
  account_id: string;
  platform_post_id?: string;
  platform_url?: string;
  adapted_content?: string;
  status: SocialTargetStatus;
  error?: string;
  published_at?: string;
  created_at: string;
  platform?: SocialPlatform;
  platform_username?: string;
}

export interface SocialPostMedia {
  id: string;
  post_id: string;
  media_type: string;
  url: string;
  thumbnail_url?: string;
  filename?: string;
  mime_type?: string;
  file_size?: number;
  width?: number;
  height?: number;
  duration_seconds?: number;
  sort_order: number;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface SocialPost {
  id: string;
  owner_id: string;
  title?: string;
  content: string;
  post_type: string;
  status: SocialPostStatus;
  scheduled_at?: string;
  published_at?: string;
  post_group_id?: string;
  parent_post_id?: string;
  metadata?: Record<string, unknown>;
  error?: string;
  created_at: string;
  updated_at: string;
  targets?: SocialPostTarget[];
  media?: SocialPostMedia[];
}

export interface ContentSchedulePage {
  id: string;
  schedule_id: string;
  page_id: string;
  page_name?: string;
  page_type: string;
  platform: string;
  account_id: string;
}

export interface ContentSchedule {
  id: string;
  owner_id: string;
  name: string;
  enabled: boolean;
  content_source: "agent";
  agent_id?: string;
  prompt?: string;
  cron_expression: string;
  timezone: string;
  cron_job_id?: string;
  last_run_at?: string;
  last_status?: string;
  last_error?: string;
  posts_count: number;
  pages?: ContentSchedulePage[];
  created_at: string;
  updated_at: string;
}

export interface ContentScheduleLog {
  id: string;
  schedule_id: string;
  post_id?: string;
  status: string;
  error?: string;
  content_preview?: string;
  pages_targeted: number;
  pages_published: number;
  duration_ms?: number;
  ran_at: string;
}

export interface PlatformLimits {
  max_chars: number;
  max_hashtags: number;
  link_length?: number;
}

export interface AdaptResult {
  adapted: string;
  warnings: string[];
}
