# Benchmark summary

Generated 2026-10-19 23:29 from 54 runs in `results/runs.csv`.

Left out: 6 run(s) the API refused or cut off (a usage limit or another API error): 20261019-143535-meowdoku-normal-max-r1, 20261019-151600-meowdoku-normal-r1, 20261019-151606-lazygit-conflict-markers-high-r2, 20261019-151640-lazygit-conflict-markers-light-force-r2, 20261019-151712-lazygit-conflict-markers-light-r2, 20261019-151745-lazygit-conflict-markers-normal-max-r2.

Medians unless stated. `pass` = deterministic check pass rate (errored runs count as failures), `judge` = median LLM-judge score (0-10), `cost` = Claude Code's client-side estimate (`total_cost_usd`, all models incl. subagents and advisor, excludes the judge), `advisor` = the part of that cost billed to the advisor model, `cache read` = median cache_read_input_tokens.

## Per use case

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| lazygit-conflict-markers | high | 2 | 0 | 100% | 9.0 | 1003.0 | 997.1-1008.8 | $6.645 | $3.114 | $13.289 | 61.5 | 6243477 |
| lazygit-conflict-markers | light | 2 | 0 | 100% | 9.0 | 245.2 | 230.1-260.4 | $0.593 | $0.000 | $1.187 | 25.0 | 1467207 |
| lazygit-conflict-markers | light-force | 2 | 0 | 100% | 8.5 | 304.7 | 264.7-344.7 | $0.700 | $0.000 | $1.400 | 29.5 | 1873322 |
| lazygit-conflict-markers | normal | 3 | 0 | 100% | 9.0 | 389.2 | 357.5-410.1 | $1.703 | - | $5.098 | 35.0 | 2380050 |
| lazygit-conflict-markers | normal-max | 2 | 2 | 0% | 8.0 | 1751.9 | 1694.6-1809.3 | $9.645 | - | $19.290 | 101.0 | 19434458 |
| linkbox-review | high | 3 | 0 | 100% | 9.0 | 585.9 | 498.8-617.8 | $13.900 | $8.332 | $41.402 | 9.0 | 1003626 |
| linkbox-review | light | 3 | 0 | 100% | 8.5 | 116.2 | 77.4-223.9 | $1.663 | $0.660 | $4.650 | 6.0 | 425952 |
| linkbox-review | light-force | 3 | 0 | 100% | 9.0 | 145.8 | 125.9-209.3 | $0.776 | $0.341 | $2.758 | 16.0 | 502308 |
| linkbox-review | normal | 3 | 0 | 100% | 9.0 | 142.1 | 125.6-1300.8 | $2.653 | - | $7.947 | 4.0 | 313687 |
| linkbox-review | normal-max | 3 | 0 | 100% | 8.5 | 924.2 | 879.4-1297.3 | $12.310 | - | $37.213 | 4.0 | 782812 |
| linkbox-storage-adr | high | 3 | 0 | 100% | 9.0 | 259.1 | 240.1-280.2 | $2.175 | $1.449 | $6.495 | 10.0 | 430017 |
| linkbox-storage-adr | light | 3 | 0 | 100% | 8.0 | 32.0 | 29.7-33.0 | $0.161 | $0.000 | $0.478 | 5.0 | 110426 |
| linkbox-storage-adr | light-force | 3 | 0 | 100% | 7.5 | 32.7 | 32.3-33.8 | $0.165 | $0.000 | $0.497 | 5.0 | 112669 |
| linkbox-storage-adr | normal | 3 | 0 | 100% | 9.0 | 56.1 | 45.9-59.9 | $0.370 | - | $1.086 | 5.0 | 152076 |
| linkbox-storage-adr | normal-max | 3 | 0 | 100% | 9.0 | 507.8 | 484.8-511.1 | $1.839 | - | $5.453 | 8.0 | 326029 |
| meowdoku | high | 3 | 0 | 100% | 9.0 | 2206.0 | 1989.4-2985.8 | $11.834 | $4.235 | $36.103 | 110.0 | 13233417 |
| meowdoku | light | 3 | 0 | 100% | 8.0 | 407.6 | 351.8-615.0 | $0.635 | $0.000 | $1.981 | 24.0 | 1110821 |
| meowdoku | light-force | 3 | 0 | 100% | 8.0 | 384.0 | 350.6-939.8 | $0.745 | $0.000 | $2.274 | 33.0 | 1578987 |
| meowdoku | normal | 2 | 0 | 100% | 8.8 | 749.0 | 615.7-882.2 | $2.267 | - | $4.534 | 49.5 | 2811068 |
| meowdoku | normal-max | 2 | 0 | 100% | 9.0 | 3462.2 | 3286.0-3638.3 | $15.738 | - | $31.476 | 173.0 | 31345010 |

