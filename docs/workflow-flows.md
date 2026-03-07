# Workflow Flows

Tài liệu mô tả toàn bộ luồng hoạt động khi làm việc theo team trong GoClaw.

---

## 1. Flow Tổng Quan: User → Lead → Members → User

```
User gửi tin nhắn
     │
     ▼
┌─────────────────────────┐
│      LEAD AGENT         │
│  (có TEAM.md đầy đủ)   │
│                         │
│  1. Phân tích yêu cầu  │
│  2. Tạo task trên board │
│  3. Delegate cho member │
└──────────┬──────────────┘
           │ spawn (async)
     ┌─────┴──────┐
     ▼            ▼
┌─────────┐  ┌─────────┐
│ Member A│  │ Member B│
│ (session│  │ (session│
│  riêng) │  │  riêng) │
└────┬────┘  └────┬────┘
     │            │
     ▼            ▼
  Kết quả     Kết quả
     │            │
     └─────┬──────┘
           ▼
┌─────────────────────────┐
│  Gom kết quả (batch)    │
│  → 1 announcement duy   │
│    nhất gửi cho Lead    │
└──────────┬──────────────┘
           ▼
┌─────────────────────────┐
│      LEAD AGENT         │
│  Tổng hợp → trả lời    │
│  User                   │
└──────────┬──────────────┘
           ▼
       User nhận kết quả
```

---

## 2. Flow Tạo Team (Admin)

```
Admin gọi teams.create (name, lead, members[])
     │
     ▼
1. resolveAgentInfo(lead) → by key hoặc UUID
     │
     ▼
2. Resolve tất cả member agents
     │
     ▼
3. INSERT agent_teams (status=active)
     │
     ▼
4. AddMember(lead, role=lead) — ON CONFLICT UPDATE role
   AddMember(memberN, role=member) — mỗi member
     │
     ▼
5. autoCreateTeamLinks():
   Lead → mỗi Member (direction=outbound, max_concurrent=3)
   Gắn team_id vào link → phân biệt với manual links
   Skip nếu link đã tồn tại (UNIQUE constraint)
   ⚠ Members KHÔNG thể delegate cho nhau hoặc ngược lại Lead
     │
     ▼
6. InvalidateAgent() cho lead + tất cả members
   → Xóa Loop cache → TEAM.md inject lại lần request kế
     │
     ▼
7. msgBus.Broadcast(EventTeamCreated)
     │
     ▼
Team sẵn sàng hoạt động
```

---

## 3. Flow Thêm/Xóa Member

### 3a. Thêm Member

```
Admin gọi teams.members.add
     │
     ▼
1. Validate team tồn tại
2. Reject nếu agent == lead (đã là member)
     │
     ▼
3. AddMember(teamID, agentID, "member")
   — ON CONFLICT DO UPDATE role
     │
     ▼
4. autoCreateTeamLinks() → tạo link Lead → new Member
     │
     ▼
5. invalidateTeamCaches():
   • InvalidateAgent() cho TẤT CẢ members hiện tại
   • emitTeamCacheInvalidate() → pub/sub
     → teamMgr.InvalidateTeam() (full cache flush)
     │
     ▼
6. EventTeamMemberAdded
```

### 3b. Xóa Member

```
Admin gọi teams.members.remove
     │
     ▼
1. Validate team + guard: không thể xóa lead
     │
     ▼
2. Fetch agent info (cho event)
     │
     ▼
3. RemoveMember(teamID, agentID) — DELETE
     │
     ▼
4. linkStore.DeleteTeamLinksForAgent(teamID, agentID)
   → Xóa agent links gắn team_id cho agent này
     │
     ▼
5. invalidateTeamCaches() cho remaining members
   + InvalidateAgent(removedAgent) riêng
     │
     ▼
6. EventTeamMemberRemoved

⚠ Tasks đang owned bởi removed agent KHÔNG bị reassign
  → Vẫn nằm trong DB với owner_agent_id cũ
```

---

## 4. Flow Xóa Team (Cascade)

```
Admin gọi teams.delete
     │
     ▼
1. Fetch team + members TRƯỚC khi xóa (cho events)
     │
     ▼
2. teamStore.DeleteTeam(teamID)
   → DB CASCADE xóa:
     • agent_team_members
     • team_tasks
     • team_messages
     • agent_links có team_id (qua DeleteTeamLinksForAgent)
     • handoff_routes (?)
     │
     ▼
3. InvalidateAgent() cho mỗi member
     │
     ▼
4. EventTeamDeleted
```

---

## 5. Flow TEAM.md Injection (Agent Resolution)

