# GitHub evidence for PR-review reactivation

Research for [Establish GitHub evidence for reactivating a reviewed pull request](https://github.com/YouriLieverdink/fylla/issues/111), 2026-07-29.

## Result

Fylla cannot establish this state using its current GitHub **Search API-only** access. `GET /search/issues` returns a PR search item with generic issue timestamps and a `pull_request` link object, but neither the current head SHA nor review records. Its `updated_at` is not a safe signal: it may change for review activity, comments, labels, and other PR updates as well as a push. It would create false actionable work items.

The current client confirms this limitation: `app/GitHub/Client.php` consumes only `/search/issues` and persists `id`, number, repository, title/body, URL, state, and `created_at` ([ADR-0009](../adr/0009-github-prs-as-timeable-tasks.md)). Its classic PAT is known to receive 403 from repo-scoped endpoints, so the richer REST evidence below is not available to the application today.

A local `gh api` probe did show that the developer's CLI credential can call `GET /repos/Back-to-code/f2f-app/pulls/279` and receive `head.sha`. This is evidence about that CLI credential only; it must not be treated as evidence that Fylla's configured PAT can do the same.

## Viable evidence if application access changes

For each candidate open PR, use these repository-scoped REST calls:

1. [`GET /repos/{owner}/{repo}/pulls/{pull_number}`](https://docs.github.com/en/rest/pulls/pulls#get-a-pull-request) for `head.sha`.
2. [`GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews`](https://docs.github.com/en/rest/pulls/reviews#list-reviews-for-a-pull-request) for reviews by the authenticated user, including each review's `commit_id`, `state`, and `submitted_at`.

Compare the current `head.sha` with the user's most recent submitted review's `commit_id`. A difference means the user's latest review did not cover the current head and is a practical reactivation signal. Persist the observed head SHA and/or latest qualifying review metadata at sync time so the worklist can identify a transition and avoid continuously reintroducing an unchanged PR.

This comparison does **not** prove the exact chronological wording “a commit landed after the review” in every race/force-push case: it proves that the latest review is for a different commit. If strict push chronology is required, additionally inspect repository-visible timeline/push evidence and define force-push handling; that requires the same unavailable repo access.

## Access, pagination, and limits

Both richer REST endpoints require access to the repository. GitHub documents fine-grained PAT permissions for pull-request endpoints; the actual organization policy and token type must be validated with the configured Fylla credential before design depends on them. A GitHub App installation token or an organization-approved fine-grained PAT is the likely route; this research does not select one.

The current search feed makes one `per_page=100` request per configured query and marks any result beyond that page truncated. The PR endpoint adds one request per candidate PR; reviews are paginated and must be followed until the user's newest relevant review is known. Cache/persist the result and limit richer calls to candidate PRs rather than scanning all historic reviewed PRs. GitHub's REST API uses rate-limit headers, so sync should respect them and retain the prior local state on incomplete data.

## Safe fallback

Until the application credential succeeds against both repository-scoped endpoints for representative private organization PRs, do **not** add reviewer-reactivated PRs to the worklist. Retain the existing `review-requested:@me` feed; it is an explicit request and does not manufacture a false action from `updated_at`. A surfaced PR may still follow the existing manual PR-to-Kendo resolution before timing.

## Primary sources

- [GitHub REST: Search issues and pull requests](https://docs.github.com/en/rest/search/search?apiVersion=2022-11-28#search-issues-and-pull-requests)
- [GitHub: Searching issues and pull requests (qualifiers, including `reviewed-by`)](https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests)
- [GitHub REST: Get a pull request](https://docs.github.com/en/rest/pulls/pulls#get-a-pull-request)
- [GitHub REST: List reviews for a pull request](https://docs.github.com/en/rest/pulls/reviews#list-reviews-for-a-pull-request)
- [GitHub REST pagination](https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api)
- [GitHub REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
