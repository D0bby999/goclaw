import {
  Facebook,
  Instagram,
  Twitter,
  Youtube,
  Music2,
  AtSign,
  Linkedin,
  Cloud,
  type LucideIcon,
} from "lucide-react";
import type { SocialPlatform } from "@/types/social";

interface PlatformMeta {
  label: string;
  icon: LucideIcon;
  color: string;
}

export const PLATFORM_META: Record<SocialPlatform, PlatformMeta> = {
  facebook:  { label: "Facebook",  icon: Facebook,  color: "text-blue-600" },
  instagram: { label: "Instagram", icon: Instagram, color: "text-pink-500" },
  twitter:   { label: "Twitter/X", icon: Twitter,   color: "text-sky-500" },
  youtube:   { label: "YouTube",   icon: Youtube,   color: "text-red-500" },
  tiktok:    { label: "TikTok",    icon: Music2,    color: "text-foreground" },
  threads:   { label: "Threads",   icon: AtSign,    color: "text-foreground" },
  linkedin:  { label: "LinkedIn",  icon: Linkedin,  color: "text-blue-700" },
  bluesky:   { label: "Bluesky",   icon: Cloud,     color: "text-sky-400" },
};

export const ALL_PLATFORMS: SocialPlatform[] = [
  "facebook", "instagram", "twitter", "youtube",
  "tiktok", "threads", "linkedin", "bluesky",
];

export function PlatformIcon({ platform, className }: { platform: SocialPlatform; className?: string }) {
  const meta = PLATFORM_META[platform];
  if (!meta) return null;
  const Icon = meta.icon;
  return <Icon className={`${meta.color} ${className ?? "h-4 w-4"}`} />;
}
