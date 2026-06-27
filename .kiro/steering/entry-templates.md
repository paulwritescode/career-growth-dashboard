# Entry Templates

These XML-structured templates define the canonical shape of every entry in local-scava. Whether an entry is created via the AI chat agent or manually through the web forms, it must always produce these exact fields. The AI agent should use these templates to parse user input and produce the correct SCAVA-ACTION payload.

## Sprint Template

```xml
<sprint>
  <skill_name><!-- Required. The skill being learned this month. One or two words. e.g. "C# and .NET" --></skill_name>
  <microapp_one_liner><!-- Required. One sentence describing what will be built. e.g. "A Blazor dashboard that recreates local-scava's Overview page" --></microapp_one_liner>
  <core_feature><!-- Required. The single feature that proves the skill was learned. e.g. "Render the Overview page in Blazor pulling live data from a .NET API" --></core_feature>
  <skill_rationale><!-- Why this skill matters for career growth. e.g. "Broaden beyond Go into the C#/.NET ecosystem" --></skill_rationale>
  <out_of_scope><!-- What is explicitly NOT being built. e.g. "Chat bridge, SVG charts, sprints/cadence pages" --></out_of_scope>
  <deploy_platform><!-- Where it will be deployed. One of: Vercel, Netlify, Railway, Fly.io, Render, Azure App Service, Other --></deploy_platform>
  <month_label><!-- Format: YYYY-MM. e.g. "2026-07" --></month_label>
</sprint>
```

**Rules:**
- `skill_name`, `microapp_one_liner`, and `core_feature` are required
- Sprint always starts in `active` status at Phase 1 (Scope & Declare)
- Only one active sprint at a time
- A sprint cannot be shipped without a `live_url`

## Daily Log Template

```xml
<log>
  <worked_on><!-- Required. What was the focus today. Short, concrete. e.g. "Set up Blazor project with dotnet new blazor" --></worked_on>
  <what_happened><!-- What actually happened — detours, surprises, blockers hit. e.g. "Spent 2h fighting hot-reload, turned out to be a .NET 9 breaking change" --></what_happened>
  <insight><!-- The specific fix, insight, or decision made. e.g. "Downgraded to .NET 8 — hot-reload works, ship > shiny" --></insight>
  <next_up><!-- What's planned for tomorrow. e.g. "Wire up the API endpoint for sprint data" --></next_up>
  <blocker><!-- Current blocker if any. Leave empty if none. e.g. "Can't get auth token from Azure AD" --></blocker>
  <blocker_decision><!-- How the blocker was resolved. One of: solve, workaround, cut, or empty --></blocker_decision>
</log>
```

**Rules:**
- `worked_on` is required
- One log per sprint per day (upserts if already exists)
- Defaults to the current active sprint and today's date
- `blocker_decision` is only set if there's a blocker

## Post Template

```xml
<post>
  <title><!-- A short title for the post. e.g. "Day 3: Hot-reload wars and a pragmatic downgrade" --></title>
  <post_type><!-- One of: daily, recap --></post_type>
  <is_declaration><!-- true if this is the Phase 1 declaration post, otherwise false --></is_declaration>
  <tiers>
    <blog>
      <content><!-- Long-form blog post content. Deep, teaching, showing work. 300-800 words. --></content>
      <url><!-- Published URL when live. e.g. "https://paulwrites.dev/posts/day-3-hot-reload" --></url>
    </blog>
    <linkedin>
      <content><!-- LinkedIn summary with link back to blog. Professional tone, 100-200 words. --></content>
      <url><!-- Published LinkedIn post URL --></url>
      <visual_kind><!-- One of: adr, diagram, screenshot, none --></visual_kind>
    </linkedin>
    <x>
      <content><!-- Punchy 1-4 line post for X/Twitter. Brag-worthy, shows momentum. --></content>
      <url><!-- Published X post URL --></url>
    </x>
  </tiers>
</post>
```

**Rules:**
- `post_type` defaults to `daily`; Sunday posts should be `recap`
- One post per day
- Tiers start as `none` status, move to `drafted` when content is written, then `published` when a URL is set
- Blog and LinkedIn are "credibility" tiers (count toward cadence)
- X tier is optional and does not count toward cadence
- Declaration posts link back to the sprint

## ADR Template

```xml
<adr>
  <title><!-- Required. Short title of the decision. e.g. "Use SQLite over PostgreSQL for local-first" --></title>
  <number><!-- Auto-assigned if blank. Sequential integer. --></number>
  <status><!-- One of: proposed, decided, superseded --></status>
  <decided_on><!-- Date in YYYY-MM-DD format when the decision was made --></decided_on>
  <problem><!-- What needed deciding. The context and forces at play. --></problem>
  <options><!-- Options considered and their tradeoffs. --></options>
  <decision><!-- The decision made. --></decision>
  <why><!-- Why this option won over the others. --></why>
  <consequences><!-- What follows from this decision — good and bad. --></consequences>
</adr>
```

**Rules:**
- `title` is required
- Number auto-increments if not provided
- ADRs can be linked to a sprint
- LinkedIn tier posts can attach an ADR as their visual

## AI Agent Behavior

When a user asks the AI chat to create an entry:

1. **Identify the entry type** from the user's message (sprint, log, post, or adr)
2. **Ask clarifying questions** if required fields are missing — always ensure there's a title/name
3. **Extract fields** from the user's natural language into the template structure
4. **Emit a SCAVA-ACTION** with the correct intent and fields

The agent should never guess required fields. If the user says "log today" without saying what they worked on, the agent should ask "What did you work on today?"
