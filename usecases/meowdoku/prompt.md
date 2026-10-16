---
# Provisional caps: calibrate with `python3 bench.py run --runs 1 -c light -u meowdoku`
max_turns: 150
max_budget_usd: 30
timeout_seconds: 3600
---
Build **Meowdoku**, a single-player logic puzzle with cats, as a Go web app in this directory.

## Rules

- The board is a 10×10 grid split into 10 coloured regions. Each region is one 4-connected group of cells.
- The player places 10 cats so that every row, every column and every region holds exactly one cat, and no two cats touch, not even diagonally.
- The player can also mark a cell with ✕ as a note that no cat goes there. Marks never affect the rules.
- The game is won when 10 cats are on the board and no rule is broken. The board starts empty, and every generated puzzle has exactly one solution.

## Stack

- Go 1.27, standard library only. Do not edit `go.mod` (module `meowdoku`).
- Server-rendered HTML with `html/template`. Templates and static files are embedded with `embed`.
- htmx 4.0.0 and Alpine.js 3.17.4 are vendored in `web/static/` as `htmx.min.js` and `alpine.min.js`. Serve them from there, do not modify or replace these two files, and load nothing from a CDN. htmx 4 is not htmx 2: check https://four.htmx.org/ (start with the migration guide) instead of relying on memory.
- htmx does all communication with the server. Alpine holds client-only UI state, such as the how-to-play panel and the win message. No game logic in JavaScript, and no hand-written script files.

## Contract

Acceptance tests you cannot see exercise exactly this contract. Everything else is your call.

### Package `meowdoku/engine`

Pure game logic: no I/O, no `net/http`, no package-level mutable state.

```go
const Size = 10

type Cell int

const (
	Empty Cell = iota
	Mark
	Cat
)

type Board [Size][Size]Cell // [row][col]

type Pos struct{ Row, Col int }

type Puzzle struct {
	Seed    int64
	Regions [Size][Size]int // [row][col] -> region id 0..Size-1
}

// Generate returns the puzzle for seed. Same seed, identical Puzzle, in any process
// (no global rand, clock or map-order dependence). Seed is set to seed, Validate passes,
// and the puzzle has exactly one solution. It terminates for every int64 seed.
func Generate(seed int64) Puzzle

// Validate returns an error unless every region id is in 0..Size-1, all Size ids are used,
// and every region is 4-connected.
func (p Puzzle) Validate() error

// CountSolutions returns min(number of solutions, limit), and 0 when limit <= 0.
func CountSolutions(p Puzzle, limit int) int

// Solve returns a solution as the column of the cat in each row, or ok == false if there is none.
func Solve(p Puzzle) (cols [Size]int, ok bool)

// Check returns every cat that shares a row, column or region with another cat, or touches
// another cat (8 neighbours), sorted by Row then Col, without duplicates. Marks are ignored.
// won is true when the board holds exactly Size cats and conflicts is empty.
func Check(p Puzzle, b Board) (conflicts []Pos, won bool)
```

`CountSolutions` and `Solve` are only called with puzzles that pass `Validate`. Each `Generate` call must finish in under 2 seconds; the tests time it on many seeds. Different seeds give different puzzles: any 30 consecutive seeds yield at least 27 distinct region layouts.

### Package `meowdoku/web`

```go
// NewHandler returns the whole app: routes, embedded static files and an in-memory
// game store. Every call returns an independent instance. Safe for concurrent use.
func NewHandler() http.Handler
```

`go run ./cmd/meowdoku` serves it on the address given by `-addr` (default `127.0.0.1:8080`).

### Routes

| Request | Success | Errors |
|---|---|---|
| `GET /` | 200 `text/html`: a start page from which a new game can be started | |
| `POST /games`, form field `seed` (decimal int64; optional, random when absent or empty) | 303 with `Location: /games/{id}`, where `id` matches `[A-Za-z0-9_-]{1,64}` | 400 for an invalid seed |
| `GET /games/{id}` | 200 `text/html`: the full page with the board | 404 for an unknown game |
| `GET /games/{id}/state` | 200 `application/json`, shape below | 404 |
| `POST /games/{id}/cells/{row}/{col}`, form field `state` = `empty`, `mark` or `cat` | sets that cell; 200 `text/html` with the board fragment (`#board`) | 404 for an unknown game; 400 for a row or col outside 0–9 or any other `state`; 409 once the game is won, with the board unchanged |
| `GET /static/htmx.min.js`, `GET /static/alpine.min.js` | 200, the vendored bytes | |

A rejected request (any 4xx response) leaves the game unchanged.

A route requested with any other method answers 405; HEAD is accepted wherever GET is. The response to the move that wins the game carries an `HX-Trigger` response header that fires a `won` event; no other response does.

State JSON:

```json
{
  "id": "k3Jx9",
  "seed": 42,
  "regions": [[0, 0, 1, ...], ...],
  "board": [["empty", "mark", "cat", ...], ...],
  "conflicts": [[3, 4], [3, 7]],
  "won": false
}
```

- `regions` and `board` are 10×10, indexed `[row][col]`.
- `regions` equals `engine.Generate(seed).Regions`.
- `conflicts` are `[row, col]` pairs from `engine.Check`, and `[]` when there are none.

### Markup the tests read

- The board element has `id="board"` and `data-won="true"` or `data-won="false"`.
- Inside it are exactly 100 cell elements. Each has:
  - `data-row` and `data-col` (0–9)
  - `data-region` (0–9)
  - `data-state` (`empty`, `mark` or `cat`)
  - `data-conflict="true"` when the cell is in `conflicts` (otherwise absent or `"false"`)

  Use double quotes for these attributes.
- Every full page loads `/static/htmx.min.js` and `/static/alpine.min.js`.

## Experience

- Clicking or tapping a cell cycles empty → mark → cat → empty. Cells are buttons with accessible labels, so the game also works by keyboard.
- Regions can be told apart without relying on colour alone, for example by thick borders between them.
- Conflicting cats are highlighted after every move. A win shows a clear message, locks the board and offers a new game.
- An error response from the server never replaces the board in the browser.
- It works at phone width. Cats and marks are emoji, your own SVG or CSS; no third-party art.

## Done means

- `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass.
- Your own tests cover the rules, the generator's uniqueness and determinism, and the routes.
- If you start the server to try it, run it in the background and stop it before you finish.
- Finish with a short summary: how the generator guarantees a single solution, and anything you left out.
