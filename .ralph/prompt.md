# Ralph Loop

@docs/prd.json @docs/requirements.md @.ralph/progress.md @CLAUDE.md

## Mission

Implement requirements from `prd.json`. Work by **feature** (grouped requirements), not individual requirements.

## Feature Selection

Pick the first item in `prd.json.features` where `passes === false`. Features are pre-ordered by dependency.

## Process

For each requirement in the feature:

1. Read `stepsToVerify` in prd.json
2. Search codebase for existing code before creating new files
3. Implement minimal code to satisfy requirement
4. Write tests covering all `stepsToVerify` — name each test `Test_[REQ-ID]_description`. Tests for all requirements in a feature go in the same `_test.go` file alongside the source.
5. Set requirement's `passes: true` in prd.json

After all requirements in the feature pass:

6. Run build, lint, and test (see CLAUDE.md). **DO NOT PROCEED** if any fail.
7. Set feature's `passes: true` in prd.json
8. Commit: `git add -A && git commit -m "feat([target-folder]): [FEATURE-ID] description"`
9. Update `progress.md`:

```markdown
## Feature: [feature-id]
- REQ-001: PASS - [brief summary]
- REQ-002: PASS - [brief summary]
Build: SUCCESS | Lint: SUCCESS | Test: [N] passed
```

## Constraints

- One feature per iteration
- Search before creating new files
- All verification must pass before commit
- Always commit after verification — never leave changes uncommitted
- If blocked, document in progress.md and continue

## Completion

When ALL features have `passes: true`:

```
<promise>COMPLETE</promise>
```
