---
name: hylo-recap
description: "Reviews content against the Hylo knowledge base (_knowledge/) and surfaces what's related and what's in tension. Covers two scenarios: (1) associative recap — cross-check a single short note, daily note, draft, or pasted text against the knowledge base right now, while the thought is fresh; (2) periodic review — recap everything created or updated over a recent window (e.g. the past week). Trigger on phrases like 'recap this', 'what do I already know that relates to this', 'does this conflict with anything I've written', 'weekly review', 'review this week's notes', '联想', '回顾一下', '和我的知识库对照一下', '有没有矛盾的观点', '实时联想', '本周回顾', '周回顾', or any request to recall related or conflicting knowledge, either for a piece of writing or for a recent time window."
---

# Hylo Recap — Router

## Tools

The tools named here are Hylo's MCP tools (server `hylo`); your client may show them prefixed, e.g. `mcp__hylo__read`. If they are not in your tool list, search your deferred tools for "hylo". If they are still missing, tell the user Hylo's tools are unavailable and stop.

---

## Identify intent → load sub-skill

| Scenario              | Trigger signals                                                                                                                       | Sub-skill           |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| **Associative recap** | user shares/references a single note, short note, draft, or pasted text and wants related or conflicting knowledge surfaced right now | @associative.md     |
| **Periodic review**   | "weekly review", "回顾一下这周/这个月", review of everything created/updated over a past window rather than one piece of content      | @periodic-review.md |

Load the matching sub-skill. If the request is ambiguous (e.g. just "recap" with no note and no time window named), ask the user which they mean before proceeding.

Both sub-skills share `references/extract.md` for the partial-read tools, and both produce a conversational recap only — never a saved file.