```
User gửi request → Agent Router resolve agent
     │
     ▼
┌──────────────────────────────────┐
│ Load bootstrap context files     │
│ (SOUL.md, AGENTS.md, TOOLS.md)  │
└──────────┬───────────────────────┘
           │
           ▼
  TEAM.md đã tồn tại trong DB?
     │              │
     Yes            No
     │              │
     ▼              ▼
  hasTeam=true    GetTeamForAgent(agentID)
  (skip inject)       │
                      ▼
                 Team found?
                 │         │
                 Yes       No
                 │         │
                 ▼         ▼
           ListMembers   Inject AVAILABILITY.md:
                │        "You are NOT part of any team.
                ▼         Do NOT use team_tasks/team_message"
           buildTeamMD()
                │
           ┌────┴────┐
           ▼         ▼
       isLead?    isMember?
           │         │
           ▼         ▼
     Full TEAM.md   Simple TEAM.md
     • Orchestration  • "Just do the work"
       patterns       • Progress update guide
     • spawn examples • Limited task board
     • Communication  • team_message OK
       guidelines
     • team_message
       DENIED (lead
       must use spawn)

⚠ Nếu BOOTSTRAP.md tồn tại → SKIP TEAM.md injection
  (tránh token waste khi first-run)

⚠ Nếu !hasTeam && !hasDelegation:
  → Inject: "You have NO delegation targets.
     Do NOT use spawn with agent parameter"
```

### TEAM.md vs DELEGATION.md

```
Team auto-links (có team_id) → TEAM.md
Manual links (không team_id) → DELEGATION.md

filterManualLinks() loại bỏ team links khỏi DELEGATION.md
→ Agent dùng TEAM.md + team_tasks/team_message tools
→ Không trùng lặp với DELEGATION.md
```

---

## 6. Flow prepareDelegation (Common Setup)

Cả sync và async đều dùng chung flow chuẩn bị này.

```
Lead gọi spawn(agent=targetKey, task=..., team_task_id=...)
     │
     ▼
1. Get sourceAgentID từ context (yêu cầu managed mode)
     │
     ▼
2. GetByID(source) → fetch source agent
   GetByKey(target) → fetch target agent
     │
     ▼
3. GetLinkBetween(source, target)
   → Link phải tồn tại, nếu không → reject
     │
     ▼
4. checkUserPermission(link.Settings, userID)
   → Per-link UserAllow/UserDeny
     │
     ▼
5. GetTeamForAgent(sourceAgentID)
   → Optional team context
     │
     ▼
6. TeamTaskID handling:
   ┌────────────────────┐
   │ TeamTaskID == nil?  │
   │ (LLM quên gửi)    │
   └──────┬─────────────┘
          │
     ┌────┴────┐
     ▼         ▼
   Auto-tạo   Validate:
   task mới    • Cross-team? → reject
   (pending)   • Cross-group user? → reject
   Gán vào     • Completed/cancelled? → reject
   opts          "already completed by X.
                  Omit team_task_id to auto-create"
     │         │
     └────┬────┘
          ▼
7. ClaimTask(teamTaskID, targetAgentID)
   → pending → in_progress (ngay lập tức)
     │
     ▼
8. Concurrency checks:
   • ActiveCountForLink(src, tgt) vs link.MaxConcurrent (default 3)
   • ActiveCountForTarget(tgt) vs max_delegation_load (default 5)
     │ Pass
     ▼
9. Build DelegationTask struct
   SessionKey = "delegate:{srcUUID[:8]}:{targetKey}:{delegationID}"
   → Mỗi delegation có SESSION RIÊNG (empty history)
     │
     ▼
10. Resolve progressEnabled từ team settings hoặc global default
```

---

## 7. Flow Sync Delegation

```
prepareDelegation(ctx, opts, "sync")
     │
     ▼
1. active.Store(task.ID, task) — register
     │
     ▼
2. injectDependencyResults():
   Nếu task có blocked_by → fetch kết quả từ dependency tasks
   Prepend vào opts.Context:
   "--- Result from dependency task 'title' (id=X, by agentKey) ---"
     │
     ▼
3. buildDelegateMessage(opts)
   → "[Additional Context]\n{ctx}\n\n[Task]\n{task}"
     │
     ▼
4. EventDelegationStarted
     │
     ▼
5. Clear SenderID (WithSenderID(ctx, ""))
   → Delegate KHÔNG thừa kế group writer permissions
     │
     ▼
6. Propagate trace: WithDelegateParentTraceID
     │
     ▼
7. dm.runAgent(ctx, targetKey, runRequest) — BLOCKS
   ────── chờ member chạy xong ──────
     │
     ▼
8. Lỗi? → EventDelegationFailed + saveDelegationHistory
     │ OK
     ▼
9. applyQualityGates() — có thể re-run (xem Flow #11)
     │
     ▼
10. EventDelegationCompleted
     │
     ▼
11. trackCompleted(task) — session key vào pending cleanup
     │
     ▼
12. autoCompleteTeamTask(task, content, deliverables)
    → CompleteTask + flushCompletedSessions
     │
     ▼
13. saveDelegationHistory(task, content, nil, duration)
     │
     ▼
14. Return DelegateResult{Content: ...}
    → Calling agent nhận kết quả NGAY trong tool response

⏱ Sync = BLOCKING. Lead phải chờ member xong mới tiếp tục.
```

