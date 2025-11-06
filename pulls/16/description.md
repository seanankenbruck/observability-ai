GitHub Copilot Chat Assistant — summary of PR #16 (Feature/define user experience)

Overview
- Title: Feature/define user experience
- Author: seanankenbruck
- Branch: feature/define-user-experience → base: main
- Commits: 9
- Files changed: 8
- Additions: +5,668, Deletions: -130
- State: open, mergeable: true (mergeable_state: clean), rebaseable: true

What this PR does (high-level)
- Large documentation and UX-focused overhaul. The PR adds a comprehensive quickstart and a full set of user-facing docs to define how users install, run, and interact with Observability AI.
- Almost all changes are documentation content (README and multiple new docs); no significant application-code changes were included in the changed files returned.

Key file-level changes
- QUICKSTART.md — Completely rewritten/expanded into a step-by-step quick start with detailed Docker/local development instructions, verification steps, example queries, troubleshooting, and expected outputs.
- README.md — Updated messaging (value proposition, before/after examples), clarified quick-start pointers, added benefits and real-world examples; reorganized intro and overview content.
- docs/ARCHITECTURE.md — New, detailed architecture doc covering system diagram, request flow, component responsibilities, DB schema, discovery, auth, deployment, performance, and security.
- docs/CONFIGURATION.md — New full configuration reference describing every environment variable, presets for dev/prod/Kubernetes, validation checklists, and troubleshooting for config-related issues.
- docs/FAQ.md — New FAQ covering common questions, costs, Claude usage, privacy, setup, and deployment.
- docs/QUERY_EXAMPLES.md — New library of 30+ natural language → PromQL examples across resource, application, comparisons, time-based, aggregations, and troubleshooting queries.
- docs/TROUBLESHOOTING.md — New comprehensive troubleshooting guide for health checks, startup, DB/Redis/Claude/Prometheus issues, auth, performance, frontend problems, Docker, and advanced debugging.
- docs/WHY_OBSERVABILITY_AI.md — New rationale/benefit doc explaining the problem with PromQL, solution overview, cost/ROI arguments, and use cases.

Notable metadata / behavior
- Large documentation additions (several long markdown documents added).
- PR is authored by the repository owner and currently open and ready for review (no reviewers requested yet).