## Per config, all use cases pooled

| use case | config | n | err | pass | judge | wall s (med) | min-max | cost (med) | advisor (med) | cost (sum) | turns | cache read |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ALL | high | 11 | 0 | 100% | 9.0 | 617.8 | 240.1-2985.8 | $10.803 | $3.864 | $97.289 | 23.0 | 1140128 |
| ALL | light | 11 | 0 | 100% | 8.0 | 223.9 | 29.7-615.0 | $0.635 | $0.000 | $8.296 | 17.0 | 455654 |
| ALL | light-force | 11 | 0 | 100% | 8.0 | 209.3 | 32.3-939.8 | $0.722 | $0.000 | $6.929 | 21.0 | 697660 |
| ALL | normal | 11 | 0 | 100% | 9.0 | 357.5 | 45.9-1300.8 | $1.721 | - | $18.665 | 5.0 | 327138 |
| ALL | normal-max | 10 | 2 | 80% | 9.0 | 1110.8 | 484.8-3638.3 | $11.017 | - | $93.432 | 22.0 | 1558065 |

## Models billed per config

- **high**: claude-fable-5-1, claude-haiku-5-5, claude-opus-5-5
- **light**: claude-haiku-5-5, claude-opus-5-5, claude-sonnet-5-5
- **light-force**: claude-haiku-5-5, claude-opus-5-5, claude-sonnet-5-5
- **normal**: claude-haiku-5-5, claude-opus-5-5
- **normal-max**: claude-haiku-5-5, claude-opus-5-5

## Runs with notes

- `20261019-120758-lazygit-conflict-markers-light-force-r1`: advisor 'opus' not in modelUsage
- `20261019-121310-lazygit-conflict-markers-light-r1`: advisor 'opus' not in modelUsage
- `20261019-133618-linkbox-storage-adr-light-force-r1`: advisor 'opus' not in modelUsage
- `20261019-133716-linkbox-storage-adr-light-r1`: advisor 'opus' not in modelUsage
- `20261019-142158-meowdoku-light-force-r1`: advisor 'opus' not in modelUsage
- `20261019-142847-meowdoku-light-r1`: advisor 'opus' not in modelUsage
- `20261019-160923-linkbox-storage-adr-light-force-r2`: advisor 'opus' not in modelUsage
- `20261019-161015-linkbox-storage-adr-light-r2`: advisor 'opus' not in modelUsage
- `20261019-165802-meowdoku-light-force-r2`: advisor 'opus' not in modelUsage
- `20261019-171422-meowdoku-light-r2`: advisor 'opus' not in modelUsage
- `20261019-185530-lazygit-conflict-markers-light-force-r3`: advisor 'opus' not in modelUsage
- `20261019-190340-lazygit-conflict-markers-light-r3`: advisor 'opus' not in modelUsage
- `20261019-204246-linkbox-storage-adr-light-force-r3`: advisor 'opus' not in modelUsage
- `20261019-204343-linkbox-storage-adr-light-r3`: advisor 'opus' not in modelUsage
- `20261019-214515-meowdoku-light-force-r3`: advisor 'opus' not in modelUsage
- `20261019-215257-meowdoku-light-r3`: advisor 'opus' not in modelUsage