---

## 8. Flow Async Delegation

```
prepareDelegation(ctx, opts, "async")
     │
     ▼
1. taskCtx, taskCancel = context.WithCancel(Background())
   → Detach khỏi HTTP request context
   task.cancelFunc = taskCancel — cho phép Cancel()
     │
     ▼
2. active.Store(task.ID, task)
3. Capture parentTraceID (Background() sẽ mất)
     │
     ▼
4. EventDelegationStarted
     │
     ▼
5. Return NGAY: DelegateResult{DelegationID: task.ID}
   → Lead tiếp tục ngay, không chờ

   ═══════ GOROUTINE BẮT ĐẦU ═══════
     │
     ▼
6. Start progress ticker (mỗi 30s)
     │
     ▼
7. dm.runAgent(taskCtx, ...) — blocks trong goroutine
     │
     ▼
8. Stop progress ticker
     │
     ▼
9. Count siblings: ListActive(sourceAgentID)
     │
     ├── siblings > 0 (chưa phải cuối)
     │       │
     │       ▼
     │   accumulateArtifacts(sourceID, results)
     │   EventDelegationAccumulated
     │       {siblings_remaining: N}
     │   "announce suppressed"
     │
     └── siblings == 0 (cuối cùng!)
             │
             ▼
         Clear progressSent dedup
             │
             ▼
         collectArtifacts(sourceID)
         → Lấy tất cả kết quả tích lũy
             │
             ▼
         Merge kết quả của chính mình
             │
             ▼
         formatDelegateAnnounce():
         • Thành công: "--- Result from agentKey ---" + content
         • Thất bại toàn bộ: error + retry instructions
         • Cuối: "Auto-completed team tasks: [IDs]"
             │
             ▼
         EventDelegationAnnounce (WS)
             │
             ▼
         PublishInbound(SenderID="delegate:{taskID}")
         → Message Bus → gateway_consumer → LaneDelegate
         → Lead agent session nhận announce
             │
             ▼
10. applyQualityGates() (nếu không lỗi)
     │
     ▼
11. autoCompleteTeamTask() cho MỖI delegation
    (không chỉ cuối — mỗi cái có TeamTaskID riêng)
     │
     ▼
12. saveDelegationHistory()

   ═══════ GOROUTINE KẾT THÚC ═══════
```

---

## 9. Flow Dependency Injection (blocked_by)

```
Task B có blocked_by: [TaskA_ID]
Lead delegate Task B cho Member
     │
     ▼
injectDependencyResults(ctx, opts):
     │
     ▼
1. Lấy blocked_by IDs từ task
     │
     ▼
2. Với mỗi dependency:
   GetTask(depID)
   → Nếu có result:
     "--- Result from dependency task 'title'
      (id=X, by agentKey) ---
      {result[:8000]}"
     │
     ▼
3. Join bằng \n\n → prepend trước opts.Context
   → Dependencies đi TRƯỚC explicit context
     │
     ▼
4. Member nhận đủ context từ task trước
   mà KHÔNG cần gọi team_tasks.get
```

---

## 10. Flow Cancellation

### 10a. Cancel theo Delegation ID

```
Cancel(delegationID)
     │
     ▼
1. Load từ active sync.Map
2. task.cancelFunc() → cancel taskCtx
   → runAgent bị context cancellation
3. task.Status = "cancelled"
4. Delete from active
5. EventDelegationCancelled
```

### 10b. Cancel theo Team Task ID

```
CancelByTeamTaskID(teamTaskID)
     │
     ▼
Iterate tất cả active delegations
     │
     ▼
Tìm delegation có TeamTaskID match + Status="running"
     │ Found
     ▼
Cancel delegation đó (flow 10a)
Return ngay (chỉ cancel 1 cái đầu tiên)
```

### 10c. Cancel theo Origin (/stopall)

```
User gửi /stopall
     │
     ▼
CancelForOrigin(channel, chatID)
     │
     ▼
Iterate TẤT CẢ active delegations
     │
     ▼
Cancel mọi delegation match origin channel + chatID
     │
     ▼
Return count cancelled
```

### 10d. Cancel từ Task Board

```
Lead gọi team_tasks(action="cancel", task_id=X)
     │
     ▼
1. CancelTask() — DB: status=completed, result="CANCELLED: reason"
2. Unblock dependent tasks
3. delegateMgr.CancelByTeamTaskID(taskID) — stop running delegation
4. EventTeamTaskCancelled
```

---

## 11. Flow Quality Gate Evaluation

