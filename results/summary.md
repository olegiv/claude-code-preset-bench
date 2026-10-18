# Benchmark summary

Generated 2026-10-18 11:30 from 10 runs in `results/runs.csv`.

Medians unless stated. `pass` = deterministic check pass rate (errored runs count as failures), `judge` = median LLM-judge score (0-10), `cost` = Claude Code's client-side estimate (`total_cost_usd`, all models incl. subagents and advisor, excludes the judge), `advisor` = the part of that cost billed to the advisor model, `cache read` = median cache_read_input_tokens.

## Per use case

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| example-fix-tests | high | 1 | 0 | 100% | 8.0 | 24.9 | 24.9-24.9 | $0.348 | $0.000 | $0.348 | 6.0 | 184666 |
| example-fix-tests | light | 1 | 0 | 100% | 10.0 | 11.5 | 11.5-11.5 | $0.153 | $0.000 | $0.153 | 6.0 | 141203 |
| example-fix-tests | light-force | 1 | 0 | 100% | 10.0 | 12.0 | 12.0-12.0 | $0.153 | $0.000 | $0.153 | 6.0 | 140608 |
| example-fix-tests | normal | 1 | 0 | 100% | 9.0 | 17.0 | 17.0-17.0 | $0.303 | - | $0.303 | 4.0 | 136028 |
| example-fix-tests | normal-max | 1 | 0 | 100% | 8.0 | 35.0 | 35.0-35.0 | $0.392 | - | $0.392 | 9.0 | 225424 |
| meowdoku | high | 1 | 0 | 100% | - | 2526.9 | 2526.9-2526.9 | $13.881 | $4.410 | $13.881 | 127.0 | 20319704 |
| meowdoku | light | 1 | 0 | 100% | - | 557.1 | 557.1-557.1 | $0.871 | $0.000 | $0.871 | 32.0 | 1963785 |
| meowdoku | light-force | 1 | 0 | 100% | - | 661.3 | 661.3-661.3 | $0.822 | $0.000 | $0.822 | 27.0 | 1623792 |
| meowdoku | normal | 1 | 0 | 100% | - | 703.5 | 703.5-703.5 | $3.128 | - | $3.128 | 69.0 | 5123715 |
| meowdoku | normal-max | 1 | 1 | 0% | - | 3602.5 | 3602.5-3602.5 | - | - | - | - | - |

## Per config, all use cases pooled

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ALL | high | 2 | 0 | 100% | 8.0 | 1275.9 | 24.9-2526.9 | $7.115 | $2.205 | $14.230 | 66.5 | 10252185 |
| ALL | light | 2 | 0 | 100% | 10.0 | 284.3 | 11.5-557.1 | $0.512 | $0.000 | $1.024 | 19.0 | 1052494 |
| ALL | light-force | 2 | 0 | 100% | 10.0 | 336.6 | 12.0-661.3 | $0.488 | $0.000 | $0.975 | 16.5 | 882200 |
| ALL | normal | 2 | 0 | 100% | 9.0 | 360.2 | 17.0-703.5 | $1.715 | - | $3.431 | 36.5 | 2629872 |
| ALL | normal-max | 2 | 1 | 50% | 8.0 | 1818.8 | 35.0-3602.5 | $0.392 | - | - | 9.0 | 225424 |

## Models billed per config

- **high**: claude-fable-5-1, claude-haiku-5-5, claude-opus-5-5
- **light**: claude-haiku-5-5, claude-sonnet-5-5
- **light-force**: claude-haiku-5-5, claude-sonnet-5-5
- **normal**: claude-haiku-5-5, claude-opus-5-5
- **normal-max**: claude-opus-5-5

## Runs with notes

- `20261015-113425-example-fix-tests-high-r1`: advisor 'fable' not in modelUsage
- `20261015-113459-example-fix-tests-light-force-r1`: advisor 'opus' not in modelUsage
- `20261015-113519-example-fix-tests-light-r1`: advisor 'opus' not in modelUsage
- `20261017-184032-meowdoku-light-r1`: advisor 'opus' not in modelUsage
- `20261018-054240-meowdoku-normal-max-r1`: timed out after 3600s; no JSON result; stderr: 
- `20261018-064304-meowdoku-light-force-r1`: advisor 'opus' not in modelUsage
