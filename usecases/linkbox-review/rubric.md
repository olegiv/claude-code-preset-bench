Score the agent's review of the linkbox pull request from 0 to 10.

The agent was asked to review an uncommitted pull request ("Add link previews, pagination and CSV export", about 550 lines), not to fix it, and to write REVIEW.md with one finding per line: `- [severity] path:line — title: explanation`. The diff you see contains the pull request and REVIEW.md. The deterministic check matched REVIEW.md's findings against ten problems planted in the pull request and five decoys (code that looks wrong but is correct): its score is (problems found − decoys flagged) / 10, and PASS needs at least 0.60 with no failed gate. A failed gate means REVIEW.md is missing, the code was changed or the run committed (score 0.00), or extra files, a broken format, more than 25 findings, or no completed pr-review-toolkit agent (score unchanged). The check measures how many problems were found; you judge everything else. The diff may be cut off at 120 000 characters: do not penalise what you cannot see.

The planted problems (line numbers are in the changed files):

1. high, security: `internal/preview/preview.go` fetches any URL a user saved, loopback and internal addresses included (SSRF); `previewLink` also returns the fetch error text.
2. medium, resources: `preview.go` `fetch` returns on a non-200 status without closing the response body.
3. high, concurrency: `Fetcher.cache` is read and written without a lock while `TitleAll` and concurrent requests call `Title`: a data race.
4. medium, tests: `TestTitleTimeout` never exercises the timeout (the server answers in 50 ms, the client waits 5 s).
5. high, security: `export.WriteCSV` joins the client's `?name=` into the path, checking only `.csv`, so `/../../x.csv` escapes the export directory.
6. high, authorization: `exportLinks` honours `?owner=` for any user, so anyone can export anyone's links.
7. medium, logic: `listLinks` documents pages from 1 but treats `page` as 0-based, so `?page=1` skips the first page.
8. medium, error handling: `previewLink` discards `UpdatePreview`'s error and answers 200.
9. low, comments: `NewFetcher` says titles are kept for a day; the TTL is 24 minutes.
10. medium, type design: `Store.ListByOwner` returns pointers into the store, breaking its documented promise of copies.

The decoys, where a finding is wrong: the export-cleanup goroutine leaking (it stops on shutdown); `UpdatePreview` splitting multi-byte characters (it cuts runes); `export.Cleanup` deleting arbitrary files (it removes only prefixed regular files it listed); a double `Close` in `WriteCSV` (the deferred one is a harmless fallback); ignored `csv.Writer.Write` errors (the writer's sticky error is checked after `Flush`).

- 9–10: At least eight planted problems, each with the right location, a correct explanation of what breaks and a sensible severity (the security and race problems high); no decoy flagged; few or no findings outside the list, and those correct. REVIEW.md is concise and follows the format; the verdict is clear.
- 7–8: Six or seven planted problems explained correctly, at most one decoy or a few weak extra findings, or minor severity misjudgements.
- 4–6: Three to five planted problems, or more but with wrong or vague explanations, inflated severities, several decoys or noisy extra findings.
- 1–3: Fewer than three planted problems, a review mostly made of noise, or code changed instead of reviewed.
- 0: No review.

Findings outside the list may be real: judge them on their merits, and count only wrong or trivial ones as noise. Do not reward length or a review that pads itself with style remarks.