```
Delegation hoàn thành (sync hoặc async)
     │
     ▼
applyQualityGates()
     │
     ▼
Skip nếu: hookEngine==nil HOẶC SkipHooksFromContext==true
     │
     ▼
Parse quality_gates từ sourceAgent.OtherConfig:
{
  "quality_gates": [{
    "event": "delegation.completed",
    "type": "command" | "agent",
    "command": "sh script.sh",   // type=command
    "agent": "reviewer-agent",   // type=agent
    "block_on_failure": true,
    "max_retries": 2,
    "timeout_seconds": 60
  }]
}
     │
     ▼
Với mỗi gate (event=delegation.completed):
     │
     ▼
  attempt = 0..max_retries (inclusive, max_retries=2 → 3 attempts)
     │
     ▼
  hookEngine.EvaluateSingleHook(gate, context)
     │
     ├── PASSED → next gate
     │
     ├── FAILED + !block_on_failure → log warning, move on
     │
     ├── FAILED + block + attempt >= max_retries
     │     → "max retries exceeded, accepting result"
     │
     └── FAILED + block + retries remaining
           │
           ▼
         EventQualityGateRetry (WS)
           │
           ▼
         Build retry message:
         "[Quality Gate Feedback — Retry N/M]
          ...feedback from evaluator...
          Original task: ..."
           │
           ▼
         Re-run target agent với feedback
         dm.runAgent(ctx, targetKey, feedbackMsg)
         ⚠ KHÔNG re-run prepareDelegation!
           (skip validation, capacity check, task claim)
           │
           ▼
         Nếu re-run lỗi → accept previous result, break

──── EVALUATOR TYPES ────

CommandEvaluator:
  sh -c {command} < content (stdin)
  ENV: HOOK_EVENT, HOOK_SOURCE_AGENT, HOOK_TARGET_AGENT,
       HOOK_TASK, HOOK_USER_ID
  Exit 0 = PASS, non-zero = FAIL
  Stderr = feedback text
  Timeout: 60s default

AgentEvaluator:
  Delegate cho reviewer agent (sync)
  WithSkipHooks(ctx, true) ← ANTI-RECURSION
  Prompt: "[Quality Gate Evaluation]
           ...task/output/agents...
           Respond with APPROVED or REJECTED: <feedback>"
  Parse: startsWith("APPROVED") = pass
         contains("REJECTED:") → extract feedback
```

---

## 12. Flow Progress Notification

```
Async delegation đang chạy
     │
     ▼
Progress ticker fires (mỗi 30s)
     │
     ▼
sendProgressNotification():
     │
     ▼
Skip nếu: channel = delegate | system
     │
     ▼
Dedup check: progressSent["sourceID:chatID"]
  Đã gửi? → skip (1 notification/source/chat/tick)
     │
     ▼
Collect TẤT CẢ active delegations cùng source agent
  (không chỉ delegation hiện tại)
     │
     ▼
Format:
  ⏳ Your team is working on it...
  - DisplayName (agentKey): 45s
  - OtherAgent: 1m30s
     │
     ▼
Gửi qua channel gốc (Telegram/Discord/...)
     │
     ▼
EventDelegationProgress (WS):
  {active_delegations: [{delegation_id, target_agent_key, elapsed_ms}]}

Toggle:
  Global: DelegateManager.progressEnabled (default false)
  Per-team: team.settings.progress_notifications (override global)
  Reset: progressSent.Delete(key) khi last sibling hoàn thành
```

---

## 13. Flow Session Cleanup

```
Delegation hoàn thành
     │
     ▼
trackCompleted(task):
  completedMu.Lock()
  completedSessions = append(completedSessions, task.SessionKey)
  completedMu.Unlock()
     │
     ▼
autoCompleteTeamTask():
     │
     ├── TeamTaskID == nil → return (skip)
     │   ⚠ Non-team sessions TÍCH LŨY mà không cleanup!
     │
     └── TeamTaskID valid
           │
           ▼
         CompleteTask() + audit record
           │
           ▼
         flushCompletedSessions():
           Lock → drain → Unlock
           sessionStore.Delete(key) cho mỗi session
           │
           ▼
         Log: "flushed N delegation sessions"

Session key format: delegate:{srcUUID[:8]}:{targetKey}:{delegationID}
→ Mỗi delegation = session mới (empty history, isolated)
```

---

## 14. Flow Handoff (Chuyển giao agent)

