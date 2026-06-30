;;;; REFINEMENT-PLAN.lisp
;;;; local-scava — Refinement Plan (Phase 3 polish + sprint-as-mother)
;;;; Author: paul kinyatti   Date: 2026-06-30
;;;;
;;;; This is a SPEC / PROMPT document as s-expressions. Nothing here is
;;;; implemented yet. It mirrors REFINEMENT-PLAN.org (same content). Feed an
;;;; (epic ...) or single (task ...) back to the agent as the implementation
;;;; prompt. Verify every :file path before editing — paths reflect the repo
;;;; at 2026-06-30.
;;;;
;;;; Form vocabulary:
;;;;   (refinement-plan ...)            top-level
;;;;   (context ...)                    ground truth about the codebase
;;;;   (epic :id :title :tags ...)      a theme of work
;;;;   (task :id :status :title ...)    one unit; :status in {todo in-progress blocked done}
;;;;     :why         one line motivation (from the user's request)
;;;;     :current     what the code does today
;;;;     :target      desired behaviour (list of points)
;;;;     :files       paths to touch
;;;;     :acceptance  done-when checks

(refinement-plan
 :project "local-scava"
 :date "2026-06-30"
 :mirror "REFINEMENT-PLAN.org"

 ;; ------------------------------------------------------------------
 (context
  :stack "Go 1.25, libSQL(cgo), html/template + htmx, goose migrations, embed.FS single binary"
  :ui "Grafana-style dark dashboard, server-rendered, loopback-only 127.0.0.1:3000"

  (architecture
   (layer :domain     "internal/domain"     "entities.go, enums.go — no SQL/HTTP")
   (layer :store      "internal/store"      "libSQL queries + goose migrations 0001..0009")
   (layer :service    "internal/service"    "sprint health, cadence, metrics, traces")
   (layer :block      "internal/block"      "registry.go, service.go — per-user enable/disable")
   (layer :auth       "internal/auth"       "login, sessions, API keys, middleware")
   (layer :onboarding "internal/onboarding" "3-step wizard role->blocks->confirm")
   (layer :bridge     "internal/bridge"     "WebSocket <-> kiro-cli chat proxy")
   (layer :export     "internal/export"     "server-side PDF (fpdf)")
   (layer :web        "internal/web"        "handlers, html/template pages, static, REST API v1"))

  (block-registry  ; the "KYC blocks" — internal/block/registry.go
   (block :key "sprint"  :group "Sprints" :default-on t   :routes ("/sprints" "/sprints/"))
   (block :key "adr"     :group "Sprints" :default-on nil :routes ("/adrs" "/adrs/"))
   (block :key "logs"    :group "Cadence" :default-on t   :routes ("/logs"))
   (block :key "posts"   :group "Cadence" :default-on nil :routes ("/cadence" "/posts/"))
   (block :key "todo"    :group "Act"     :default-on nil :routes ("/todos"))
   (block :key "traces"  :group "Watch"   :default-on nil :routes ("/traces"))
   (block :key "metrics" :group "Watch"   :default-on t   :routes ("/metrics"))
   (block :key "habits"  :group "Watch"   :default-on nil :routes ("/habits"))
   (block :key "review"  :group "Act"     :default-on nil :routes ("/review"))
   (note "SetBlocks already disables every non-selected key. IsRouteEnabled exists "
         "but must be confirmed as wired into request gating — see epic C."))

  (design-tokens  ; internal/web/static/app.css
   (note "Radii flat (--r-xs..--r-xxl:2px) EXCEPT --r-pill:9999px.")
   (note ".btn is already pill; .btn-primary = blue.")
   (note ".card has border:1px solid var(--line)+shadow — THIS is the border the sign-in page must drop.")
   (note "Alert system: .alert-bar variants success|warning|critical|info; 'critical overdue' adds pulsing .alert-dot.pulse.")
   (note "Semantic colors: green=healthy, yellow/orange=watch, red=act."))

  (severity-enum  ; internal/domain/enums.go (exists)
   :values (success warning alert alarm nudge)
   :note "Map to .alert-bar variants per phase2/04-sprint-rules.md."))

 ;; ==================================================================
 (epic :id "A" :title "Sign-in page: borderless, pill, localStorage" :tags (auth)

  (task :id "A1" :status todo :title "Borderless sign-in card, pill buttons"
   :why "User wants a cleaner, borderless sign-in using the pill button style."
   :current "login.html wraps the form in .card (1px line border + shadow); inputs use default 1px border; button is .btn .btn-primary (already pill)."
   :target ("Drop card border+shadow on the sign-in container (remove .card or add .card-borderless: border:none; box-shadow:none; background:transparent)."
            "Inputs: borderless / underline-only, keep a focus affordance."
            "All actionable buttons use pill style; primary CTA full-width pill."
            "Keep centered max-width 400px chromeless layout.")
   :files ("internal/web/templates/pages/login.html"
           "internal/web/static/app.css")
   :acceptance ("No visible card border/box outline on sign-in."
                "Primary CTA is a full-width pill; secondary actions are pills/links."))

  (task :id "A2" :status todo :title "Persist user details to localStorage on sign-in"
   :why "'when user signs in, just store the details in local storage of the browser.'"
   :current "Auth is server-side cookie session only (POST /login -> handleLoginSubmit). Nothing written to localStorage."
   :target ("On successful login persist JSON to localStorage key 'scava-user': {username, displayName, role, avatarInitials, enabledBlocks[]}."
            "enabledBlocks lets the client tailor chat/new-entry actions without a round-trip."
            "Pick ONE: (a) server injects a <script> on the post-login target that writes the blob; or (b) GET /api/me JSON endpoint the client caches."
            "Clear 'scava-user' on logout (POST /logout)."
            "Additive only — cookie session stays the source of truth for auth.")
   :files ("internal/web/auth_handlers.go"
           "internal/web/templates/layout.html"
           "internal/web/api.go (if GET /api/me)"
           "internal/block/service.go (enabled keys)")
   :acceptance ("After login localStorage 'scava-user' holds the blob with enabledBlocks = KYC selection."
                "After logout the key is removed.")))

 ;; ==================================================================
 (epic :id "B" :title "KYC / Onboarding look & feel" :tags (onboarding)

  (task :id "B1" :status todo :title "Improve onboarding visual design"
   :why "'the KYC needs to be improved the way it looks and feels.'"
   :current "onboarding_role/blocks/confirm use mostly inline styles; blocks step is a 2-col checkbox grid with a hidden platforms sub-step."
   :target ("Cohesive wizard: consistent step indicator, card-based selectable blocks (icon+name+description), pill CTAs."
            "Selected/unselected states obvious (selected = orange accent / filled card driven by the checkbox)."
            "Keep chromeless; align to 4px grid + design tokens.")
   :files ("internal/web/templates/pages/onboarding_role.html"
           "internal/web/templates/pages/onboarding_blocks.html"
           "internal/web/templates/pages/onboarding_confirm.html"
           "internal/web/static/app.css")
   :acceptance ("Wizard reads as designed (not raw inline styles); selectable cards show clear selected state; CTAs are pills.")))

 ;; ==================================================================
 (epic :id "C" :title "Strict block selection + gating" :tags (blocks critical)

  (task :id "C1" :status todo :title "Never show unselected blocks anywhere"
   :why "'be really strict ... do not give me blocks that I did not select.' 'the blocks I selected are the ones shown in the website.'"
   :current "SetBlocks disables non-selected keys; buildSidebar filters by enabledKeys. Must confirm overview/new/chat/palette also surface enabled-only."
   :target ("Sidebar: enabled-only (verify; already implemented)."
            "Overview '/': a block's cards/sections render only if enabled."
            "Command palette + chat quick-actions + /new actions: enabled-only (see epics O, P)."
            "Onboarding confirm: shows exactly the selected set.")
   :files ("internal/web/handlers.go (buildSidebar, handleOverview)"
           "internal/web/templates/pages/overview.html"
           "internal/web/templates/pages/new.html"
           "internal/web/templates/layout.html"
           "internal/web/static/palette.js")
   :acceptance ("With only {sprint,logs,metrics} selected, no adr/posts/todo/habits/review/traces UI appears in sidebar, overview, /new, chat, or palette."))

  (task :id "C2" :status todo :title "Disabled block routes return a proper error page"
   :why "'the others will show an error.'"
   :current "block.Service.IsRouteEnabled(path) returns (key,enabled,err) but is NOT visibly wired as middleware in web.go/Mount; hitting /adrs while adr disabled still renders."
   :target ("Add a gating layer: for each block-owned route, if the owning block is disabled, respond with a dedicated 'block disabled' page (403 or 404 — decide+document) linking to Settings -> Blocks. Non-block routes pass through."
            "Wrap dashboard handlers (or add mux middleware) using IsRouteEnabled.")
   :files ("internal/web/web.go (Mount/middleware)"
           "internal/block/service.go (IsRouteEnabled — present)"
           "internal/web/templates/pages/block_disabled.html (new)")
   :acceptance ("Visiting a disabled block's route shows the error page, not the feature; enabled routes unaffected.")))

 ;; ==================================================================
 (epic :id "D" :title "Sidebar count badges" :tags (sidebar)

  (task :id "D1" :status todo :title "Cart-style count badges on sidebar icons"
   :why "'show a number on top of the sprint ... not beside it but on top of it, the way we have a cart icon.'"
   :current "block.Service.Metrics() returns human strings used on Settings cards. buildSidebar renders plain nav items with no badge."
   :target ("Compute per-block counts (sprints, adrs, logs, posts, todos open, habits)."
            "Attach a numeric badge at the TOP-RIGHT corner of each sidebar item icon (absolute-positioned superscript, cart-style)."
            "Badge shows only when count > 0; small pill, accent color, on top of the glyph (not inline text).")
   :files ("internal/web/handlers.go (SidebarItem +Count int; buildSidebar fills from block counts)"
           "internal/web/templates/layout.html (badge markup)"
           "internal/web/static/app.css (.nav-badge absolute positioning)")
   :acceptance ("With 3 sprints the Sprints icon shows a '3' badge at its top-right; zero-count blocks show no badge.")))

 ;; ==================================================================
 (epic :id "E" :title "Sprint rules, dates, reminders, sprint-as-mother" :tags (sprint core)

  (task :id "E1" :status todo :title "Many sprints, one RUNNING sprint per ISO week"
   :why "'we can create more than one sprints, but we can only have one running sprint per week.'"
   :current "migration 0005 adds ux_sprints_single_active (one active sprint globally). phase2/04 documents single-active + ErrActiveSprintExists. No week-scoped rule."
   :target ("Allow unlimited sprints in draft/planned."
            "Enforce at most ONE active ('running') sprint per ISO week: starting a sprint is rejected if another is already active in the same ISO week."
            "Decide + DOCUMENT: keep global single-active OR relax to per-week."
            "New sentinel error ErrActiveSprintThisWeek, inline copy: 'You already have a running sprint this week. Finish or archive it before starting another.'")
   :files ("internal/service/sprint.go (Start/Create checks)"
           "internal/store/sprints.go"
           "internal/store/migrations/ (new migration if index changes)")
   :acceptance ("Can create N sprints; starting a 2nd active sprint in the same ISO week is rejected; a sprint in a different week can start."))

  (task :id "E2" :status todo :title "Sprint dates/times, progress checks, color-coded reminders"
   :why "'have dates and times that check progress ... give reminders or notification with the different alert colors we have in the system.'"
   :current "Sprint has StartsOn/EndsOn/DurationDays. Health computed on page load uses ONLY per-phase checklist gut-check (unchecked>2 -> red). The time-vs-deliverables SprintHealth from phase2/04 is specified but not wired."
   :target ("Implement computed SprintHealth{Severity,ElapsedPct,DeliverablesPct,DaysRemaining,IncompleteCount,Message} from starts_on->ends_on vs deliverables-done, thresholds per phase2/04:"
            "  >=50% time & <25% deliverables -> warning"
            "  >=80% time & <50% deliverables -> alert (at risk)"
            "  deadline passed & incomplete   -> alarm (overdue, pulsing dot)"
            "  no active sprint & last ended >7d -> nudge"
            "  else -> success"
            "Render a health .alert-bar at top of /sprints/{id} AND a sprint-health card on Overview, using system alert colors (overdue pulses)."
            "Reminders computed on read (no background worker — local-first). Optionally feed a count to sidebar badge / topbar.")
   :files ("internal/service/sprint.go (Health method)"
           "internal/domain (SprintHealth exists)"
           "internal/web/handlers.go (handleSprintDetail, handleOverview)"
           "internal/web/templates/pages/sprint_detail.html"
           "internal/web/templates/pages/overview.html")
   :acceptance ("A sprint past 80% time with <50% deliverables shows the red 'at risk' alert with the right copy on sprint page + Overview; overdue pulses."))

  (task :id "E3" :status todo :title "Sprint is the mother: nest adr/posts/logs/traces (KYC-gated)"
   :why "'a sprint is the mother of many ... has adr, posts, logs traces — they will be there if we included them during the kyc blocks selection.'"
   :current "DailyLog/Post/ADR/TraceSpan all carry optional SprintID. Sprint detail shows checklist+logs+retro only; no adr/posts/traces sections; not gated by enabled blocks."
   :target ("On /sprints/{id} render child sections for ENABLED blocks only:"
            "  logs   -> daily logs stream (exists)"
            "  adr    -> ADRs linked to this sprint (list + create-linked)"
            "  posts  -> posts linked to this sprint (list + generate, see F3)"
            "  traces -> the sprint trace waterfall (exists; gate by traces block)"
            "  todo   -> this sprint's phase checklist (see L2) + linked todos"
            "Disabled blocks' sections do NOT appear on the sprint page.")
   :files ("internal/web/handlers.go (handleSprintDetail loads children + enabled set)"
           "internal/service/* (ListADRsBySprint/ListPostsBySprint — verify)"
           "internal/web/templates/pages/sprint_detail.html")
   :acceptance ("A sprint shows exactly the child sections for KYC-selected blocks; disabling a block removes its section.")))

 ;; ==================================================================
 (epic :id "F" :title "Sprint cadence: daily entries + generated post" :tags (sprint posts)

  (task :id "F1" :status todo :title "Sprint = weekly; logs/retro/adr = daily"
   :why "'the sprint is weekly; the logs, retro, adr should be daily.'"
   :current "Sprint duration 3/5/7/14d. DailyLog is per-day. Retro is one per-sprint block (RetroWorked/Differently/Learned/LiveLink). ADRs per-sprint, undated-by-day."
   :target ("Treat sprint as the weekly container; within it present a per-DAY entry surface: each day has its log, a retro note, and ADRs decided that day."
            "Add a daily-retro concept (new dated retro note OR reuse DailyLog fields) — DOCUMENT the data choice."
            "Minimal path: a 'daily entry' view grouping by date the day's log(s), retro note, and ADRs.")
   :files ("internal/domain/entities.go (DailyEntry view model or dated retro)"
           "internal/store/*"
           "internal/service/sprint.go"
           "internal/web/templates/pages/sprint_detail.html")
   :acceptance ("Sprint page shows day-grouped entries (log + retro + adr) per day."))

  (task :id "F2" :status todo :title "Show yesterday / last-2-days entries"
   :why "'display the details of yesterday's details or the last 2 days.'"
   :current "handleSprintDetail loads all logs for the sprint; no day-windowed view."
   :target ("Add a 'Recent' panel on the sprint page showing yesterday + prior day combined entries (log + retro + adr) for continuity.")
   :files ("internal/service/sprint.go (RecentDailyEntries(sprintID,days))"
           "internal/web/handlers.go"
           "internal/web/templates/pages/sprint_detail.html")
   :acceptance ("Panel shows last 2 days' entries; empty-state when none."))

  (task :id "F3" :status todo :title "'Generate today's post' from the day's entries via kiro-cli"
   :why "'read the daily entry of the present blocks for the sprint on that day (logs, retro, adr) ... button generate today's post ... use kiro-cli to generate this LinkedIn post ... save and display the post.'"
   :current "Posts exist with tiers; bridge.go proxies kiro-cli over WebSocket; post detail surfaces week material for recaps. No generate action."
   :target ("Add 'Generate today's post' button on the sprint daily surface. On click:"
            "  1. Gather today's present-block entries for the sprint (logs+retro+adr), enabled blocks only."
            "  2. Send a structured prompt to kiro-cli (via bridge) for a LinkedIn-tier post drafted from those entries."
            "  3. Persist as a Post (+ linkedin PostTier, status drafted) linked to sprint + source log."
            "  4. Render the generated post inline + link to its detail page."
            "Decide sync vs async (WS streaming vs request/response) and DOCUMENT. Gate on the posts block.")
   :files ("internal/bridge/bridge.go (or a service that calls kiro-cli)"
           "internal/service/content.go (CreatePost)"
           "internal/store/posts.go"
           "internal/web/web.go (POST /sprints/{id}/generate-post or /posts/generate)"
           "internal/web/templates/pages/sprint_detail.html"
           "internal/web/templates/pages/post_detail.html")
   :acceptance ("With posts enabled + a day's entries, Generate produces a saved LinkedIn draft built from that day's log/retro/adr and displays it; disabled posts hides the button.")))

 ;; ==================================================================
 (epic :id "G" :title "ADRs: full title, document detail page, PDF" :tags (adr)

  (task :id "G1" :status todo :title "Full title on ADR list page"
   :why "'give it the full name at the top ... say architecture decision record.'"
   :current "handleADRList title = 'ADRs'; adrs.html header likely 'ADRs'."
   :target ("/adrs H1 reads 'Architecture Decision Records'; keep 'ADR' only as a compact tag where space-constrained.")
   :files ("internal/web/handlers.go (handleADRList title)"
           "internal/web/templates/pages/adrs.html")
   :acceptance ("/adrs heading reads 'Architecture Decision Records'."))

  (task :id "G2" :status todo :title "ADR detail page rendered like a document + Export PDF"
   :why "'when I select an adr, take me to a page ... render it like a pdf or markdown editor ... option for export pdf there.'"
   :current "Routes exist for POST /adrs/{id}/update, /delete, GET /adrs/{id}/export.pdf, but NO GET /adrs/{id} detail page/template. List is the only ADR view."
   :target ("Add GET /adrs/{id} -> document-style page rendering the ADR (Number, Title, Status, DecidedOn, Problem, Options, Decision, Why, Consequences) as a clean 'paper' layout: markdown-rendered body, generous typography, max-width column (best markdown render with existing stack; a lightweight client-side md renderer is fine, no heavy deps)."
            "Place 'Export PDF' button wired to existing GET /adrs/{id}/export.pdf."
            "ADR list rows link to this detail page.")
   :files ("internal/web/web.go (register GET /adrs/{id})"
           "internal/web/handlers.go (handleADRDetail)"
           "internal/web/templates/pages/adr_detail.html (new)"
           "internal/web/templates/pages/adrs.html (link rows)")
   :acceptance ("Clicking an ADR opens a document-style page; Export PDF downloads the existing server-generated PDF.")))

 ;; ==================================================================
 (epic :id "H" :title "Posts / cadence page" :tags (posts)
  (task :id "H1" :status todo :title "/cadence displays correctly"
   :why "'make sure /cadence works as it should — it displays information as it should.'"
   :current "handleCadence loads heatmap+posts+30d rate into cadence.html."
   :target ("Verify 90d heatmap renders, posts list shows tier status, rate shows; fix empty/broken rendering; gate on posts block.")
   :files ("internal/web/handlers.go (handleCadence)"
           "internal/web/templates/pages/cadence.html"
           "internal/service/content.go")
   :acceptance ("/cadence shows populated heatmap + posts list + cadence rate, no template errors.")))

 ;; ==================================================================
 (epic :id "I" :title "Logbook" :tags (logs)
  (task :id "I1" :status todo :title "Logbook displays information nicely"
   :why "'even the logbook display information there nicely.'"
   :current "handleLogbook loads 200 events + 200 logs into logbook.html."
   :target ("Present career events (audit trail) and daily logs as clean SRE-style streams (mono timestamp, kind tag, summary); fix density/empty states; gate on logs block.")
   :files ("internal/web/handlers.go (handleLogbook)"
           "internal/web/templates/pages/logbook.html")
   :acceptance ("/logs shows a tidy two-stream (events + daily logs) view.")))

 ;; ==================================================================
 (epic :id "J" :title "Traces" :tags (traces)
  (task :id "J1" :status todo :title "Traces page works"
   :why "'make sure the traces page works.'"
   :current "traces block declares route /traces + API /api/v1/traces, but web.go Mount has NO GET /traces handler (only /sprints/{id} renders an inline trace). Sidebar links to /traces -> likely 404."
   :target ("Register GET /traces with a handler rendering the sprint(s) as phase-waterfall spans (reuse sprintTraceSVG / service.SprintTrace) + API-pushed TraceSpans. Add traces.html. Gate on traces block.")
   :files ("internal/web/web.go (route)"
           "internal/web/handlers.go (handleTraces)"
           "internal/web/templates/pages/traces.html (new)"
           "internal/service/traces.go")
   :acceptance ("/traces renders without 404 and shows the waterfall(s).")))

 ;; ==================================================================
 (epic :id "K" :title "Metrics (Grafana-style)" :tags (metrics)
  (task :id "K1" :status todo :title "All charts/graphs working; SRE/Grafana feel"
   :why "'the metrics — make sure all the charts and graphs are working ... look and feel like grafana, with sre features.'"
   :current "handleMetrics wires cadence 7/30/90, streaks, ship rate, tier mix/goals, daily post/log counts, sprint uptime, content stats, recent activity into metrics.html via SVG helpers (sparkline,barGauge,areaChart,uptime,progressRing,tierDonut,activityFeed)."
   :target ("Audit every panel renders with real data (no blank SVGs / divide-by-zero)."
            "Ensure Grafana layout (stat panels, time-series area charts, bar gauges, uptime strip); fix broken helper output."
            "Add SRE touches (uptime %, error/at-risk indicators) consistent with the alert color system.")
   :files ("internal/web/charts.go (SVG helpers)"
           "internal/web/handlers.go (handleMetrics)"
           "internal/web/templates/pages/metrics.html"
           "internal/service/metrics.go")
   :acceptance ("Every metrics panel shows correct data; no empty/broken charts; layout reads like a Grafana board.")))

 ;; ==================================================================
 (epic :id "L" :title "Todos + phase-checklist sync" :tags (todo)

  (task :id "L1" :status todo :title "Todos selectable in KYC + clean CRUD page"
   :why "'the todos should also be fixed nicely (all the way from the kyc — I can select it).'"
   :current "todo block is in the registry (selectable). Routes: GET /todos, POST /todos, POST /todos/{id}/status, /delete. todos.html exists."
   :target ("Verify todo block is selectable in onboarding; /todos offers clean CRUD (create with priority/due/sprint-link, toggle status, delete); polish layout; gate on todo block.")
   :files ("internal/web/templates/pages/onboarding_blocks.html"
           "internal/web/handlers.go (todo handlers)"
           "internal/web/templates/pages/todos.html"
           "internal/store/todos.go")
   :acceptance ("todo block toggles on in KYC; /todos supports full CRUD cleanly."))

  (task :id "L2" :status todo :title "Phase checklists on /todos, grouped by sprint, two-way synced"
   :why "'when we create a sprint it has the phase checklist ... have them on the todo page grouped by the sprint ... check from either the todo page or the sprint page -> update completed.'"
   :current "ChecklistItem (per sprint, per phase) is seeded per sprint and toggled via shared POST /checklist/{id}/toggle. Sprint detail renders it grouped by phase. /todos does NOT show phase checklists."
   :target ("On /todos render each sprint's phase checklist as a grouped section (group header = sprint title/skill; sub-groups = phases), alongside regular todos."
            "Reuse the SAME POST /checklist/{id}/toggle action so toggling from either page updates the one underlying ChecklistItem (single source of truth; both views reflect completion).")
   :files ("internal/web/handlers.go (handleTodos loads checklists per sprint)"
           "internal/web/templates/pages/todos.html (grouped checklists + reuse toggle form)"
           "internal/service/sprint.go (Checklist queries — exist)")
   :acceptance ("Checking an item on /todos marks it done on the sprint page and vice versa; checklists are grouped by sprint on /todos.")))

 ;; ==================================================================
 (epic :id "M" :title "Habits (sprint strip)" :tags (habits)
  (task :id "M1" :status todo :title "Second habit strip scoped to the sprint"
   :why "'the habits should also have a second strip where it shows my habits when it comes to the sprint.'"
   :current "Habit has SprintLinked bool. /habits renders a heatmap + streaks. phase2/08 says linked habits surface on sprint detail as a streak indicator."
   :target ("On /habits render TWO strips: (1) all daily habits (existing); (2) a second strip filtered to SprintLinked habits ('sprint habits') tied to the active sprint, showing streak/heatmap in sprint context."
            "Also surface sprint-linked habit streaks on the sprint detail page.")
   :files ("internal/web/handlers.go (handleHabits)"
           "internal/web/templates/pages/habits.html"
           "internal/web/templates/pages/sprint_detail.html"
           "internal/store/habits.go (filter by sprint_linked)")
   :acceptance ("/habits shows a general strip and a separate sprint-habit strip; sprint page shows linked-habit streaks.")))

 ;; ==================================================================
 (epic :id "N" :title "Settings (admin details + password)" :tags (settings)
  (task :id "N1" :status todo :title "Settings shows admin profile + password change"
   :why "'fix the setting page to show the details of the admin, the password if I want to change them.'"
   :current "handleSettings loads data.User (display name, username, role, initials) and block cards with metrics; POST /settings/profile and POST /settings/password exist. Must confirm settings.html renders profile + a working password form."
   :target ("Settings shows admin details (username, display name, role, avatar initials) editable via profile form, plus password-change form (current+new+confirm) wired to POST /settings/password."
            "Keep blocks-toggle section + daemon info (Meta). Polish per design system.")
   :files ("internal/web/templates/pages/settings.html"
           "internal/web/handlers.go (handleSettings, handleProfileUpdate, handlePasswordChange; verify ?msg/?err feedback)")
   :acceptance ("Admin can see/edit profile and change password from /settings with success/error feedback.")))

 ;; ==================================================================
 (epic :id "O" :title "Chat (kiro-cli): copy + block-aware actions" :tags (chat)

  (task :id "O1" :status todo :title "Chat welcome + connection + sprint prompt copy"
   :why "User specified exact chat copy."
   :current "layout.html chat welcome = 'Hey! I'm your build assistant. What are you working on today?' (matches). No 'connected to kiro-cli — ...' line; no seeded sprint prompt."
   :target ("Add system/connection line on WS connect: 'connected to kiro-cli — ask me to log today, start a sprint, or draft a post' (append in onopen as a system message)."
            "On 'create a sprint' intent, kiro-cli responds with: 'Tell me more about the sprint you want to create — what skill do you want to learn this month, and what will you build to prove it?'")
   :files ("internal/web/templates/layout.html (chat markup + onopen system line)"
           "internal/bridge/bridge.go (sprint intent prompt)"
           "internal/service/intents.go")
   :acceptance ("Opening chat shows the connection line; asking to create a sprint yields the specified prompt."))

  (task :id "O2" :status todo :title "Chat quick-actions reflect KYC blocks"
   :why "'depending on the block I chose during KYC ... show ... Create new Sprint / Create new Post / Log today's work / Create new ADR.'"
   :current "/new + chat advertise a fixed action set, not filtered by enabled blocks."
   :target ("Filter actions to ENABLED blocks: Sprint->sprint, Post->posts, Log->logs, ADR->adr. Disabled blocks' actions do not appear."
            "Source the enabled set from server (template) or the localStorage blob (A2).")
   :files ("internal/web/templates/layout.html (chat actions)"
           "internal/web/templates/pages/new.html"
           "internal/web/handlers.go (pass enabled set)"
           "internal/service/intents.go")
   :acceptance ("With posts+adr disabled, chat/new only offer Sprint + Log actions.")))

 ;; ==================================================================
 (epic :id "P" :title "New-entry hub" :tags (new)
  (task :id "P1" :status todo :title "Sprint action at the top of /new"
   :why "'the new entry page, have the sprint at the top.'"
   :current "/new lists action cards (sprint/post/log/adr) + recent sprints/adrs; ordering not guaranteed sprint-first."
   :target ("Reorder so 'Create new Sprint' is the first/primary action; keep 'What do you want to do today? Pick an action...' framing; actions gated by enabled blocks (shares O2).")
   :files ("internal/web/templates/pages/new.html"
           "internal/web/handlers.go (handleNew)")
   :acceptance ("/new shows Sprint as the top action.")))

 ;; ==================================================================
 (epic :id "Q" :title "CRUD completeness for every block" :tags (crud)
  (task :id "Q1" :status todo :title "Verify/complete CRUD for all blocks"
   :why "'remember to set up CRUD for all the blocks that we have.'"
   :current "Routes: sprints(create/phase/status/retro/delete), checklist toggle, logs(create/delete), posts(create/tier/delete), adrs(create/update/delete), todos(create/status/delete), habits(create/toggle), review(save). MISSING: edit for logs+todos, habit delete/archive, post-level edit, sprint deliverables CRUD."
   :target ("Ensure full Create/Read/Update/Delete per enabled block:"
            "  sprint: + deliverables add/toggle/delete (phase2/04 AddDeliverable/Toggle); edit core fields."
            "  logs:   add update/edit (currently create+delete)."
            "  todos:  add edit (text/priority/due) beyond status/delete."
            "  habits: add delete/archive (Archived field exists)."
            "  posts:  add post-level edit (title/type) + per-tier (exists)."
            "  adr:    full edit exists; add detail page (G2)."
            "  review: edit/save exists.")
   :files ("internal/web/web.go (new routes)"
           "internal/web/handlers.go + internal/web/forms.go"
           "internal/store/*"
           "internal/service/*")
   :acceptance ("Each enabled block supports the full CRUD lifecycle from its page.")))

 ;; ==================================================================
 (suggested-order
  (step 1 "C (gating) + B (onboarding) — establish the selected-only invariant")
  (step 2 "A (auth/localStorage) — provides enabledBlocks to the client for O/P")
  (step 3 "E (sprint rules/health/mother) — core data + nesting")
  (step 4 "F (daily entries + generate post) — builds on E + bridge")
  (step 5 "G/H/I/J/K (per-page fixes) — independent, parallelizable")
  (step 6 "L (todos+checklist sync) + M (habits) + N (settings) + O/P (chat/new) + Q (CRUD)"))

 (open-questions
  (q "E1" "Keep GLOBAL single-active sprint or relax to per-ISO-week active? Default: one running at a time, reject 2nd active in same week.")
  (q "F1" "Model 'daily retro' as a new dated table or reuse DailyLog fields?")
  (q "F3" "generate-post sync (request/response) vs streamed over the existing WS?")
  (q "A2" "localStorage written via injected script on redirect, or via GET /api/me?")
  (q "C2" "disabled-route response = 403 or 404? Default: 404 with a friendly page.")))
