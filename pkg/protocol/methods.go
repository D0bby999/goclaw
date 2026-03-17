package protocol

// RPC method name constants.
// Organized by priority: CRITICAL (Phase 1) → NEEDED (Phase 2) → NICE TO HAVE (Phase 3+).

// Phase 1 - CRITICAL methods
const (
	// Agent
	MethodAgent            = "agent"
	MethodAgentWait        = "agent.wait"
	MethodAgentIdentityGet = "agent.identity.get"

	// Chat
	MethodChatSend    = "chat.send"
	MethodChatHistory = "chat.history"
	MethodChatAbort   = "chat.abort"
	MethodChatInject  = "chat.inject"

	// Agents management
	MethodAgentsList     = "agents.list"
	MethodAgentsCreate   = "agents.create"
	MethodAgentsUpdate   = "agents.update"
	MethodAgentsDelete   = "agents.delete"
	MethodAgentsFileList = "agents.files.list"
	MethodAgentsFileGet  = "agents.files.get"
	MethodAgentsFileSet  = "agents.files.set"

	// Config
	MethodConfigGet    = "config.get"
	MethodConfigApply  = "config.apply"
	MethodConfigPatch  = "config.patch"
	MethodConfigSchema = "config.schema"

	// Sessions
	MethodSessionsList    = "sessions.list"
	MethodSessionsPreview = "sessions.preview"
	MethodSessionsPatch   = "sessions.patch"
	MethodSessionsDelete  = "sessions.delete"
	MethodSessionsReset   = "sessions.reset"

	// System
	MethodConnect = "connect"
	MethodHealth  = "health"
	MethodStatus  = "status"
)

// Phase 2 - NEEDED methods
const (
	MethodSkillsList  = "skills.list"
	MethodSkillsGet   = "skills.get"
	MethodSkillsUpdate = "skills.update"

	MethodCronList   = "cron.list"
	MethodCronCreate = "cron.create"
	MethodCronUpdate = "cron.update"
	MethodCronDelete = "cron.delete"
	MethodCronToggle = "cron.toggle"
	MethodCronStatus = "cron.status"
	MethodCronRun    = "cron.run"
	MethodCronRuns   = "cron.runs"

	MethodChannelsList   = "channels.list"
	MethodChannelsStatus = "channels.status"
	MethodChannelsToggle = "channels.toggle"

	MethodPairingRequest = "device.pair.request"
	MethodPairingApprove = "device.pair.approve"
	MethodPairingDeny    = "device.pair.deny"
	MethodPairingList    = "device.pair.list"
	MethodPairingRevoke  = "device.pair.revoke"

	MethodBrowserPairingStatus = "browser.pairing.status"

	MethodApprovalsList    = "exec.approval.list"
	MethodApprovalsApprove = "exec.approval.approve"
	MethodApprovalsDeny    = "exec.approval.deny"

	MethodUsageGet     = "usage.get"
	MethodUsageSummary = "usage.summary"

	MethodQuotaUsage = "quota.usage"

	MethodSend = "send"
)

// Channel instances management
const (
	MethodChannelInstancesList   = "channels.instances.list"
	MethodChannelInstancesGet    = "channels.instances.get"
	MethodChannelInstancesCreate = "channels.instances.create"
	MethodChannelInstancesUpdate = "channels.instances.update"
	MethodChannelInstancesDelete = "channels.instances.delete"
)

// Agent links (inter-agent delegation)
const (
	MethodAgentsLinksList   = "agents.links.list"
	MethodAgentsLinksCreate = "agents.links.create"
	MethodAgentsLinksUpdate = "agents.links.update"
	MethodAgentsLinksDelete = "agents.links.delete"
)

// Agent teams
const (
	MethodTeamsList     = "teams.list"
	MethodTeamsCreate   = "teams.create"
	MethodTeamsGet      = "teams.get"
	MethodTeamsDelete   = "teams.delete"
	MethodTeamsTaskList      = "teams.tasks.list"
	MethodTeamsTaskGet       = "teams.tasks.get"
	MethodTeamsTaskApprove   = "teams.tasks.approve"
	MethodTeamsTaskReject    = "teams.tasks.reject"
	MethodTeamsTaskComment   = "teams.tasks.comment"
	MethodTeamsTaskComments  = "teams.tasks.comments"
	MethodTeamsTaskEvents    = "teams.tasks.events"
	MethodTeamsTaskCreate    = "teams.tasks.create"
	MethodTeamsTaskDelete    = "teams.tasks.delete"
	MethodTeamsTaskAssign    = "teams.tasks.assign"
	MethodTeamsMembersAdd    = "teams.members.add"
	MethodTeamsMembersRemove = "teams.members.remove"
	MethodTeamsUpdate        = "teams.update"
	MethodTeamsKnownUsers    = "teams.known_users"
	MethodTeamsScopes        = "teams.scopes"
)

// Team workspace
const (
	MethodTeamsWorkspaceList   = "teams.workspace.list"
	MethodTeamsWorkspaceRead   = "teams.workspace.read"
	MethodTeamsWorkspaceDelete = "teams.workspace.delete"
)