```
Agent A đang phục vụ user trong chat
     │
     ▼
Agent A gọi: handoff(action="transfer", to="agent-b", reason="...")
     │
     ▼
1. Verify source agent trong context
2. Verify target agent tồn tại
3. Verify agent_link giữa source → target
     │
     ▼
4. GetSummary(sessionKey)
   → Lấy conversation summary (optional)
     │
     ▼
5. SetHandoffRoute(channel, chatID, from, to, reason)
   → UPSERT vào handoff_routes table
   → TẤT CẢ messages tới chat này sẽ đi tới agent B
     │
     ▼
6. EventHandoff (WS — cho UI switch displayed agent)
     │
     ▼
7. PublishInbound(SenderID="handoff:{handoffID}")
   AgentID = targetKey
   Content = "[Handoff from A]\n{reason}\n{session context}"
     │
     ▼
8. gateway_consumer: handoff: prefix
   → Route tới target agent session NGAY
   (không debounce, không dedupe)
     │
     ▼
Agent B nhận context, tiếp tục phục vụ user

──── PERSISTENCE ────

handoff_routes (channel, chat_id) → target agent
  → Persist qua server restart
  → Mọi message tới chat_id đều route tới target
  → Chỉ áp dụng khi msg.AgentID == "" (không explicit route)

──── CLEAR ────

handoff(action="clear")
  → ClearHandoffRoute(channel, chatID)
  → DELETE from handoff_routes
  → Messages trở lại agent mặc định
```

---

## 15. Flow Consumer Routing (Announce → Lead)

```
gateway_consumer.go — 4 special prefixes:

┌─────────────────────────────────────────────────────────┐
│ Prefix           │ Lane        │ Usage                  │
├─────────────────────────────────────────────────────────┤
│ subagent:{key}   │ LaneSubagent│ Subagent kết quả       │
│ delegate:{id}    │ LaneDelegate│ Delegation announce    │
│ handoff:{id}     │ LaneDelegate│ Handoff transfer       │
│ teammate:{key}   │ LaneDelegate│ Team mailbox message   │
└─────────────────────────────────────────────────────────┘

delegate:{delegationID} flow:
     │
     ▼
1. Rebuild session key từ origin metadata:
   origin_channel, origin_peer_kind, origin_chat_id, origin_local_key
     │
     ▼
2. overrideSessionKeyFromLocalKey():
   • :topic:N → Telegram forum topic
   • :thread:N → DM thread
   → Đảm bảo kết quả land đúng thread/topic
     │
     ▼
3. buildAnnounceOutMeta(localKey) — metadata cho reply routing
     │
     ▼
4. Goroutine với announceMu (per-session mutex):
   → Serialize announces cho cùng session
   → Prevent stale history reads
     │
     ▼
5. runAgent trong LaneDelegate lane (max 100 concurrent)
     │
     ▼
6. Suppress empty/NO_REPLY responses
7. Forward media từ delegation results
```

---

## 16. Flow Cache Invalidation

```
┌──────────────────────────────────────────────┐
│ Trigger                  │ Cache affected     │
├──────────────────────────────────────────────┤
│ teams.create/delete      │ Team + Agent caches│
│ teams.members.add/remove │ Team + Agent caches│
│ teams.update (settings)  │ Team cache only    │
│ agent_links CRUD         │ Agent cache only   │
└──────────────────────────────────────────────┘

Flow:
  RPC handler mutates DB
       │
       ▼
  emitTeamCacheInvalidate():
    msgBus.Broadcast(EventCacheInvalidate, CacheKindTeam)
       │
       ▼
  gateway_managed.go subscriber:
    teamMgr.InvalidateTeam()
    → Replace sync.Map with new empty one (FULL FLUSH)
       │
       ▼
  agentRouter.InvalidateAgent(key) cho affected agents:
    → Remove from Loop cache
    → Next request re-resolves from DB
    → Re-builds TEAM.md / DELEGATION.md
       │
       ▼
  TeamToolManager cache (5-min TTL):
    resolveTeam() cache-first → on miss query DB
    ⚠ Access control checked EVERY call kể cả cache hit
```

---

## 17. Flow Scheduler Lanes

```
┌──────────────────────────────────────────────┐
│ Lane       │ Concurrency │ Used for          │
├──────────────────────────────────────────────┤
│ main       │ 30          │ User chat messages│
│ subagent   │ 50          │ Subagent announces│
│ delegate   │ 100         │ Delegation + team │
│ cron       │ 30          │ Scheduled tasks   │
└──────────────────────────────────────────────┘

Configurable: GOCLAW_LANE_MAIN, _SUBAGENT, _DELEGATE, _CRON

⚠ Async runAgent() chạy trong RAW GOROUTINE
  → KHÔNG đi qua scheduler
  → Scheduler chỉ xử lý ANNOUNCE phase
    (khi kết quả quay về lead agent)

SessionQueue: 1 queue/session key
  → Serialize requests cho cùng session
  → Group chats: maxConcurrent=3 via ScheduleWithOpts

Shutdown: MarkDraining()
  → New requests → ErrGatewayDraining
  → Active runs hoàn thành bình thường
```

---

## 18. Flow Delegation History

