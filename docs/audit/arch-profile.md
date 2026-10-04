# Architecture Profile

Generated: 2026-10-04

## Detected Patterns

### Pipe and Filter — Medium confidence

Evidence:

- `main.handleMain` runs three stages in a fixed order: `api.Client.FetchContributions` /
  `FetchIssueComments` (fetch) → `analysis.Calculator.Calculate` (transform) →
  `output.Format*` / `GenerateHTML` (render).
- Each stage hands the next a plain value: `[]models.Event` plus calendar and totals
  into the calculator, `models.Statistics` into every formatter.
- Import direction (`go list`) follows the stage order and never reverses:
  - `internal/api` → `daterange`, `ghutil`, `models`
  - `internal/analysis` → `daterange`, `ghutil`, `models`
  - `internal/output` → `analysis`, `ghutil`, `models`
  - `internal/models` → `daterange`, `ghutil`
  - `internal/daterange` → `ghutil`
  - `internal/ghutil` → (stdlib only)
- No package under `internal/` imports `main`, and no stage imports a later stage.

No other catalogued pattern has a signal: there are no `domain/`, `ports/`,
`adapters/`, `commands/`/`queries/`, controllers, plugins or service roots.

## Detected Combination

None. A single pattern: a three-stage pipeline with shared leaf packages
(`models`, `daterange`, `ghutil`) carrying the values between stages.

## Inferred Structural Rules

- `internal/api` must not import `internal/analysis` or `internal/output`.
- `internal/analysis` must not import `internal/api` or `internal/output`.
- `internal/output` must not import `internal/api`; it renders `models.Statistics` only.
- `internal/models`, `internal/daterange` and `internal/ghutil` are leaves: they must
  not import any of the three stage packages.
- Orchestration (flag parsing, host and token resolution, stage sequencing) lives in
  `main` and nowhere else.

## Ambiguities & Contradictions

- `internal/output` imports `internal/analysis` for one thing, `CategoryLabels`. That
  is a render stage reading the transform stage's vocabulary, which is downstream to
  upstream and so consistent with the stage order. If the labels move, `models` is the
  natural home.
