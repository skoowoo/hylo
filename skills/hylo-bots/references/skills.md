# Choosing skills

Every new job, whether a new bot or a new trigger, goes through this before its prompt is written.

## Skill or prompt

- **A skill** when the job reaches outside the vault (news, Hacker News, podcasts, WeChat Reading, a website or API) or follows an established method (a writing process, a close-reading framework). The skill owns how it's done and gets updated; the trigger prompt names it and adds only the user's preferences (topics, how many, language).
- **A plain prompt** when the job works only on the vault (summarize, organize, recap notes), which the model and Hylo's tools already cover, or when no skill fits.

What you did by hand earlier in the chat (curl calls, URLs, API parameters) is not a method to copy into a prompt. Look for a skill that does the same job.

## Finding one

1. `skills_list`: a fitting skill whose `dirs` include the bot's `cwd` is already on for it; use it as is.
2. `skills_catalog` (add `category` when the job clearly fits one): a fitting skill is part of the plan, marked as already added or not. When several fit, offer them as multi-pick `choices`.
3. Nothing fits but the job reaches outside the vault: say in the plan that there's no ready skill for it, so the bot will search the web itself each run.

Don't add skills the job doesn't need. Built-in skills are on for the main bot already; any other bot starts with none.

## Adding one

`skills_add {"repo", "skill"}` with the catalog values if it isn't added yet, then `skills_enable {"name", "bot_id"}`. Do this before the trigger that uses it.