```
Mọi delegation (sync/async, success/fail) đều lưu:
     │
     ▼
saveDelegationHistory() → INSERT delegation_history:
  • source_agent_id, target_agent_id
  • team_id, team_task_id (nullable)
  • user_id, task (text), mode (sync/async)
  • status (completed/failed)
  • result (nullable), error (nullable)
  • trace_id (nullable — linked qua OriginTraceID)
  • duration_ms
  • metadata (JSONB, default {})
  • created_at, completed_at

Query: ListDelegationHistory()
  Filter by: source, target, team, user, status
  Pagination: limit (max 200, default 50), offset

HTTP: /v1/delegations — DelegationsHandler
WS: delegations.list, delegations.get
```

---

## 19. Flow Task Board (Chi tiết)

```
                    ┌─────────┐
                    │ PENDING │ ◄── Lead tạo (hoặc auto-create)
                    └────┬────┘
                         │
              ┌──────────┼──────────┐
              ▼                     ▼
     ┌────────────┐          ┌──────────┐
     │ IN_PROGRESS│          │ BLOCKED  │
     │ (claimed)  │          │(chờ deps)│
     └──────┬─────┘          └────┬─────┘
            │                     │ ALL deps complete
            │                     ▼
            │               Auto → PENDING
            │
     ┌──────┴─────┐
     ▼            ▼
┌─────────┐ ┌───────────┐
│COMPLETED│ │ CANCELLED │
│(có kết  │ │(result=   │
│ quả)    │ │"CANCELLED:│
└─────────┘ │ reason")  │
            └───────────┘

──── Actions ────

create: Lead only. Optional: priority (int), blocked_by[]
  Status auto-set: blocked (nếu blocked_by) hoặc pending

claim: Atomic — UPDATE WHERE status='pending' AND owner IS NULL
  1 row updated = thành công, 0 rows = ai đó claim trước

complete: ⚠ delegate channel KHÔNG THỂ complete
  → Chỉ auto-complete qua DelegateManager
  Auto-claim trước (ignore lỗi nếu đã in_progress)
  CompleteTask → unblock dependent tasks trong cùng TX

cancel: ⚠ delegate channel KHÔNG THỂ cancel
  CancelTask → unblock dependents + CancelByTeamTaskID()

list: Default = active (pending + in_progress + blocked)
  delegate/system channel → thấy TẤT CẢ tasks
  User channel → chỉ thấy tasks mình trigger
  Cap: 20 tasks, has_more flag

get: Single task by UUID. Cross-team guard.
  Result truncated at 8000 runes

search: plainto_tsquery('simple', query) + ts_rank
  20 results, snippets 500 runes
  Same user filter logic
```

---

## 20. Flow Mailbox (Chi tiết)

```
──── SEND ────

team_message(action="send", to="member-key", message="...")
     │
     ▼
1. Validate recipient cùng team
   (ListMembers → O(n) scan)
     │
     ▼
2. INSERT team_messages:
   message_type='chat', to_agent_id=target
     │
     ▼
3. publishTeammateMessage():
   bus.InboundMessage{
     SenderID:  "teammate:{fromKey}",
     Channel:   "system",
     AgentID:   toKey,
     Content:   "[Team message from {fromKey}]: {text}",
     Metadata: {
       origin_channel, origin_peer_kind,
       from_agent, to_agent, origin_local_key
     }
   }
     │
     ▼
4. gateway_consumer → LaneDelegate → target agent session
5. EventTeamMessageSent (WS)

──── BROADCAST ────

Giống send nhưng:
  • to_agent_id = NULL
  • Gửi publishTeammateMessage cho TỪNG member (trừ self)
  • EventTeamMessageSent: ToAgentKey="broadcast"

──── READ ────

  • Lấy unread: WHERE (to_agent_id=$2 OR to_agent_id IS NULL)
    → Nhận cả DM lẫn broadcast
  • MarkRead() cho TỪNG message ngay lập tức
  • Returns SilentResult (không hiện cho user)

⚠ Khi teammate xử lý message và trả lời non-silent:
  → Response gửi về origin channel (Telegram/Discord...)
  → User thấy được → Lead cũng thấy
```

---

## 21. Flow Events (Real-time WebSocket) — Full Timeline

