import {
  LayoutDashboard,
  MessageSquare,
  Bot,
  History,
  Zap,
  Clock,
  Activity,
  Radio,
  Radar,
  Terminal,
  Settings,
  ShieldCheck,
  Users,
  Link,
  Package,
  Plug,
  Volume2,
  Cpu,
  Newspaper,
  Share2,
  ClipboardList,
  HardDrive,
  Inbox,
  Brain,
  BookOpen,
  Wrench,
  Network,
  KeyRound,
  FileText,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { SidebarGroup } from "./sidebar-group";
import { SidebarItem } from "./sidebar-item";
import { ConnectionStatus } from "./connection-status";
import { ROUTES } from "@/lib/constants";
import { cn } from "@/lib/utils";
import { usePendingPairingsCount } from "@/hooks/use-pending-pairings-count";

interface SidebarProps {
  collapsed: boolean;
  onNavItemClick?: () => void;
}

export function Sidebar({ collapsed, onNavItemClick }: SidebarProps) {
  const { t } = useTranslation("sidebar");
  const { pendingCount } = usePendingPairingsCount();

  return (
    <aside
      className={cn(
        "flex h-full flex-col border-r bg-sidebar text-sidebar-foreground transition-all duration-200",
        collapsed ? "w-16" : "w-64",
      )}
      onClick={(e) => {
        // Close mobile drawer when clicking a nav link
        if (onNavItemClick && (e.target as HTMLElement).closest("a")) {
          onNavItemClick();
        }
      }}
    >
      {/* Logo / title */}
      <div className="flex h-14 items-center border-b px-4">
        {!collapsed && (
          <span className="text-base font-semibold tracking-tight">
            GoClaw
          </span>
        )}
        {collapsed && (
          <span className="mx-auto text-lg font-bold">OC</span>
        )}
      </div>

      {/* Nav items */}
      <nav className="flex-1 space-y-4 overflow-y-auto px-2 py-4">
        <SidebarGroup label={t("groups.core")} collapsed={collapsed}>
          <SidebarItem to={ROUTES.OVERVIEW} icon={LayoutDashboard} label={t("nav.overview")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.CHAT} icon={MessageSquare} label={t("nav.chat")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.AGENTS} icon={Bot} label={t("nav.agents")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.TEAMS} icon={Users} label={t("nav.agentTeams")} collapsed={collapsed} />
        </SidebarGroup>

        <SidebarGroup label="Management" collapsed={collapsed}>
          <SidebarItem to={ROUTES.AGENTS} icon={Bot} label="Agents" collapsed={collapsed} />
          <SidebarItem to={ROUTES.TEAMS} icon={Users} label="Agent Teams" collapsed={collapsed} />
          <SidebarItem to={ROUTES.PROJECTS} icon={Terminal} label="Projects" collapsed={collapsed} />
          <SidebarItem to={ROUTES.SESSIONS} icon={History} label="Sessions" collapsed={collapsed} />
          <SidebarItem to={ROUTES.PENDING_MESSAGES} icon={Inbox} label="Pending Messages" collapsed={collapsed} />
          <SidebarItem to={ROUTES.CHANNELS} icon={Radio} label="Channels" collapsed={collapsed} />
          <SidebarItem to={ROUTES.SKILLS} icon={Zap} label="Skills" collapsed={collapsed} />
          <SidebarItem to={ROUTES.CRON} icon={Clock} label="Cron" collapsed={collapsed} />
          <SidebarItem to={ROUTES.NEWS_DIGEST} icon={Newspaper} label="News Digest" collapsed={collapsed} />
          <SidebarItem to={ROUTES.SOCIAL} icon={Share2} label="Social" collapsed={collapsed} />
          <SidebarItem to={ROUTES.CUSTOM_TOOLS} icon={Wrench} label="Custom Tools" collapsed={collapsed} />
          <SidebarItem to={ROUTES.BUILTIN_TOOLS} icon={Package} label="Built-in Tools" collapsed={collapsed} />
          <SidebarItem to={ROUTES.MCP} icon={Plug} label="MCP Servers" collapsed={collapsed} />
          <SidebarItem to={ROUTES.MEMORY} icon={Brain} label="Memory" collapsed={collapsed} />
          <SidebarItem to={ROUTES.KNOWLEDGE_BASE} icon={BookOpen} label="Knowledge Base" collapsed={collapsed} />
        </SidebarGroup>

        <SidebarGroup label={t("groups.connectivity")} collapsed={collapsed}>
          <SidebarItem to={ROUTES.CHANNELS} icon={Radio} label={t("nav.channels")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.NODES} icon={Link} label={t("nav.nodes")} collapsed={collapsed} badge={pendingCount} />
        </SidebarGroup>

        <SidebarGroup label={t("groups.capabilities")} collapsed={collapsed}>
          <SidebarItem to={ROUTES.SKILLS} icon={Zap} label={t("nav.skills")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.BUILTIN_TOOLS} icon={Package} label={t("nav.builtinTools")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.MCP} icon={Plug} label={t("nav.mcpServers")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.TTS} icon={Volume2} label={t("nav.tts")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.CRON} icon={Clock} label={t("nav.cron")} collapsed={collapsed} />
        </SidebarGroup>

        <SidebarGroup label={t("groups.data")} collapsed={collapsed}>
          <SidebarItem to={ROUTES.MEMORY} icon={Brain} label={t("nav.memory")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.KNOWLEDGE_GRAPH} icon={Network} label={t("nav.knowledgeGraph")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.STORAGE} icon={HardDrive} label={t("nav.storage")} collapsed={collapsed} />
        </SidebarGroup>

        <SidebarGroup label={t("groups.monitoring")} collapsed={collapsed}>
          <SidebarItem to={ROUTES.TRACES} icon={Activity} label={t("nav.traces")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.EVENTS} icon={Radar} label={t("nav.realtimeEvents")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.ACTIVITY} icon={ClipboardList} label={t("nav.activity")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.LOGS} icon={Terminal} label={t("nav.logs")} collapsed={collapsed} />
        </SidebarGroup>

        <SidebarGroup label={t("groups.system")} collapsed={collapsed}>
          <SidebarItem to={ROUTES.PROVIDERS} icon={Cpu} label={t("nav.providers")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.CLI_CREDENTIALS} icon={KeyRound} label={t("nav.cliCredentials")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.API_KEYS} icon={KeyRound} label={t("nav.apiKeys")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.CONFIG} icon={Settings} label={t("nav.config")} collapsed={collapsed} />
          <SidebarItem to={ROUTES.APPROVALS} icon={ShieldCheck} label={t("nav.approvals")} collapsed={collapsed} />
          <SidebarItem to="/docs" icon={FileText} label={t("nav.apiDocs")} collapsed={collapsed} external />
        </SidebarGroup>
      </nav>

      {/* Footer: connection status */}
      <div className={cn("border-t py-3", collapsed ? "px-2 flex justify-center" : "px-4")}>
        <ConnectionStatus collapsed={collapsed} />
      </div>
    </aside>
  );
}
