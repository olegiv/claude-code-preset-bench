# Benchmark summary

Generated 2026-10-19 11:00 from 29 runs in `results/runs.csv`.

Medians unless stated. `pass` = deterministic check pass rate (errored runs count as failures), `judge` = median LLM-judge score (0-10), `cost` = Claude Code's client-side estimate (`total_cost_usd`, all models incl. subagents and advisor, excludes the judge), `advisor` = the part of that cost billed to the advisor model, `cache read` = median cache_read_input_tokens.

## Per use case

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| example-fix-tests | high | 1 | 0 | 100% | 8.0 | 24.9 | 24.9-24.9 | $0.348 | $0.000 | $0.348 | 6.0 | 184666 |
| example-fix-tests | light | 1 | 0 | 100% | 10.0 | 11.5 | 11.5-11.5 | $0.153 | $0.000 | $0.153 | 6.0 | 141203 |
| example-fix-tests | light-force | 1 | 0 | 100% | 10.0 | 12.0 | 12.0-12.0 | $0.153 | $0.000 | $0.153 | 6.0 | 140608 |
| example-fix-tests | normal | 1 | 0 | 100% | 9.0 | 17.0 | 17.0-17.0 | $0.303 | - | $0.303 | 4.0 | 136028 |
| example-fix-tests | normal-max | 1 | 0 | 100% | 8.0 | 35.0 | 35.0-35.0 | $0.392 | - | $0.392 | 9.0 | 225424 |
| lazygit-conflict-markers | high | 1 | 0 | 0% | - | 876.9 | 876.9-876.9 | $7.033 | $3.418 | $7.033 | 64.0 | 5777402 |
| lazygit-conflict-markers | light | 1 | 0 | 100% | - | 281.3 | 281.3-281.3 | $0.682 | $0.000 | $0.682 | 24.0 | 1536960 |
| lazygit-conflict-markers | light-force | 2 | 1 | 50% | - | 473.9 | 473.9-473.9 | $0.756 | $0.000 | - | 28.0 | 1957842 |
| lazygit-conflict-markers | normal | 2 | 1 | 50% | - | 327.7 | 327.7-327.7 | $1.602 | - | - | 32.0 | 2187190 |
| lazygit-conflict-markers | normal-max | 2 | 1 | 50% | - | 2640.2 | 2640.2-2640.2 | $13.723 | - | - | 2.0 | 701486 |
| linkbox-review | high | 1 | 0 | 100% | - | 1481.5 | 1481.5-1481.5 | $12.371 | $7.018 | $12.371 | 2.0 | 231621 |
| linkbox-review | light | 2 | 0 | 100% | - | 245.5 | 123.7-367.3 | $1.531 | $0.532 | $3.062 | 8.0 | 242081 |
| linkbox-review | light-force | 1 | 0 | 100% | - | 292.8 | 292.8-292.8 | $0.481 | $0.000 | $0.481 | 1.0 | 77098 |
| linkbox-review | normal | 1 | 0 | 100% | - | 1266.1 | 1266.1-1266.1 | $2.581 | - | $2.581 | 1.0 | 78864 |
| linkbox-review | normal-max | 1 | 0 | 100% | - | 1067.5 | 1067.5-1067.5 | $12.305 | - | $12.305 | 8.0 | 1179509 |
| linkbox-storage-adr | high | 1 | 0 | 100% | 9.0 | 305.5 | 305.5-305.5 | $2.266 | $1.509 | $2.266 | 11.0 | 458030 |
| linkbox-storage-adr | light | 1 | 0 | 100% | 8.5 | 36.0 | 36.0-36.0 | $0.172 | $0.000 | $0.172 | 4.0 | 112041 |
| linkbox-storage-adr | light-force | 1 | 0 | 100% | 8.5 | 31.1 | 31.1-31.1 | $0.205 | $0.000 | $0.205 | 5.0 | 134670 |
| linkbox-storage-adr | normal | 1 | 0 | 100% | 8.5 | 48.5 | 48.5-48.5 | $0.360 | - | $0.360 | 5.0 | 150713 |
| linkbox-storage-adr | normal-max | 1 | 0 | 100% | 9.0 | 590.5 | 590.5-590.5 | $2.099 | - | $2.099 | 9.0 | 403658 |
| meowdoku | high | 1 | 0 | 100% | - | 2526.9 | 2526.9-2526.9 | $13.881 | $4.410 | $13.881 | 127.0 | 20319704 |
| meowdoku | light | 1 | 0 | 100% | - | 557.1 | 557.1-557.1 | $0.871 | $0.000 | $0.871 | 32.0 | 1963785 |
| meowdoku | light-force | 1 | 0 | 100% | - | 661.3 | 661.3-661.3 | $0.822 | $0.000 | $0.822 | 27.0 | 1623792 |
| meowdoku | normal | 1 | 0 | 100% | - | 703.5 | 703.5-703.5 | $3.128 | - | $3.128 | 69.0 | 5123715 |
| meowdoku | normal-max | 1 | 1 | 0% | - | 3602.5 | 3602.5-3602.5 | - | - | - | - | - |

## Per config, all use cases pooled

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ALL | high | 5 | 0 | 80% | 8.5 | 876.9 | 24.9-2526.9 | $7.033 | $3.418 | $35.900 | 11.0 | 458030 |
| ALL | light | 6 | 0 | 100% | 9.2 | 202.5 | 11.5-557.1 | $0.776 | $0.000 | $4.939 | 10.5 | 273963 |
| ALL | light-force | 6 | 1 | 83% | 9.2 | 292.8 | 12.0-661.3 | $0.481 | $0.000 | - | 6.0 | 140608 |
| ALL | normal | 6 | 1 | 83% | 8.8 | 327.7 | 17.0-1266.1 | $1.602 | - | - | 5.0 | 150713 |
| ALL | normal-max | 6 | 2 | 67% | 8.5 | 1067.5 | 35.0-3602.5 | $7.202 | - | - | 8.5 | 552572 |

## Models billed per config

- **high**: claude-fable-5-1, claude-haiku-5-5, claude-opus-5-5
- **light**: claude-haiku-5-5, claude-opus-5-5, claude-sonnet-5-5
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
- `20261018-173108-lazygit-conflict-markers-light-r1`: advisor 'opus' not in modelUsage
- `20261018-182803-lazygit-conflict-markers-normal-r1`: harness error: setup.sh failed: Cannot connect to the Docker daemon at unix:///Users/olegiv/.docker/run/docker.sock. Is the docker daemon running?

- `20261018-182804-lazygit-conflict-markers-normal-max-r1`: harness error: setup.sh failed: Cannot connect to the Docker daemon at unix:///Users/olegiv/.docker/run/docker.sock. Is the docker daemon running?

- `20261018-182805-lazygit-conflict-markers-light-force-r1`: harness error: setup.sh failed: Cannot connect to the Docker daemon at unix:///Users/olegiv/.docker/run/docker.sock. Is the docker daemon running?

- `20261018-192231-lazygit-conflict-markers-light-force-r1`: advisor 'opus' not in modelUsage
- `20261018-222010-linkbox-review-light-force-r1`: advisor 'opus' not in modelUsage
- `20261019-104652-linkbox-storage-adr-light-force-r1`: advisor 'opus' not in modelUsage
- `20261019-104745-linkbox-storage-adr-light-r1`: advisor 'opus' not in modelUsage