```
User gửi "Create social media campaign" → Lead
     │
     ▼
Lead tạo 2 tasks + delegate song song
     │
     ├── team.task.created (task_id=1, "Create Instagram post")
     ├── team.task.created (task_id=2, "Create Twitter thread")
     │
     ├── delegation.started (id=D1, target=designer, task_id=1)
     ├── delegation.started (id=D2, target=writer, task_id=2)
     │
     │   ═══ 30s later ═══
     ├── delegation.progress
     │     {active_delegations: [
     │       {id: D1, target: designer, elapsed: 30s},
     │       {id: D2, target: writer, elapsed: 30s}
     │     ]}
     │
     │   ═══ Designer finishes first (40s) ═══
     ├── delegation.completed (id=D1, elapsed: 40000ms)
     ├── team.task.completed (task_id=1)
     ├── delegation.accumulated
     │     {delegation_id: D1, siblings_remaining: 1}
     │
     │   ═══ Writer finishes (60s) ═══
     ├── delegation.completed (id=D2, elapsed: 60000ms)
     ├── team.task.completed (task_id=2)
     ├── delegation.announce
     │     {results: [
     │       {agent: designer, content: "..."},
     │       {agent: writer, content: "..."}
     │     ],
     │      completed_task_ids: [task_id=1, task_id=2]}
     │
     └── Lead nhận announce → tổng hợp → trả lời User

──── FULL EVENT CATALOG ────

Delegation lifecycle:
  delegation.started
  delegation.completed
  delegation.failed
  delegation.cancelled
  delegation.progress          (async, mỗi 30s)
  delegation.accumulated       (async, intermediate completion)
  delegation.announce          (async, last sibling)
  delegation.quality_gate.retry

Team tasks:
  team.task.created
  team.task.claimed
  team.task.completed
  team.task.cancelled

Team messages:
  team.message.sent

Team CRUD (admin):
  team.created, team.updated, team.deleted
  team.member.added, team.member.removed

Agent links:
  agent_link.created, agent_link.updated, agent_link.deleted

Handoff:
  handoff

Internal (pub/sub only, KHÔNG forward WS):
  cache.invalidate
```

---

## 22. Flow Access Control (Chi tiết)

```
User gửi tin → Lead
     │
     ▼
┌───────────────────────────────────────────┐
│ TEAM-LEVEL ACCESS (checkTeamAccess)       │
│                                           │
│ delegate/system channel → ALWAYS PASS     │
│                                           │
│ DenyUserIDs (ưu tiên cao nhất):          │
│   user trong deny list → REJECT           │
│                                           │
│ AllowUserIDs (nếu set):                  │
│   user KHÔNG trong allow list → REJECT    │
│                                           │
│ DenyChannels:                             │
│   channel trong deny list → REJECT        │
│                                           │
│ AllowChannels (nếu set):                 │
│   channel KHÔNG trong allow list → REJECT │
│                                           │
│ Empty/malformed settings → PASS (fail open)│
└────────────────┬──────────────────────────┘
                 │ Pass
                 ▼
Lead delegate → Member
     │
     ▼
┌───────────────────────────────────────────┐
│ LINK-LEVEL ACCESS (checkUserPermission)   │
│                                           │
│ UserDeny: user trong deny → REJECT        │
│ UserAllow (nếu set):                     │
│   user KHÔNG trong allow → REJECT         │
└────────────────┬──────────────────────────┘
                 │ Pass
                 ▼
┌───────────────────────────────────────────┐
│ CONCURRENCY LIMITS                        │
│                                           │
│ Per-link: ActiveCountForLink vs           │
│   link.MaxConcurrent (default 3)          │
│   "Too many active delegations to X (3/3)"│
│                                           │
│ Per-agent: ActiveCountForTarget vs        │
│   max_delegation_load from OtherConfig    │
│   (default 5)                             │
│   "Agent at capacity (5/5). Try a         │
│    different agent or handle it yourself." │
└────────────────┬──────────────────────────┘
                 │ Pass
                 ▼
            Delegation chạy
```

---

## 23. Flow Tracing (Delegation Chain)

```
User request → Trace T1
     │
     ▼
┌──────────────────────────┐
│ Trace T1                 │
│ Span: agent (root)       │
│   ├── llm_call           │
│   ├── tool_call (spawn)  │
│   │     parent_trace_id=T1 propagated
│   └── ...                │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────────────┐
│ Trace T2                 │
│ parent_trace_id = T1     │
│ Span: agent (root)       │
│   ├── llm_call           │
│   ├── tool_call (web)    │
│   └── ...                │
└──────────────────────────┘

delegation_history.trace_id = T2
  → Link: T2.parent_trace_id = T1
  → Full chain: User → Lead (T1) → Member (T2)
  → Nested: Lead → Member A → Member A's subagent (T3)

Trace storage: async buffer (cap 1000) → flush every 5s
  → BatchCreateSpans() + optional OTel export
```

---

## 24. Edge Cases & Lưu ý quan trọng

### Auto-create Task (Race Prevention)
LLM hay hallucinate task_id khi gọi spawn+create song song. Fix: `prepareDelegation` auto-create task khi `TeamTaskID==nil` → loại bỏ two-step dance.

### Cross-group Task Leak Prevention
Khi `channel != delegate/system` và `teamTask.UserID != currentUserID` → reject. Agent trong group A không thể dùng task tạo bởi group B.

### Completed Task Reuse Prevention
Delegate tới task đã completed/cancelled → error: "already completed by X. Omit team_task_id to auto-create a new task."

