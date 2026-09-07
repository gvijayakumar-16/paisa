# Performance notes: ledger save & Assets page

Internal engineering notes from investigating slow `/api/editor/save` (>10s
on large ledgers) and slow `/api/assets/balance` right after a save. Not
end-user documentation (see `docs/` for that) — kept here so the next person
touching this code path doesn't re-discover the same dead ends.

Measured on a real ledger: 20 `include`d files, ~10K lines each, ~21,340
postings, ~197 distinct accounts/commodities.

## Fixes shipped

1. **`internal/model/posting/posting.go` `UpsertAll`** — was one `tx.Create()`
   per posting on every save. Now `tx.CreateInBatches(postings, 50)`. Batch
   size kept at 50 (not larger): Posting has 17 columns, 50×17=850 stays
   under the old SQLite 999-SQL-variable limit regardless of bundled SQLite
   version.

2. **`internal/ledger/ledger.go` `hasPeriodicTransactions()`** —
   `LedgerCLI.Parse`/`HLedgerCLI.Parse` each ran a second CLI subprocess for
   the `--budget`/`--forecast` pass unconditionally, even when the journal
   has zero periodic transaction directives (`~` lines) — confirmed via
   logging: 941ms subprocess, 0 records. This helper scans the journal
   (following `include` directives recursively, glob patterns, circular-safe)
   for `~` lines; if none found, the budget/forecast subprocess is skipped
   entirely. Covered by unit tests in `ledger_test.go`.
   Note: didn't reduce wall-clock Save time by itself here, because the
   budget pass was already running *concurrently* with the dominant
   normal-parse call (fix 5) — saves one subprocess spawn, not necessarily
   wall time, unless the budget pass isn't otherwise hidden under a longer
   concurrent call.