// Team events
const (
	MethodTeamsEventsList = "teams.events.list"
)

// Delegation history
const (
	MethodDelegationsList = "delegations.list"
	MethodDelegationsGet  = "delegations.get"
)

// Projects orchestration (managed mode)
const (
	MethodProjectsList   = "projects.list"
	MethodProjectsCreate = "projects.create"
	MethodProjectsGet    = "projects.get"
	MethodProjectsUpdate = "projects.update"
	MethodProjectsDelete = "projects.delete"
	MethodProjectSessionsList   = "projects.sessions.list"
	MethodProjectSessionsStart  = "projects.sessions.start"
	MethodProjectSessionsGet    = "projects.sessions.get"
	MethodProjectSessionsPrompt = "projects.sessions.prompt"
	MethodProjectSessionsStop   = "projects.sessions.stop"
	MethodProjectSessionsDelete = "projects.sessions.delete"
	MethodProjectSessionsUpdate = "projects.sessions.update"
	MethodProjectSessionsLogs   = "projects.sessions.logs"
	MethodProjectMembersList    = "projects.members.list"
	MethodProjectMembersAdd     = "projects.members.add"
	MethodProjectMembersRemove  = "projects.members.remove"
)

// Scraper cookie management
const (
	MethodScraperCookiesList   = "scraper.cookies.list"
	MethodScraperCookiesSet    = "scraper.cookies.set"
	MethodScraperCookiesDelete = "scraper.cookies.delete"
	MethodScraperCookiesLogin  = "scraper.cookies.login"
)

// News digest management
const (
	MethodNewsSourcesList   = "news.sources.list"
	MethodNewsSourcesCreate = "news.sources.create"
	MethodNewsSourcesUpdate = "news.sources.update"
	MethodNewsSourcesDelete = "news.sources.delete"
	MethodNewsItemsList     = "news.items.list"
	MethodNewsItemsStats    = "news.items.stats"
)

// Social management
const (
	MethodSocialAccountsList   = "social.accounts.list"
	MethodSocialAccountsCreate = "social.accounts.create"
	MethodSocialAccountsUpdate = "social.accounts.update"
	MethodSocialAccountsDelete = "social.accounts.delete"
	MethodSocialPostsList      = "social.posts.list"
	MethodSocialPostsCreate    = "social.posts.create"
	MethodSocialPostsGet       = "social.posts.get"
	MethodSocialPostsUpdate    = "social.posts.update"
	MethodSocialPostsDelete    = "social.posts.delete"
	MethodSocialPostsPublish   = "social.posts.publish"
	MethodSocialTargetsAdd     = "social.posts.targets.add"
	MethodSocialTargetsRemove  = "social.posts.targets.remove"
	MethodSocialMediaAdd       = "social.posts.media.add"
	MethodSocialMediaRemove    = "social.posts.media.remove"
)

// Notifications
const (
	MethodNotificationsList    = "notifications.list"
	MethodNotificationsUnread  = "notifications.unread_count"
	MethodNotificationsRead    = "notifications.mark_read"
	MethodNotificationsReadAll = "notifications.mark_all_read"
)

// Content schedule management
const (
	MethodSchedulesList   = "schedules.list"
	MethodSchedulesCreate = "schedules.create"
	MethodSchedulesGet    = "schedules.get"
	MethodSchedulesUpdate = "schedules.update"
	MethodSchedulesDelete = "schedules.delete"
	MethodSchedulesToggle = "schedules.toggle"
	MethodSchedulesLogs   = "schedules.logs"
)

// Knowledge Base
const (
	MethodKBCollectionsList   = "kb.collections.list"
	MethodKBCollectionsCreate = "kb.collections.create"
	MethodKBCollectionsUpdate = "kb.collections.update"
	MethodKBCollectionsDelete = "kb.collections.delete"
	MethodKBDocumentsList     = "kb.documents.list"
	MethodKBDocumentsGet      = "kb.documents.get"
	MethodKBDocumentsDelete   = "kb.documents.delete"
	MethodKBDocumentsReprocess = "kb.documents.reprocess"
	MethodKBSearch            = "kb.search"
)

// API key management
const (
	MethodAPIKeysList   = "api_keys.list"
	MethodAPIKeysCreate = "api_keys.create"
	MethodAPIKeysRevoke = "api_keys.revoke"
)

// Phase 3+ - NICE TO HAVE methods
const (
	MethodLogsTail = "logs.tail"

	MethodTTSStatus      = "tts.status"
	MethodTTSEnable      = "tts.enable"
	MethodTTSDisable     = "tts.disable"
	MethodTTSConvert     = "tts.convert"
	MethodTTSSetProvider = "tts.setProvider"
	MethodTTSProviders   = "tts.providers"

	MethodBrowserAct        = "browser.act"
	MethodBrowserSnapshot   = "browser.snapshot"
	MethodBrowserScreenshot = "browser.screenshot"

	// Zalo Personal
	MethodZaloPersonalQRStart   = "zalo.personal.qr.start"
	MethodZaloPersonalContacts  = "zalo.personal.contacts"
)