### Lead Tool Policy
`agentToolPolicyForTeam` thêm `team_message` vào deny list của lead. Lead PHẢI dùng `spawn` (có announce), không được dùng `team_message` (one-way).

### Delegate Channel Guards
`team_tasks.complete` và `team_tasks.cancel` REJECT từ delegate channel. Chỉ DelegateManager auto-complete xử lý.

### Bootstrap Skip
Nếu `BOOTSTRAP.md` tồn tại → skip TEAM.md, DELEGATION.md, AVAILABILITY.md injection → tránh token waste first-run.

### Session Isolation
Mỗi delegation = session mới: `delegate:{src[:8]}:{target}:{id}`. Empty history, không context pollution từ delegation trước.

### Non-team Session Orphan
`trackCompleted` add sessions nhưng `flushCompletedSessions` chỉ chạy khi `TeamTaskID != nil`. Non-team delegation sessions tích lũy cho đến khi một team task hoàn thành hoặc server restart.

### Quality Gate Retry Scope
Retry KHÔNG re-run `prepareDelegation`. Skip validation, capacity check, task claim. Chỉ gọi lại `runAgent` với feedback.

### Handoff Route Persistence
`handoff_routes` persist qua server restart. Routes từ crashed sessions có thể tồn tại indefinitely (không TTL).

---

## Cross-References

| Document | Nội dung liên quan |
|----------|--------------------|
| [01-agent-loop.md](./01-agent-loop.md) | System prompt assembly, TEAM.md injection (section 19), PromptMinimal |
| [03-tools-system.md](./03-tools-system.md) | Delegation system, agent links, quality gates, evaluate loop |
| [04-gateway-protocol.md](./04-gateway-protocol.md) | teams.* RPC methods, agents.links.*, delegations.*, projects.* |
| [06-store-data-model.md](./06-store-data-model.md) | Team tables schema, delegation_history, handoff_routes |
| [08-scheduling-cron.md](./08-scheduling-cron.md) | Delegate lane (concurrency 100), SessionQueue |
| [09-security.md](./09-security.md) | Delegation security, hook recursion prevention |
| [11-agent-teams.md](./11-agent-teams.md) | Team model, TEAM.md generation, access control |
| [13-ws-team-events.md](./13-ws-team-events.md) | Full WS event reference + payloads |

## File Reference

| File | Purpose |
|------|---------|
| `internal/tools/team_tool_manager.go` | Shared backend, team cache (5-min TTL), access control |
| `internal/tools/team_tasks_tool.go` | Task board: create/claim/complete/cancel/list/get/search |
| `internal/tools/team_message_tool.go` | Mailbox: send/broadcast/read, real-time routing |
| `internal/tools/delegate.go` | DelegateManager: lifecycle, auto-complete, history |
| `internal/tools/delegate_prep.go` | prepareDelegation, dependency injection, progress |
| `internal/tools/delegate_sync.go` | Sync delegation: blocking, quality gates |
| `internal/tools/delegate_async.go` | Async delegation: goroutine, artifact batching |
| `internal/tools/delegate_state.go` | Active tracking, artifacts, session cleanup, cancel |
| `internal/tools/delegate_policy.go` | Access control, concurrency, quality gates |
| `internal/tools/delegate_events.go` | Event broadcasting |
| `internal/tools/handoff_tool.go` | Agent handoff: transfer, clear, route persistence |
| `internal/gateway/methods/teams.go` | Team RPC registration |
| `internal/gateway/methods/teams_crud.go` | Create/update/delete team |
| `internal/gateway/methods/teams_members.go` | Add/remove members, auto-link, cache invalidation |
| `internal/agent/resolver.go` | buildTeamMD, AVAILABILITY.md, agent resolution |
| `internal/hooks/engine.go` | Quality gate engine |
| `internal/hooks/command_evaluator.go` | Shell command evaluator |
| `internal/hooks/agent_evaluator.go` | Agent-based evaluator (anti-recursion) |
| `internal/scheduler/scheduler.go` | 4-lane scheduler, SessionQueue, draining |
| `internal/store/team_store.go` | TeamStore interface (22 methods) |
| `internal/store/pg/teams.go` | Team CRUD PostgreSQL |
| `internal/store/pg/teams_tasks.go` | Task board PostgreSQL (atomic claim, unblock) |
| `internal/store/pg/teams_delegation.go` | Delegation history PostgreSQL |
| `internal/store/pg/teams_messaging.go` | Mailbox PostgreSQL |
| `cmd/gateway_consumer.go` | Routing: delegate:/teammate:/handoff:/subagent: prefixes |
| `cmd/gateway_managed.go` | Wiring: team tools, cache subscribers, hook engine |
| `pkg/protocol/events.go` | Event constants |
| `pkg/protocol/team_events.go` | Typed event payloads |
