package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/cache"
	ccpkg "github.com/nextlevelbuilder/goclaw/internal/claudecode"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	httpapi "github.com/nextlevelbuilder/goclaw/internal/http"
	"github.com/nextlevelbuilder/goclaw/internal/kb"
	"github.com/nextlevelbuilder/goclaw/internal/channels/discord"
	"github.com/nextlevelbuilder/goclaw/internal/channels/feishu"
	slackchannel "github.com/nextlevelbuilder/goclaw/internal/channels/slack"
	"github.com/nextlevelbuilder/goclaw/internal/channels/telegram"
	"github.com/nextlevelbuilder/goclaw/internal/channels/whatsapp"
	"github.com/nextlevelbuilder/goclaw/internal/channels/zalo"
	zalopersonal "github.com/nextlevelbuilder/goclaw/internal/channels/zalo/personal"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/gateway/methods"
	mcpbridge "github.com/nextlevelbuilder/goclaw/internal/mcp"
	"github.com/nextlevelbuilder/goclaw/internal/media"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/scraper"
	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/scheduler"
	"github.com/nextlevelbuilder/goclaw/internal/skills"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pg"
	"github.com/nextlevelbuilder/goclaw/internal/tasks"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func runGateway() {
	// Setup structured logging
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	logTee := gateway.NewLogTee(textHandler)
	slog.SetDefault(slog.New(logTee))

	// Load config
	cfgPath := resolveConfigPath()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create core components
	msgBus := bus.New()

	// Create provider registry
	providerRegistry := providers.NewRegistry()
	registerProviders(providerRegistry, cfg)

	// Resolve workspace (must be absolute for system prompt + file tool path resolution)
	workspace := config.ExpandHome(cfg.Agents.Defaults.Workspace)
	if !filepath.IsAbs(workspace) {
		workspace, _ = filepath.Abs(workspace)
	}
	os.MkdirAll(workspace, 0755)

	// Bootstrap files live in Postgres.

	// Detect server IPs for output scrubbing (prevents IP leaks via web_fetch, exec, etc.)
	tools.DetectServerIPs(context.Background())

	toolsReg, execApprovalMgr, mcpMgr, sandboxMgr, browserMgr, webFetchTool, permPE, toolPE, dataDir, agentCfg := setupToolRegistry(cfg, workspace, providerRegistry)
	if browserMgr != nil {
		defer browserMgr.Close()
	}
	if mcpMgr != nil {
		defer mcpMgr.Stop()
	}

	pgStores, traceCollector, snapshotWorker := setupStoresAndTracing(cfg, dataDir, msgBus)
	if traceCollector != nil {
		defer traceCollector.Stop()
		// OTel OTLP export: compiled via build tags. Build with 'go build -tags otel' to enable.
		initOTelExporter(context.Background(), cfg, traceCollector)
	}
	if snapshotWorker != nil {
		defer snapshotWorker.Stop()
	}

	// Redis cache: compiled via build tags. Build with 'go build -tags redis' to enable.
	redisClient := initRedisClient(cfg)
	defer shutdownRedis(redisClient)

	// Wire cron config from config.json
	cronRetryCfg := cfg.Cron.ToRetryConfig()
	// Apply retry config via type assertion on the concrete cron store.
	pgStores.Cron.SetOnJob(nil) // ensure initialized; actual handler set below
	_ = cronRetryCfg            // config available; pg cron store reads it internally
	if cfg.Cron.DefaultTimezone != "" {
		pgStores.Cron.SetDefaultTimezone(cfg.Cron.DefaultTimezone)
	}

	// Load secrets from config_secrets table before env overrides.
	// Precedence: config.json → DB secrets → env vars (highest).
	if pgStores.ConfigSecrets != nil {
		if secrets, err := pgStores.ConfigSecrets.GetAll(context.Background()); err == nil && len(secrets) > 0 {
			cfg.ApplyDBSecrets(secrets)
			cfg.ApplyEnvOverrides()
			slog.Info("config secrets loaded from DB", "count", len(secrets))
		}
	}

	// Wire scraper cookie store (deferred — pgStores now available).
	var scraperCookieStore *scraper.ScraperCookieStore
	if pgStores.ConfigSecrets != nil {
		scraperCookieStore = scraper.NewScraperCookieStore(pgStores.ConfigSecrets)
		if st, ok := toolsReg.Get("scraper"); ok {
			if scraperTool, ok := st.(*scraper.ScraperTool); ok {
				scraperTool.SetCookieStore(scraperCookieStore)
			}
		}
	}


	// Register providers from DB (overrides config providers).
	if pgStores.Providers != nil {
		dbGatewayAddr := loopbackAddr(cfg.Gateway.Host, cfg.Gateway.Port)
		registerProvidersFromDB(providerRegistry, pgStores.Providers, pgStores.ConfigSecrets, dbGatewayAddr, cfg.Gateway.Token, pgStores.MCP)
	}

	setupMemoryEmbeddings(cfg, pgStores, providerRegistry)

	loadBootstrapFiles(pgStores, workspace, agentCfg)

	// Subagent system
	subagentMgr := setupSubagents(providerRegistry, cfg, msgBus, toolsReg, workspace, sandboxMgr)
	if subagentMgr != nil {
		// Wire announce queue for batched subagent result delivery (matching TS debounce pattern)
		announceQueue := tools.NewAnnounceQueue(1000, 20,
			func(sessionKey string, items []tools.AnnounceQueueItem, meta tools.AnnounceMetadata) {
				remainingActive := subagentMgr.CountRunningForParent(meta.ParentAgent)
				content := tools.FormatBatchedAnnounce(items, remainingActive)
				senderID := fmt.Sprintf("subagent:batch-%d", len(items))
				label := items[0].Label
				if len(items) > 1 {
					label = fmt.Sprintf("%d tasks", len(items))
				}
				batchMeta := map[string]string{
					"origin_channel":      meta.OriginChannel,
					"origin_peer_kind":    meta.OriginPeerKind,
					"parent_agent":        meta.ParentAgent,
					"subagent_label":      label,
					"origin_trace_id":     meta.OriginTraceID,
					"origin_root_span_id": meta.OriginRootSpanID,
				}
				if meta.OriginLocalKey != "" {
					batchMeta["origin_local_key"] = meta.OriginLocalKey
				}
				if meta.OriginSessionKey != "" {
					batchMeta["origin_session_key"] = meta.OriginSessionKey
				}
				// Collect media from all items in the batch.
				var batchMedia []bus.MediaFile
				for _, item := range items {
					batchMedia = append(batchMedia, item.Media...)
				}
				msgBus.PublishInbound(bus.InboundMessage{
					Channel:  "system",
					SenderID: senderID,
					ChatID:   meta.OriginChatID,
					Content:  content,
					UserID:   meta.OriginUserID,
					Metadata: batchMeta,
					Media:    batchMedia,
				})
			},
			func(parentID string) int {
				return subagentMgr.CountRunningForParent(parentID)
			},
		)
		subagentMgr.SetAnnounceQueue(announceQueue)

		toolsReg.Register(tools.NewSpawnTool(subagentMgr, "default", 0))
		slog.Info("subagent system enabled", "tools", []string{"spawn"})
	}

	skillsLoader, skillSearchTool, globalSkillsDir := setupSkillsSystem(cfg, workspace, dataDir, pgStores, toolsReg, providerRegistry, msgBus)
	_ = skillSearchTool // used via wireExtras → skillsLoader; kept for type clarity

	// DateTime tool (precise time for cron scheduling, memory timestamps, etc.)
	toolsReg.Register(tools.NewDateTimeTool())

	// Cron tool (agent-facing, matching TS cron-tool.ts)
	toolsReg.Register(tools.NewCronTool(pgStores.Cron))
	slog.Info("cron tool registered")

	// News tool (unified: save, query, sources, ideas)
	toolsReg.Register(tools.NewNewsTool(pgStores.News))
	slog.Info("news tool registered")

	// Knowledge base search tool
	if pgStores.KB != nil {
		toolsReg.Register(tools.NewKBSearchTool(pgStores.KB))
		slog.Info("kb_search tool registered")
	}

	// Analytics tool
	toolsReg.Register(tools.NewAnalyticsTool(pgStores.Analytics))
	slog.Info("analytics tool registered")

	// Notification tool
	toolsReg.Register(tools.NewNotificationTool(pgStores.Notifications, msgBus))
	slog.Info("notification tool registered")

	// Social management
	social.GraphVersion = cfg.Social.FacebookGraphVersion()
	var socialManager *social.Manager
	if pgStores.Social != nil {
		socialManager = social.NewManager(pgStores.Social, os.Getenv("GOCLAW_ENCRYPTION_KEY"))
		if browserMgr != nil {
			socialManager.SetBrowser(browserMgr)
			slog.Info("social manager: browser automation enabled")
		}
		if scraperCookieStore != nil {
			socialManager.SetCookieStore(scraperCookieStore)
		}
		slog.Info("social manager initialized")
	}

	// Session tools (list, status, history, send)
	toolsReg.Register(tools.NewSessionsListTool())
	toolsReg.Register(tools.NewSessionStatusTool())
	toolsReg.Register(tools.NewSessionsHistoryTool())
	toolsReg.Register(tools.NewSessionsSendTool())

	// Message tool (send to channels)
	toolsReg.Register(tools.NewMessageTool(workspace, agentCfg.RestrictToWorkspace))
	slog.Info("session + message tools registered")

	// Register legacy tool aliases (backward-compat names from policy.go).
	for alias, canonical := range tools.LegacyToolAliases() {
		toolsReg.RegisterAlias(alias, canonical)
	}

	// Register Claude Code tool aliases so Claude Code skills work without modification.
	// LLM calls alias name → registry resolves to canonical tool → executes.
	for alias, canonical := range map[string]string{
		"Read":       "read_file",
		"Write":      "write_file",
		"Edit":       "edit",
		"Bash":       "exec",
		"WebFetch":   "web_fetch",
		"WebSearch":  "web_search",
		"Agent":      "spawn",
		"Skill":      "use_skill",
		"ToolSearch": "mcp_tool_search",
	} {
		toolsReg.RegisterAlias(alias, canonical)
	}
	slog.Info("tool aliases registered", "count", len(toolsReg.Aliases()))

	// Allow read_file to access skills directories and CLI workspaces (outside workspace).
	// Skills can live under dataDir/skills/, ~/.agents/skills/, dataDir/skills-store/, etc.
	// CLI workspaces live in dataDir/cli-workspaces/ (agent working files).
	homeDir, _ := os.UserHomeDir()
	if readTool, ok := toolsReg.Get("read_file"); ok {
		if pa, ok := readTool.(tools.PathAllowable); ok {
			pa.AllowPaths(globalSkillsDir)
			if homeDir != "" {
				pa.AllowPaths(filepath.Join(homeDir, ".agents", "skills"))
			}
			pa.AllowPaths(filepath.Join(dataDir, "cli-workspaces"))
			// Also allow the skills store directory (uploaded skill content).
			if pgStores.Skills != nil {
				pa.AllowPaths(pgStores.Skills.Dirs()...)
			}
		}
	}

	// Memory tools are PG-backed; always available.
	hasMemory := true

	// Wire SessionStoreAware + BusAware on tools that need them
	for _, name := range []string{"sessions_list", "session_status", "sessions_history", "sessions_send"} {
		if t, ok := toolsReg.Get(name); ok {
			if sa, ok := t.(tools.SessionStoreAware); ok {
				sa.SetSessionStore(pgStores.Sessions)
			}
			if ba, ok := t.(tools.BusAware); ok {
				ba.SetMessageBus(msgBus)
			}
		}
	}
	// Wire BusAware on message tool
	if t, ok := toolsReg.Get("message"); ok {
		if ba, ok := t.(tools.BusAware); ok {
			ba.SetMessageBus(msgBus)
		}
	}

	// Create all agents — resolved lazily from database by the managed resolver.
	agentRouter := agent.NewRouter()
	slog.Info("agents will be resolved lazily from database")

	// Create gateway server and wire enforcement
	server := gateway.NewServer(cfg, msgBus, agentRouter, pgStores.Sessions, toolsReg)
	server.SetVersion(Version)
	server.SetDB(pgStores.DB)
	server.SetPolicyEngine(permPE)
	server.SetPairingService(pgStores.Pairing)
	server.SetMessageBus(msgBus)
	server.SetOAuthHandler(httpapi.NewOAuthHandler(cfg.Gateway.Token, pgStores.Providers, pgStores.ConfigSecrets, providerRegistry, msgBus))

	// contextFileInterceptor is created inside wireExtras.
	// Declared here so it can be passed to registerAllMethods → AgentsMethods
	// for immediate cache invalidation on agents.files.set.
	var contextFileInterceptor *tools.ContextFileInterceptor
	var projectManagerForShutdown *ccpkg.ProcessManager
	var projectStoreForChannels store.ProjectStore

	// Set agent store for tools_invoke context injection + wire extras
	if pgStores.Agents != nil {
		server.SetAgentStore(pgStores.Agents)
	}

	// Dynamic custom tools: load global tools from DB before resolver
	var dynamicLoader *tools.DynamicToolLoader
	if pgStores.CustomTools != nil {
		dynamicLoader = tools.NewDynamicToolLoader(pgStores.CustomTools, workspace)
		if err := dynamicLoader.LoadGlobal(context.Background(), toolsReg); err != nil {
			slog.Warn("failed to load global custom tools", "error", err)
		}
	}

	var mcpPool *mcpbridge.Pool
	var mediaStore media.Storage
	var postTurn tools.PostTurnProcessor
	contextFileInterceptor, mcpPool, mediaStore, postTurn = wireExtras(pgStores, agentRouter, providerRegistry, msgBus, pgStores.Sessions, toolsReg, toolPE, skillsLoader, hasMemory, traceCollector, workspace, cfg.Gateway.InjectionAction, cfg, sandboxMgr, dynamicLoader, redisClient)
	if mcpPool != nil {
		defer mcpPool.Stop()
	}
	gatewayAddr := loopbackAddr(cfg.Gateway.Host, cfg.Gateway.Port)
	var mcpToolLister httpapi.MCPToolLister
	if mcpMgr != nil {
		mcpToolLister = mcpMgr
	}
	agentsH, skillsH, tracesH, mcpH, customToolsH, channelInstancesH, providersH, delegationsH, builtinToolsH, pendingMessagesH, teamEventsH, secureCLIH := wireHTTP(pgStores, cfg.Gateway.Token, cfg.Agents.Defaults.Workspace, msgBus, toolsReg, providerRegistry, permPE.IsOwner, gatewayAddr, mcpToolLister)
	if agentsH != nil {
		server.SetAgentsHandler(agentsH)
	}
	if skillsH != nil {
		server.SetSkillsHandler(skillsH)
	}
	if tracesH != nil {
		server.SetTracesHandler(tracesH)
	}
	// External wake/trigger API
	wakeH := httpapi.NewWakeHandler(agentRouter, cfg.Gateway.Token)
	server.SetWakeHandler(wakeH)
	if mcpH != nil {
		server.SetMCPHandler(mcpH)
	}
	if customToolsH != nil {
		server.SetCustomToolsHandler(customToolsH)
	}
	if channelInstancesH != nil {
		server.SetChannelInstancesHandler(channelInstancesH)
	}
	if providersH != nil {
		server.SetProvidersHandler(providersH)
	}
	if delegationsH != nil {
		server.SetDelegationsHandler(delegationsH)
	}
	if teamEventsH != nil {
		server.SetTeamEventsHandler(teamEventsH)
	}
	if builtinToolsH != nil {
		server.SetBuiltinToolsHandler(builtinToolsH)
	}
	if pendingMessagesH != nil {
		if pc := cfg.Channels.PendingCompaction; pc != nil {
			pendingMessagesH.SetKeepRecent(pc.KeepRecent)
			pendingMessagesH.SetMaxTokens(pc.MaxTokens)
			pendingMessagesH.SetProviderModel(pc.Provider, pc.Model)
		}
		server.SetPendingMessagesHandler(pendingMessagesH)
	}

	if secureCLIH != nil {
		server.SetSecureCLIHandler(secureCLIH)
	}

	// Activity audit log API
	if pgStores.Activity != nil {
		server.SetActivityHandler(httpapi.NewActivityHandler(pgStores.Activity, cfg.Gateway.Token))
	}

	// Usage analytics API
	if pgStores.Snapshots != nil {
		server.SetUsageHandler(httpapi.NewUsageHandler(pgStores.Snapshots, pgStores.DB, cfg.Gateway.Token))
	}

	// API key management
	// API documentation (OpenAPI spec + Swagger UI at /docs)
	server.SetDocsHandler(httpapi.NewDocsHandler(cfg.Gateway.Token))

	if pgStores != nil && pgStores.APIKeys != nil {
		server.SetAPIKeysHandler(httpapi.NewAPIKeysHandler(pgStores.APIKeys, cfg.Gateway.Token, msgBus))
		server.SetAPIKeyStore(pgStores.APIKeys)
		httpapi.InitAPIKeyCache(pgStores.APIKeys, msgBus)
	}

	// Allow browser-paired users to access HTTP APIs
	if pgStores.Pairing != nil {
		httpapi.InitPairingAuth(pgStores.Pairing)
	}

	// Memory management API (wired directly, only needs MemoryStore + token)
	if pgStores != nil && pgStores.Memory != nil {
		server.SetMemoryHandler(httpapi.NewMemoryHandler(pgStores.Memory, cfg.Gateway.Token))
	}

	// Knowledge Base API (RAG: upload, process, search)
	var kbProc *kb.Processor
	if pgStores != nil && pgStores.KB != nil && mediaStore != nil {
		memCfg := cfg.Agents.Defaults.Memory
		var kbEmbProvider store.EmbeddingProvider
		if ep := resolveEmbeddingProvider(cfg, memCfg, providerRegistry); ep != nil {
			kbEmbProvider = ep
		}
		kbProc = kb.NewProcessor(pgStores.KB, mediaStore, kbEmbProvider)
		server.SetKBHandler(httpapi.NewKBHandler(pgStores.KB, kbProc, mediaStore, cfg.Gateway.Token))
	}

	// Workspace file serving endpoint — serves files by absolute path, auth-token protected.
	// Supports media from any agent workspace (each agent has its own workspace from DB).
	server.SetFilesHandler(httpapi.NewFilesHandler(cfg.Gateway.Token))

	// Storage file management — browse/delete files under the resolved workspace directory.
	// Uses GOCLAW_WORKSPACE (or default ~/.goclaw/workspace) so it works correctly
	// in Docker deployments where volumes are mounted outside ~/.goclaw/.
	server.SetStorageHandler(httpapi.NewStorageHandler(workspace, cfg.Gateway.Token))

	// Media upload endpoint — accepts multipart file uploads, returns temp path + MIME type.
	server.SetMediaUploadHandler(httpapi.NewMediaUploadHandler(cfg.Gateway.Token))

	// Media serve endpoint — serves persisted media files by ID for WS/web clients.
	if mediaStore != nil {
		server.SetMediaServeHandler(httpapi.NewMediaServeHandler(mediaStore, cfg.Gateway.Token))
	}

	// Seed + apply builtin tool disables
	if pgStores.BuiltinTools != nil {
		seedBuiltinTools(context.Background(), pgStores.BuiltinTools)
		migrateBuiltinToolSettings(context.Background(), pgStores.BuiltinTools)
		applyBuiltinToolDisables(context.Background(), pgStores.BuiltinTools, toolsReg)
	}

	// Projects orchestration
	if pgStores.Projects != nil {
		projectEventCB := func(sessionID uuid.UUID, event ccpkg.StreamEvent) {
			// Persist log
			_ = pgStores.Projects.AppendLog(context.Background(), &store.ProjectSessionLogData{
				SessionID: sessionID,
				EventType: event.Type,
				Content:   event.Raw,
			})
			// Broadcast to WS clients
			msgBus.Broadcast(bus.Event{
				Name: protocol.EventProjectOutput,
				Payload: map[string]interface{}{
					"session_id": sessionID,
					"event":      event,
				},
			})
		}
		projectManager := ccpkg.NewProcessManager(pgStores.Projects, projectEventCB)
		projectManager.SetGatewayConfig(gatewayAddr, cfg.Gateway.Token)
		projectsHandler := httpapi.NewProjectsHandler(pgStores.Projects, projectManager, cfg.Gateway.Token, msgBus, permPE.IsOwner, pgStores.Teams)
		server.SetProjectsHandler(projectsHandler)

		// Register WS RPC methods
		methods.NewProjectsMethods(pgStores.Projects, projectManager, msgBus, pgStores.Teams, permPE.IsOwner).Register(server.Router())

		// Agent tools for project session management
		toolsReg.Register(tools.NewProjectsListTool(pgStores.Projects))
		toolsReg.Register(tools.NewProjectSessionStartTool(pgStores.Projects, projectManager))
		toolsReg.Register(tools.NewProjectSessionStatusTool(pgStores.Projects, projectManager))
		toolsReg.Register(tools.NewProjectSessionsListTool(pgStores.Projects))
		toolsReg.Register(tools.NewProjectSessionStopTool(pgStores.Projects, projectManager))

		// Store for channel injection and graceful shutdown
		projectManagerForShutdown = projectManager
		projectStoreForChannels = pgStores.Projects
		slog.Info("projects orchestration enabled")
	}

	// Social management — HTTP + WS
	if pgStores.Social != nil && socialManager != nil {
		socialH := httpapi.NewSocialHandler(pgStores.Social, socialManager, cfg.Gateway.Token)
		server.SetSocialHandler(socialH)
		methods.NewSocialMethods(pgStores.Social, socialManager).Register(server.Router())

		// Social OAuth (all platforms)
		oauthCfgs := httpapi.PlatformOAuthConfigs{}
		if cfg.Social.FacebookAppID != "" && cfg.Social.FacebookAppSecret != "" {
			oauthCfgs.Meta = &social.OAuthConfig{
				ClientID:     cfg.Social.FacebookAppID,
				ClientSecret: cfg.Social.FacebookAppSecret,
			}
		}
		if cfg.Social.TwitterClientID != "" && cfg.Social.TwitterClientSecret != "" {
			oauthCfgs.Twitter = &social.OAuthConfig{
				ClientID:     cfg.Social.TwitterClientID,
				ClientSecret: cfg.Social.TwitterClientSecret,
			}
		}
		if cfg.Social.LinkedInClientID != "" && cfg.Social.LinkedInClientSecret != "" {
			oauthCfgs.LinkedIn = &social.OAuthConfig{
				ClientID:     cfg.Social.LinkedInClientID,
				ClientSecret: cfg.Social.LinkedInClientSecret,
			}
		}
		if cfg.Social.GoogleClientID != "" && cfg.Social.GoogleClientSecret != "" {
			oauthCfgs.Google = &social.OAuthConfig{
				ClientID:     cfg.Social.GoogleClientID,
				ClientSecret: cfg.Social.GoogleClientSecret,
			}
		}
		if cfg.Social.TikTokClientKey != "" && cfg.Social.TikTokClientSecret != "" {
			oauthCfgs.TikTok = &social.OAuthConfig{
				ClientID:     cfg.Social.TikTokClientKey,
				ClientSecret: cfg.Social.TikTokClientSecret,
			}
		}
		socialOAuthH := httpapi.NewSocialOAuthHandler(pgStores.Social, socialManager, cfg.Gateway.Token, oauthCfgs)
		server.SetSocialOAuthHandler(socialOAuthH)

		socialPagesH := httpapi.NewSocialPagesHandler(pgStores.Social, socialManager, socialOAuthH, cfg.Gateway.Token)
		server.SetSocialPagesHandler(socialPagesH)
		var enabledPlatforms []string
		if oauthCfgs.Meta != nil {
			enabledPlatforms = append(enabledPlatforms, "facebook", "instagram", "threads")
		}
		if oauthCfgs.Twitter != nil {
			enabledPlatforms = append(enabledPlatforms, "twitter")
		}
		if oauthCfgs.LinkedIn != nil {
			enabledPlatforms = append(enabledPlatforms, "linkedin")
		}
		if oauthCfgs.Google != nil {
			enabledPlatforms = append(enabledPlatforms, "youtube")
		}
		if oauthCfgs.TikTok != nil {
			enabledPlatforms = append(enabledPlatforms, "tiktok")
		}
		if len(enabledPlatforms) > 0 {
			slog.Info("social OAuth enabled", "platforms", strings.Join(enabledPlatforms, ","))
		}

		slog.Info("social management enabled")
	}

	// Content schedule management — HTTP + WS
	if pgStores.ContentSchedules != nil {
		scheduleH := httpapi.NewContentScheduleHandler(pgStores.ContentSchedules, pgStores.Cron, cfg.Gateway.Token)
		server.SetContentScheduleHandler(scheduleH)
		methods.NewContentScheduleMethods(pgStores.ContentSchedules, pgStores.Cron).Register(server.Router())
		slog.Info("content schedule management enabled")
	}

	// Register all RPC methods
	server.SetLogTee(logTee)
	pairingMethods := registerAllMethods(server, agentRouter, pgStores.Sessions, pgStores.Cron, pgStores.Pairing, cfg, cfgPath, workspace, dataDir, msgBus, execApprovalMgr, pgStores.Agents, pgStores.Skills, pgStores.ConfigSecrets, pgStores.Teams, contextFileInterceptor, logTee, browserMgr, scraperCookieStore, pgStores.News, pgStores.Notifications)

	// Knowledge Base RPC methods
	if pgStores.KB != nil && kbProc != nil {
		methods.NewKBMethods(pgStores.KB, kbProc).Register(server.Router())
		slog.Info("kb RPC methods registered")
	}

	// News HTTP API
	newsHandler := httpapi.NewNewsHandler(pgStores.News, cfg.Gateway.Token)
	server.SetNewsHandler(newsHandler)

	// Notifications HTTP API
	notificationHandler := httpapi.NewNotificationHandler(pgStores.Notifications, cfg.Gateway.Token)
	server.SetNotificationHandler(notificationHandler)
	slog.Info("notification HTTP API registered")

	// Analytics HTTP API
	analyticsHandler := httpapi.NewAnalyticsHandler(pgStores.Analytics, cfg.Gateway.Token)
	server.SetAnalyticsHandler(analyticsHandler)
	slog.Info("analytics HTTP API registered")

	// Wire pairing event broadcasts to all WS clients.
	pairingMethods.SetBroadcaster(server.BroadcastEvent)
	if ps, ok := pgStores.Pairing.(*pg.PGPairingStore); ok {
		ps.SetOnRequest(func(code, senderID, channel, chatID string) {
			server.BroadcastEvent(*protocol.NewEvent(protocol.EventDevicePairReq, map[string]any{
				"code": code, "sender_id": senderID, "channel": channel, "chat_id": chatID,
			}))
		})
	}

	// Channel manager
	channelMgr := channels.NewManager(msgBus)

	// Wire channel sender on message tool (now that channelMgr exists)
	if t, ok := toolsReg.Get("message"); ok {
		if cs, ok := t.(tools.ChannelSenderAware); ok {
			cs.SetChannelSender(channelMgr.SendToChannel)
		}
	}

	// Load channel instances from DB.
	var instanceLoader *channels.InstanceLoader
	if pgStores.ChannelInstances != nil {
		instanceLoader = channels.NewInstanceLoader(pgStores.ChannelInstances, pgStores.Agents, channelMgr, msgBus, pgStores.Pairing)
		// Use projects-enabled factory if project manager is available
		if projectManagerForShutdown != nil && projectStoreForChannels != nil {
			instanceLoader.RegisterFactory(channels.TypeTelegram, telegram.FactoryWithAllStores(pgStores.Agents, pgStores.Teams, projectStoreForChannels, projectManagerForShutdown, pgStores.News, pgStores.Social, socialManager, pgStores.PendingMessages))
		} else {
			instanceLoader.RegisterFactory(channels.TypeTelegram, telegram.FactoryWithAllStores(pgStores.Agents, pgStores.Teams, nil, nil, pgStores.News, pgStores.Social, socialManager, pgStores.PendingMessages))
		}
		instanceLoader.RegisterFactory(channels.TypeDiscord, discord.FactoryWithPendingStore(pgStores.PendingMessages))
		instanceLoader.RegisterFactory(channels.TypeFeishu, feishu.FactoryWithPendingStore(pgStores.PendingMessages))
		instanceLoader.RegisterFactory(channels.TypeZaloOA, zalo.Factory)
		instanceLoader.RegisterFactory(channels.TypeZaloPersonal, zalopersonal.FactoryWithPendingStore(pgStores.PendingMessages))
		instanceLoader.RegisterFactory(channels.TypeWhatsApp, whatsapp.Factory)
		instanceLoader.RegisterFactory(channels.TypeSlack, slackchannel.FactoryWithPendingStore(pgStores.PendingMessages))
		if err := instanceLoader.LoadAll(context.Background()); err != nil {
			slog.Error("failed to load channel instances from DB", "error", err)
		}
	}

	// Register config-based channels as fallback when no DB instances loaded.
	registerConfigChannels(cfg, channelMgr, msgBus, pgStores, instanceLoader)

	// Register channels/instances/links/teams RPC methods
	wireChannelRPCMethods(server, pgStores, channelMgr, agentRouter, msgBus, workspace)

	// Wire channel event subscribers (cache invalidation, pairing, cascade disable)
	wireChannelEventSubscribers(msgBus, server, pgStores, channelMgr, instanceLoader, pairingMethods, cfg)

	// Audit log subscriber — persists audit events to activity_logs table.
	// Uses a buffered channel with a single worker to avoid unbounded goroutines.
	var auditCh chan bus.AuditEventPayload
	if pgStores.Activity != nil {
		auditCh = make(chan bus.AuditEventPayload, 256)
		msgBus.Subscribe(bus.TopicAudit, func(evt bus.Event) {
			if evt.Name != protocol.EventAuditLog {
				return
			}
			payload, ok := evt.Payload.(bus.AuditEventPayload)
			if !ok {
				return
			}
			select {
			case auditCh <- payload:
			default:
				slog.Warn("audit.queue_full", "action", payload.Action)
			}
		})
		go func() {
			for payload := range auditCh {
				if err := pgStores.Activity.Log(context.Background(), &store.ActivityLog{
					ActorType:  payload.ActorType,
					ActorID:    payload.ActorID,
					Action:     payload.Action,
					EntityType: payload.EntityType,
					EntityID:   payload.EntityID,
					IPAddress:  payload.IPAddress,
					Details:    payload.Details,
				}); err != nil {
					slog.Warn("audit.log_failed", "action", payload.Action, "error", err)
				}
			}
		}()
		slog.Info("audit subscriber registered")
	}

	// Team task event subscriber — records task lifecycle events to team_task_events.
	// Listens to bus events (team.task.*) so callers don't need direct RecordTaskEvent calls.
	if pgStores.Teams != nil {
		teamEventStore := pgStores.Teams
		msgBus.Subscribe(bus.TopicTeamTaskAudit, func(evt bus.Event) {
			eventType := teamTaskEventType(evt.Name)
			if eventType == "" {
				return
			}
			payload, ok := evt.Payload.(protocol.TeamTaskEventPayload)
			if !ok {
				return
			}
			taskID, err := uuid.Parse(payload.TaskID)
			if err != nil {
				return
			}
			if err := teamEventStore.RecordTaskEvent(context.Background(), &store.TeamTaskEventData{
				TaskID:    taskID,
				EventType: eventType,
				ActorType: payload.ActorType,
				ActorID:   payload.ActorID,
			}); err != nil {
				slog.Warn("team_task_audit.record_failed", "task_id", payload.TaskID, "event", eventType, "error", err)
			}
		})
		slog.Info("team task event subscriber registered")
	}

	// Team progress notification subscriber — forwards task events to chat channels.
	// Reads team.settings.notifications config; direct mode sends outbound, leader mode
	// injects into leader agent session.
	if pgStores.Teams != nil {
		notifyTeamStore := pgStores.Teams
		notifyAgentStore := pgStores.Agents
		msgBus.Subscribe("consumer.team-notify", func(evt bus.Event) {
			payload, ok := evt.Payload.(protocol.TeamTaskEventPayload)
			if !ok || payload.TeamID == "" || payload.Channel == "" {
				return
			}
			// Only forward assigned/failed events (completed handled by announce-back).
			var notifyType string
			switch evt.Name {
			case protocol.EventTeamTaskAssigned:
				notifyType = "dispatched"
			case protocol.EventTeamTaskFailed:
				notifyType = "failed"
			case protocol.EventTeamTaskProgress:
				notifyType = "progress"
			default:
				return
			}

			teamUUID, err := uuid.Parse(payload.TeamID)
			if err != nil {
				return
			}
			team, err := notifyTeamStore.GetTeam(context.Background(), teamUUID)
			if err != nil || team == nil {
				return
			}
			cfg := tools.ParseTeamNotifyConfig(team.Settings)

			// Check if this notification type is enabled.
			switch notifyType {
			case "dispatched":
				if !cfg.Dispatched {
					return
				}
			case "failed":
				if !cfg.Failed {
					return
				}
			case "progress":
				if !cfg.Progress {
					return
				}
			}

			// Skip internal channels.
			if payload.Channel == tools.ChannelSystem || payload.Channel == tools.ChannelDelegate {
				return
			}

			// Build notification message.
			var content string
			agentName := payload.OwnerAgentKey
			if payload.OwnerDisplayName != "" {
				agentName = payload.OwnerDisplayName
			}
			switch notifyType {
			case "dispatched":
				content = fmt.Sprintf("📋 Task #%d \"%s\" → assigned to %s", payload.TaskNumber, payload.Subject, agentName)
			case "progress":
				if payload.ProgressStep != "" {
					content = fmt.Sprintf("⏳ Task #%d \"%s\": %d%% — %s", payload.TaskNumber, payload.Subject, payload.ProgressPercent, payload.ProgressStep)
				} else {
					content = fmt.Sprintf("⏳ Task #%d \"%s\": %d%%", payload.TaskNumber, payload.Subject, payload.ProgressPercent)
				}
			case "failed":
				reason := payload.Reason
				if len(reason) > 200 {
					reason = reason[:200] + "..."
				}
				content = fmt.Sprintf("❌ Task #%d \"%s\" failed: %s", payload.TaskNumber, payload.Subject, reason)
			}

			if cfg.Mode == "leader" {
				// Route through leader agent — model reformulates.
				leadAgent := ""
				if notifyAgentStore != nil {
					if la, err := notifyAgentStore.GetByID(context.Background(), team.LeadAgentID); err == nil {
						leadAgent = la.AgentKey
					}
				}
				if leadAgent == "" {
					return
				}
				leaderContent := fmt.Sprintf("[Auto-status — relay to user, NO task actions]\n%s\n\nBriefly inform the user. Do NOT create, retry, reassign, or modify any tasks.", content)
				msgBus.TryPublishInbound(bus.InboundMessage{
					Channel:  payload.Channel,
					SenderID: "notification:progress",
					ChatID:   payload.ChatID,
					AgentID:  leadAgent,
					UserID:   payload.UserID,
					Content:  leaderContent,
				})
			} else {
				// Direct mode — send outbound directly to channel.
				msgBus.PublishOutbound(bus.OutboundMessage{
					Channel: payload.Channel,
					ChatID:  payload.ChatID,
					Content: content,
				})
			}
		})
		slog.Info("team progress notification subscriber registered")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Skills directory watcher — auto-detect new/removed/modified skills at runtime.
	if skillsWatcher, err := skills.NewWatcher(skillsLoader); err != nil {
		slog.Warn("skills watcher unavailable", "error", err)
	} else {
		if err := skillsWatcher.Start(ctx); err != nil {
			slog.Warn("skills watcher start failed", "error", err)
		} else {
			defer skillsWatcher.Stop()
		}
	}

	// Start channels
	if err := channelMgr.StartAll(ctx); err != nil {
		slog.Error("failed to start channels", "error", err)
	}

	// Create lane-based scheduler (matching TS CommandLane pattern).
	// The RunFunc resolves the agent from the RunRequest metadata.
	// Must be created before cron setup so cron jobs route through the scheduler.
	sched := scheduler.NewScheduler(
		scheduler.DefaultLanes(),
		scheduler.DefaultQueueConfig(),
		makeSchedulerRunFunc(agentRouter, cfg),
	)
	defer sched.Stop()

	// Build content schedule handler (wires agent scheduler → social publish pipeline).
	var scheduleHandler *social.ScheduleHandler
	if pgStores.ContentSchedules != nil && pgStores.Social != nil {
		contentGen := &scheduleContentGen{sched: sched, cfg: cfg}
		scheduleHandler = social.NewScheduleHandler(pgStores.ContentSchedules, pgStores.Social, contentGen)
		slog.Info("content schedule cron handler enabled")
	}

	// Start cron service with job handler (routes through scheduler's cron lane)
	pgStores.Cron.SetOnJob(makeCronJobHandler(sched, msgBus, cfg, channelMgr, scheduleHandler))
	pgStores.Cron.SetOnEvent(func(event store.CronEvent) {
		server.BroadcastEvent(*protocol.NewEvent(protocol.EventCron, event))
	})
	if err := pgStores.Cron.Start(); err != nil {
		slog.Warn("cron service failed to start", "error", err)
	}

	// Adaptive throttle: reduce per-session concurrency when nearing the summary threshold.
	// This prevents concurrent runs from racing with summarization.
	// Uses calibrated token estimation (actual prompt tokens from last LLM call)
	// and the agent's real context window (cached on session by the Loop).
	sched.SetTokenEstimateFunc(func(sessionKey string) (int, int) {
		history := pgStores.Sessions.GetHistory(sessionKey)
		lastPT, lastMC := pgStores.Sessions.GetLastPromptTokens(sessionKey)
		tokens := agent.EstimateTokensWithCalibration(history, lastPT, lastMC)
		cw := pgStores.Sessions.GetContextWindow(sessionKey)
		if cw <= 0 {
			cw = 200000 // fallback for sessions not yet processed
		}
		return tokens, cw
	})

	// Subscribe to agent events for channel streaming/reaction forwarding.
	// Events emitted by agent loops are broadcast to the bus; we forward them
	// to the channel manager which routes to StreamingChannel/ReactionChannel.
	msgBus.Subscribe(bus.TopicChannelStreaming, func(event bus.Event) {
		if event.Name != protocol.EventAgent {
			return
		}
		agentEvent, ok := event.Payload.(agent.AgentEvent)
		if !ok {
			return
		}
		channelMgr.HandleAgentEvent(agentEvent.Type, agentEvent.RunID, agentEvent.Payload)

		// Route activity events to Router (status registry) and DelegateManager (progress tracking).
		if agentEvent.Type == protocol.AgentEventActivity {
			payloadMap, _ := agentEvent.Payload.(map[string]any)
			phase, _ := payloadMap["phase"].(string)
			tool, _ := payloadMap["tool"].(string)
			iteration := 0
			if v, ok := payloadMap["iteration"].(int); ok {
				iteration = v
			}

			// Update Router activity registry (for status queries via LLM classify)
			if sessionKey := agentRouter.SessionKeyForRun(agentEvent.RunID); sessionKey != "" {
				agentRouter.UpdateActivity(sessionKey, agentEvent.RunID, phase, tool, iteration)
			}

		}

		// Clear activity on terminal events
		if agentEvent.Type == protocol.AgentEventRunCompleted || agentEvent.Type == protocol.AgentEventRunFailed {
			if sessionKey := agentRouter.SessionKeyForRun(agentEvent.RunID); sessionKey != "" {
				agentRouter.ClearActivity(sessionKey)
			}
		}
	})

	// Start inbound message consumer (channel → scheduler → agent → channel)
	consumerTeamStore := pgStores.Teams

	// Quota checker: enforces per-user/group request limits.
	// Merge per-group quotas from channel configs into gateway.quota.groups.
	config.MergeChannelGroupQuotas(cfg)
	var quotaChecker *channels.QuotaChecker
	if cfg.Gateway.Quota != nil && cfg.Gateway.Quota.Enabled {
		quotaChecker = channels.NewQuotaChecker(pgStores.DB, *cfg.Gateway.Quota)
		defer quotaChecker.Stop()
		slog.Info("channel quota enabled",
			"default_hour", cfg.Gateway.Quota.Default.Hour,
			"default_day", cfg.Gateway.Quota.Default.Day,
			"default_week", cfg.Gateway.Quota.Default.Week,
		)
	}

	// Register quota usage RPC.
	// Pass DB so summary cards still work when quota is disabled (queries traces directly).
	methods.NewQuotaMethods(quotaChecker, pgStores.DB).Register(server.Router())

	// API key management RPC
	if pgStores.APIKeys != nil {
		methods.NewAPIKeysMethods(pgStores.APIKeys).Register(server.Router())
	}

	// Reload quota config on config changes via pub/sub.
	if quotaChecker != nil {
		msgBus.Subscribe("quota-config-reload", func(evt bus.Event) {
			if evt.Name != bus.TopicConfigChanged {
				return
			}
			updatedCfg, ok := evt.Payload.(*config.Config)
			if !ok || updatedCfg.Gateway.Quota == nil {
				return
			}
			config.MergeChannelGroupQuotas(updatedCfg)
			quotaChecker.UpdateConfig(*updatedCfg.Gateway.Quota)
			slog.Info("quota config reloaded via pub/sub")
		})
	}

	// Reload cron default timezone on config changes via pub/sub.
	msgBus.Subscribe("cron-config-reload", func(evt bus.Event) {
		if evt.Name != bus.TopicConfigChanged {
			return
		}
		updatedCfg, ok := evt.Payload.(*config.Config)
		if !ok {
			return
		}
		pgStores.Cron.SetDefaultTimezone(updatedCfg.Cron.DefaultTimezone)
	})

	// Reload web_fetch domain policy on config changes via pub/sub.
	msgBus.Subscribe("webfetch-config-reload", func(evt bus.Event) {
		if evt.Name != bus.TopicConfigChanged {
			return
		}
		updatedCfg, ok := evt.Payload.(*config.Config)
		if !ok {
			return
		}
		webFetchTool.UpdatePolicy(updatedCfg.Tools.WebFetch.Policy, updatedCfg.Tools.WebFetch.AllowedDomains, updatedCfg.Tools.WebFetch.BlockedDomains)
	})

	// Contact collector: auto-collect user info from channels with in-memory dedup cache.
	var contactCollector *store.ContactCollector
	if pgStores.Contacts != nil {
		contactCollector = store.NewContactCollector(pgStores.Contacts, cache.NewInMemoryCache[bool]())
		channelMgr.SetContactCollector(contactCollector) // propagate to all channel handlers
	}

	go consumeInboundMessages(ctx, msgBus, agentRouter, cfg, sched, channelMgr, consumerTeamStore, quotaChecker, pgStores.Sessions, pgStores.Agents, contactCollector, postTurn)

	// Task recovery ticker: re-dispatches stale/pending team tasks on startup and periodically.
	var taskTicker *tasks.TaskTicker
	if pgStores.Teams != nil {
		taskTicker = tasks.NewTaskTicker(pgStores.Teams, pgStores.Agents, msgBus, cfg.Gateway.TaskRecoveryIntervalSec)
		taskTicker.Start()
	}

	go func() {
		sig := <-sigCh
		slog.Info("graceful shutdown initiated", "signal", sig)

		// Broadcast shutdown event
		server.BroadcastEvent(*protocol.NewEvent(protocol.EventShutdown, nil))

		// Stop channels, cron, and task ticker
		channelMgr.StopAll(context.Background())
		pgStores.Cron.Stop()
		if taskTicker != nil {
			taskTicker.Stop()
		}

		// Stop project session processes
		if projectManagerForShutdown != nil {
			slog.Info("stopping project session processes...")
			projectManagerForShutdown.StopAll()
		}

		// Close provider resources (e.g. Claude CLI temp files)
		providerRegistry.Close()

		// Stop sandbox pruning + release containers
		if sandboxMgr != nil {
			sandboxMgr.Stop()
			slog.Info("releasing sandbox containers...")
			sandboxMgr.ReleaseAll(context.Background())
		}

		cancel()
	}()

	slog.Info("goclaw gateway starting",
		"version", Version,
		"protocol", protocol.ProtocolVersion,
		"agents", agentRouter.List(),
		"tools", toolsReg.Count(),
		"channels", channelMgr.GetEnabledChannels(),
	)

	// Tailscale listener: build the mux first, then pass it to initTailscale
	// so the same routes are served on both the main listener and Tailscale.
	// Compiled via build tags: `go build -tags tsnet` to enable.
	mux := server.BuildMux()

	// Mount channel webhook handlers on the main mux (e.g. Feishu /feishu/events).
	// This allows webhook-based channels to share the main server port.
	for _, route := range channelMgr.WebhookHandlers() {
		mux.Handle(route.Path, route.Handler)
		slog.Info("webhook route mounted on gateway", "path", route.Path)
	}

	tsCleanup := initTailscale(ctx, cfg, mux)
	if tsCleanup != nil {
		defer tsCleanup()
	}

	// Phase 1: suggest localhost binding when Tailscale is active
	if cfg.Tailscale.Hostname != "" && cfg.Gateway.Host == "0.0.0.0" {
		slog.Info("Tailscale enabled. Consider setting GOCLAW_HOST=127.0.0.1 for localhost-only + Tailscale access")
	}

	if err := server.Start(ctx); err != nil {
		slog.Error("gateway error", "error", err)
		os.Exit(1)
	}
}

// teamTaskEventType maps bus event names to team_task_events.event_type values.
// Returns empty string for non-task events (caller should skip).
func teamTaskEventType(eventName string) string {
	switch eventName {
	case protocol.EventTeamTaskCreated:
		return "created"
	case protocol.EventTeamTaskClaimed:
		return "claimed"
	case protocol.EventTeamTaskAssigned:
		return "assigned"
	case protocol.EventTeamTaskCompleted:
		return "completed"
	case protocol.EventTeamTaskFailed:
		return "failed"
	case protocol.EventTeamTaskCancelled:
		return "cancelled"
	case protocol.EventTeamTaskReviewed:
		return "reviewed"
	case protocol.EventTeamTaskApproved:
		return "approved"
	case protocol.EventTeamTaskRejected:
		return "rejected"
	default:
		return ""
	}
}