3. **`internal/server/assets/balance.go` `ComputeBreakdowns`** — was
   O(accounts × postings): re-filtered the *entire* posting list once per
   account group via `lo.Filter`. Rewritten to O(postings): one forward pass
   building `grouped[accountPrefix] -> []posting.Posting` for every
   account-path prefix that exists in the `accounts` set, same
   `IsSameOrParent` prefix-match semantics, same per-group order
   preservation. Pass 1 (building the `accounts` set / leaf flags) is left
   byte-identical to the original — it has an order-dependent overwrite
   quirk (`accounts[p.Account] = true` can get reset back to `false` by a
   later posting's ancestor-path loop) that must not be "fixed" as a side
   effect of a refactor. Covered by tests in `balance_test.go` (no prior
   coverage existed) — written and run against the *original* implementation
   first, then the refactor, to confirm identical behavior both times.

4. **`internal/service/market.go` `loadPriceCache`** — **the real
   Assets-page bottleneck**, found only via phase-by-phase timing (fix 3
   above was a smaller contributor). Was issuing one DB query per distinct
   non-currency commodity when rebuilding the price cache — a classic N+1;
   with ~197 distinct commodities, that's ~197 round-trips every time the
   cache rebuilds (i.e. every request after any sync, since `cache.Clear()`
   wipes it). Batched into a single `commodity_name IN (?)` query, grouped in
   Go afterward. Same resulting cache contents. Covered by tests in
   `market_test.go` (no prior coverage existed), same before/after
   methodology as fix 3.
   **Measured: Assets load went from 3529ms → 1477ms.**
   `PopulateMarketPrice` (which triggers this cache rebuild lazily) went
   from 3153ms → 1111ms.

5. **`internal/ledger/ledger.go` `LedgerCLI.Parse` / `HLedgerCLI.Parse`** —
   the normal and budget/forecast CLI passes were sequential; now run
   concurrently via goroutines + `sync.WaitGroup` (skipped entirely when fix
   2 determines there's nothing for the budget pass to produce). Safe: two
   independent subprocess calls whose results are just concatenated
   afterward, order-preserving.

## Attempted, measured, reverted — don't retry blind

**Background price-cache warm-up after sync.** Idea: after `Sync()`
finishes and `cache.Clear()` wipes the price cache, kick off
`go service.WarmPriceCache(db)` so the rebuild happens in the background
instead of lazily on whichever request hits it first (usually Assets).
Measured: Assets load went from a 1869ms baseline to **4340ms — worse**.
Why: SQLite here isn't in WAL mode (no `journal_mode`/WAL config anywhere in
the codebase) and there's no connection-pool tuning, so the background
goroutine's full-table scans contended with the very next request's own
queries instead of overlapping cleanly. Fully reverted.

**Overlapping `ValidateFile` with `Prices`+`Parse` in `SyncJournal`.** Idea:
`ValidateFile` is a safety gate, not a data dependency for `Prices`/`Parse`
(for the plain `ledger` backend, `Parse` doesn't even consume the `prices`
argument) — run it concurrently instead of before them, hiding its ~1.1s
under the longer ~3.9s `Prices`+`Parse` chain. Measured: total Save time
barely moved (5691ms vs 5643ms). Worse, `ValidateFile` and `Prices` both got
*slower* running concurrently (1.66s each) than sequentially (1.1s / 0.78s)
— real CPU contention between two heavy `ledger` subprocesses parsing the
same ~200K-line journal at the same time. Correctness was fine throughout
(invalid-journal path still correctly failed, just after wasting the
concurrent work); the performance win just never materialized. Fully
reverted.

**General lesson:** don't assume `max(a, b)` when running two CPU-heavy
`ledger`/`hledger` subprocesses concurrently against the *same* large
journal on this kind of machine — they compete for the same CPU and can end
up slower than sequential. The one concurrency win that held up (fix 5,
normal+budget parse) worked because the budget pass was cheap/fast relative
to the dominant normal-parse call, so contention during their brief overlap
was minor.

## The remaining floor: `ledger` CLI's own valuation cost

Save still has an inherent floor of several seconds (~3.1-4.4s of it) that
is **not paisa's Go code** — it's time spent inside the external `ledger`
CLI binary itself. The CSV format string paisa asks `ledger` to compute
(`internal/ledger/ledger.go`, `execLedgerCommand`'s big `--csv-format`
argument) includes `market(amount,date,'<currency>')` — asking `ledger`'s
own valuation engine to look up the market price for *every posting*, once
per invocation. That per-posting lookup runs inside `ledger`'s own code and
scales with (postings × price-history size). Not fixable by a `ledger`
version upgrade (checked: 3.4.1 is already latest stable).

The consistent fix exists in the same file: `HLedgerCLI.Parse` and
`Beancount.Parse` already compute market value themselves in Go (via
`pricesTree`/`lookupPrice`) instead of delegating to the CLI — only
`LedgerCLI.Parse` (the default, most-used backend) still asks the external
tool to do it. Porting `LedgerCLI.Parse` to the same pattern would mean
reimplementing `ledger`'s `market()`/cost-basis semantics in Go by hand
(multi-currency, lot pricing, cost-basis edge cases) — real risk of silently
producing wrong financial numbers if any edge case is subtly off.

**This rewrite has been explicitly declined** (asked twice, both times the
answer was no) given that risk, and separately: `tests/regression.test.ts`
currently fails on all 5 fixtures on the maintainer's dev machine for
unrelated pre-existing reasons (see below), so there's no reliable local
safety net to catch a subtly-wrong valuation port right now anyway. Don't
attempt this without either closing that test-environment gap first, or
getting explicit fresh sign-off given the stakes.

## `tests/regression.test.ts` baseline

This suite fails the same 5/5 fixtures on at least one dev machine even on
a clean checkout with zero code changes (verified via `git stash` + rerun).
Failure modes: `POST /api/sync` returning `success:false` for the
hledger/beancount/eur fixtures, and an `averageBuyPrice` field diff for the
plain `inr` fixture. Likely `ledger`/`hledger`/`beancount` CLI version drift
between that machine and whenever the fixtures were recorded — not a paisa
bug.

How to use it safely given that: run it before *and* after your change
(`git stash` is the easiest way to get a same-machine baseline), and only
treat *new* failures (different fixtures, or the same fixture with a
*different* diff) as a real regression.

Mechanical notes:
- Needs `./paisa` built at repo root: `go build -o paisa .`
- Run with `unset PAISA_CONFIG && TZ=UTC bun test tests/regression.test.ts`
- `recordAndVerify` rewrites the fixture JSON to disk on **every** run
  regardless of pass/fail (only `id`/`transaction_id`/`endLine`/
  `transaction_end_line` are excluded from the diff, but the file still gets
  rewritten with fresh autoincrement IDs). Always
  `git checkout -- tests/fixture/` after running to discard that noise.
