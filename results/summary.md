# Benchmark summary

Generated 2026-10-15 11:36 from 5 runs in `results/runs.csv`.

Medians unless stated. `pass` = deterministic check pass rate (errored runs count as failures), `judge` = median LLM-judge score (0-10), `cost` = Claude Code's client-side estimate (`total_cost_usd`, all models incl. subagents and advisor, excludes the judge), `advisor` = the part of that cost billed to the advisor model, `cache read` = median cache_read_input_tokens.

## Per use case

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| example-fix-tests | high | 1 | 0 | 100% | 8.0 | 24.9 | 24.9-24.9 | $0.348 | $0.000 | $0.348 | 6.0 | 184666 |
| example-fix-tests | light | 1 | 0 | 100% | 10.0 | 11.5 | 11.5-11.5 | $0.153 | $0.000 | $0.153 | 6.0 | 141203 |
| example-fix-tests | light-force | 1 | 0 | 100% | 10.0 | 12.0 | 12.0-12.0 | $0.153 | $0.000 | $0.153 | 6.0 | 140608 |
| example-fix-tests | normal | 1 | 0 | 100% | 9.0 | 17.0 | 17.0-17.0 | $0.303 | - | $0.303 | 4.0 | 136028 |
| example-fix-tests | normal-max | 1 | 0 | 100% | 8.0 | 35.0 | 35.0-35.0 | $0.392 | - | $0.392 | 9.0 | 225424 |

## Per config, all use cases pooled

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ALL | high | 1 | 0 | 100% | 8.0 | 24.9 | 24.9-24.9 | $0.348 | $0.000 | $0.348 | 6.0 | 184666 |
| ALL | light | 1 | 0 | 100% | 10.0 | 11.5 | 11.5-11.5 | $0.153 | $0.000 | $0.153 | 6.0 | 141203 |
| ALL | light-force | 1 | 0 | 100% | 10.0 | 12.0 | 12.0-12.0 | $0.153 | $0.000 | $0.153 | 6.0 | 140608 |
| ALL | normal | 1 | 0 | 100% | 9.0 | 17.0 | 17.0-17.0 | $0.303 | - | $0.303 | 4.0 | 136028 |
| ALL | normal-max | 1 | 0 | 100% | 8.0 | 35.0 | 35.0-35.0 | $0.392 | - | $0.392 | 9.0 | 225424 |

## Models billed per config

- **high**: claude-opus-5-5
- **light**: claude-sonnet-5-5
- **light-force**: claude-sonnet-5-5
- **normal**: claude-opus-5-5
- **normal-max**: claude-opus-5-5

## Runs with notes

- `20261015-113425-example-fix-tests-high-r1`: advisor 'fable' not in modelUsage
- `20261015-113459-example-fix-tests-light-force-r1`: advisor 'opus' not in modelUsage
- `20261015-113519-example-fix-tests-light-r1`: advisor 'opus' not in modelUsage
