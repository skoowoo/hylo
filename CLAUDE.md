## Language

- Always respond in Simplified Chinese.

## Fixing Bugs

- Find the root cause before writing a fix. Ask why the bug is possible at all, not just how to make this symptom go away.
- Don't patch symptoms (measuring and compensating, forcing state back after the fact, special-casing one input). If a fix needs another fix, stop: the structure is wrong.
- Prefer the structural fix that makes the bug impossible by construction, and reuse patterns the codebase already uses for the same problem.
- Check whether the same root cause affects other places, and fix them together.
- When only a patch is feasible, say so explicitly and explain the trade-off.

## Changing UI

- Start from the whole surface, not the named point. Removing or changing one element reshapes its surroundings (hierarchy, spacing, where its information goes), so redesign the affected area in one pass instead of patching it step by step.
- Reuse shared components, classes and tokens (e.g. `.hairline`, `.avatar--*`, `--space-*`) rather than hand-rolling equivalents.
- Remove what the old design leaves behind: dead CSS, stale comments.
- Describe the resulting layout in the reply, not just the diff.

## Code Comments

Write as few comments as possible.

Comments should be:
- short (usually under one line)
- describe *why*, not *what*
- avoid explaining obvious code
- never write tutorial-style comments
- never comment every step

## Git Commits

- Use a single short one-line message.
- Never add Claude attribution (no `Co-Authored-By: Claude` or similar lines).