package engine_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"meowdoku/engine"
)

const (
	// hiddenGenerateLimit is the contract's limit for one Generate call, asserted only by
	// TestHiddenGenerateSpeed.
	hiddenGenerateLimit = 2 * time.Second
	// hiddenWatchdog only stops a hung call from stalling the package; it is no performance
	// requirement.
	hiddenWatchdog = 20 * time.Second
)

// hiddenTry runs f in its own goroutine, so a panic or a hang in the code under test fails the
// calling test instead of crashing or stalling the whole package.
func hiddenTry[T any](limit time.Duration, f func() T) (v T, took time.Duration, problem string, timedOut bool) {
	type result struct {
		v        T
		took     time.Duration
		returned bool
		panicked string
	}
	ch := make(chan result, 1)
	go func() {
		var res result
		defer func() {
			if !res.returned {
				res.panicked = fmt.Sprint(recover())
			}
			ch <- res
		}()
		start := time.Now()
		res.v = f()
		res.took = time.Since(start)
		res.returned = true
	}()
	select {
	case res := <-ch:
		if !res.returned {
			return v, 0, "panicked: " + res.panicked, false
		}
		return res.v, res.took, "", false
	case <-time.After(limit):
		return v, limit, fmt.Sprintf("did not return within %v", limit), true
	}
}

func hiddenCall[T any](t *testing.T, what string, f func() T) T {
	t.Helper()
	v, _, problem, _ := hiddenTry(hiddenWatchdog, f)
	if problem != "" {
		t.Fatalf("%s %s", what, problem)
	}
	return v
}

// Generate results are memoized per seed, so the whole package makes about 50 Generate calls
// even when each one takes close to the 2 s limit. Only TestHiddenGenerateDeterministic calls it
// fresh on purpose. A test stops at the first call that times out, so a hung generator costs
// one watchdog per test, not one per seed.
type hiddenGen struct {
	p        engine.Puzzle
	took     time.Duration
	problem  string
	timedOut bool
}

var (
	hiddenGenMu   sync.Mutex
	hiddenGenMemo = map[int64]hiddenGen{}
)

func hiddenGenerateFresh(seed int64) hiddenGen {
	p, took, problem, timedOut := hiddenTry(hiddenWatchdog, func() engine.Puzzle { return engine.Generate(seed) })
	return hiddenGen{p, took, problem, timedOut}
}

func hiddenGenerate(seed int64) hiddenGen {
	hiddenGenMu.Lock()
	g, ok := hiddenGenMemo[seed]
	hiddenGenMu.Unlock()
	if ok {
		return g
	}
	g = hiddenGenerateFresh(seed)
	hiddenRemember(seed, g)
	return g
}

func hiddenRemember(seed int64, g hiddenGen) {
	hiddenGenMu.Lock()
	defer hiddenGenMu.Unlock()
	if _, ok := hiddenGenMemo[seed]; !ok {
		hiddenGenMemo[seed] = g
	}
}

// hiddenGenerated returns the memoized Generate(seed), reporting a problem; a timeout ends the
// test.
func hiddenGenerated(t *testing.T, seed int64) (hiddenGen, bool) {
	t.Helper()
	g := hiddenGenerate(seed)
	if g.timedOut {
		t.Fatalf("Generate(%d) %s", seed, g.problem)
	}
	if g.problem != "" {
		t.Errorf("Generate(%d) %s", seed, g.problem)
		return g, false
	}
	return g, true
}

var hiddenSpecialSeeds = []int64{0, 1, 2, 42, 2026, -1, -7, math.MinInt64, math.MaxInt64}

func hiddenSeedRange(from, to int64) []int64 {
	var s []int64
	for i := from; i <= to; i++ {
		s = append(s, i)
	}
	return s
}

// hiddenCheckedSeeds: seeds 1-30 plus the special seeds, without repeats.
func hiddenCheckedSeeds() []int64 {
	s := hiddenSeedRange(1, 30)
	for _, x := range hiddenSpecialSeeds {
		if x < 1 || x > 30 {
			s = append(s, x)
		}
	}
	return s
}

func hiddenPuzzle(reg hiddenLayout) engine.Puzzle {
	return engine.Puzzle{Seed: 0, Regions: reg}
}

const (
	hiddenChildEnv    = "MEOWDOKU_HIDDEN_CHILD"
	hiddenChildPrefix = "MEOWDOKU_HIDDEN_CHILD_PUZZLES "
)

type hiddenChildPuzzle struct {
	Seed    int64
	Regions hiddenLayout
}

// TestHiddenGenerateDeterministic also runs itself in a second process, which prints the
// special seeds' puzzles, so a generator that depends on the process (a start-time salt, map
// order) fails too.
func TestHiddenGenerateDeterministic(t *testing.T) {
	if os.Getenv(hiddenChildEnv) == "1" {
		var out []hiddenChildPuzzle
		for _, seed := range hiddenSpecialSeeds {
			out = append(out, hiddenChildPuzzle{seed, engine.Generate(seed).Regions})
		}
		b, _ := json.Marshal(out)
		fmt.Println(hiddenChildPrefix + string(b))
		return
	}
	for _, seed := range hiddenSpecialSeeds {
		a := hiddenGenerateFresh(seed)
		if a.timedOut {
			t.Fatalf("Generate(%d) %s", seed, a.problem)
		}
		b := hiddenGenerateFresh(seed)
		if b.timedOut {
			t.Fatalf("Generate(%d) %s", seed, b.problem)
		}
		if a.problem != "" || b.problem != "" {
			t.Errorf("Generate(%d): %s%s", seed, a.problem, b.problem)
			continue
		}
		hiddenRemember(seed, a)
		if a.p != b.p {
			t.Errorf("Generate(%d) returned different puzzles on two calls", seed)
		}
		if a.p.Seed != seed {
			t.Errorf("Generate(%d).Seed = %d", seed, a.p.Seed)
		}
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("cannot find the test binary: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestHiddenGenerateDeterministic$", "-test.count=1")
	cmd.Env = append(os.Environ(), hiddenChildEnv+"=1")
	output, err := cmd.Output()
	var child []hiddenChildPuzzle
	sc := bufio.NewScanner(bytes.NewReader(output))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if line, ok := strings.CutPrefix(sc.Text(), hiddenChildPrefix); ok {
			if jerr := json.Unmarshal([]byte(line), &child); jerr != nil {
				t.Fatalf("second process: unreadable output: %v", jerr)
			}
		}
	}
	if len(child) != len(hiddenSpecialSeeds) {
		t.Fatalf("second process gave no puzzles (err %v); output: %.300s", err, output)
	}
	for _, c := range child {
		if g := hiddenGenerate(c.Seed); g.problem == "" && g.p.Regions != c.Regions {
			t.Errorf("Generate(%d) returned a different puzzle in another process", c.Seed)
		}
	}
}

func TestHiddenGenerateValid(t *testing.T) {
	for _, seed := range hiddenCheckedSeeds() {
		g, ok := hiddenGenerated(t, seed)
		if !ok {
			continue
		}
		if g.p.Seed != seed {
			t.Errorf("Generate(%d).Seed = %d", seed, g.p.Seed)
		}
		if why := hiddenLayoutProblem(g.p.Regions); why != "" {
			t.Errorf("Generate(%d): invalid regions: %s", seed, why)
		}
		err, _, problem, timedOut := hiddenTry(hiddenWatchdog, g.p.Validate)
		switch {
		case timedOut:
			t.Fatalf("Generate(%d).Validate() %s", seed, problem)
		case problem != "":
			t.Errorf("Generate(%d).Validate() %s", seed, problem)
		case err != nil:
			t.Errorf("Generate(%d).Validate() = %v, want nil", seed, err)
		}
	}
}

func TestHiddenGenerateUnique(t *testing.T) {
	for _, seed := range hiddenCheckedSeeds() {
		g, ok := hiddenGenerated(t, seed)
		if !ok {
			continue
		}
		if why := hiddenLayoutProblem(g.p.Regions); why != "" {
			t.Errorf("Generate(%d): invalid regions, cannot count solutions: %s", seed, why)
			continue
		}
		if n, _ := hiddenSolutions(g.p.Regions, 2); n != 1 {
			t.Errorf("Generate(%d) has %d solutions (counted up to 2), want exactly 1", seed, n)
		}
	}
}

func TestHiddenGenerateVaried(t *testing.T) {
	for _, from := range []int64{1, -30} { // two sampled windows of 30 consecutive seeds
		distinct := map[hiddenLayout]bool{}
		for _, seed := range hiddenSeedRange(from, from+29) {
			if g, ok := hiddenGenerated(t, seed); ok {
				distinct[hiddenCanonical(g.p.Regions)] = true
			}
		}
		if len(distinct) < 27 {
			t.Errorf("seeds %d to %d gave %d distinct region layouts, want at least 27", from, from+29, len(distinct))
		}
	}
}

func TestHiddenGenerateSpeed(t *testing.T) {
	for _, seed := range hiddenSeedRange(1, 30) {
		if g, ok := hiddenGenerated(t, seed); ok && g.took >= hiddenGenerateLimit {
			t.Errorf("Generate(%d) took %v, want under %v", seed, g.took, hiddenGenerateLimit)
		}
	}
}

func TestHiddenValidate(t *testing.T) {
	type tc struct {
		name   string
		reg    hiddenLayout
		wantOK bool
	}
	var cases []tc

	reg := hiddenRowsLayout()
	reg[3][4] = 10
	cases = append(cases, tc{"region id 10", reg, false})

	reg = hiddenRowsLayout()
	reg[0][0] = -1
	cases = append(cases, tc{"region id -1", reg, false})

	reg = hiddenRowsLayout()
	for c := range hiddenN {
		reg[9][c] = 8
	}
	cases = append(cases, tc{"only 9 ids used", reg, false})

	reg = hiddenRowsLayout()
	reg[5][5] = 0
	cases = append(cases, tc{"region in two parts", reg, false})

	reg = hiddenRowsLayout()
	reg[0][9], reg[1][9] = 1, 0
	cases = append(cases, tc{"parts touching only diagonally", reg, false})

	cases = append(cases,
		tc{"rows as regions", hiddenRowsLayout(), true},
		tc{"columns as regions", hiddenColsLayout(), true},
		tc{"hand-made 2x5 blocks", hiddenBlocksLayout(), true},
		tc{"hand-made forced layout", hiddenForcedLayout(), true},
		tc{"valid but unsolvable layout", hiddenNoSolutionLayout(), true},
	)

	for _, c := range cases {
		if (hiddenLayoutProblem(c.reg) == "") != c.wantOK {
			t.Fatalf("bad fixture %q: oracle says %q", c.name, hiddenLayoutProblem(c.reg))
		}
		p := hiddenPuzzle(c.reg)
		err := hiddenCall(t, "Validate on "+c.name, p.Validate)
		if c.wantOK && err != nil {
			t.Errorf("%s: Validate() = %v, want nil", c.name, err)
		}
		if !c.wantOK && err == nil {
			t.Errorf("%s: Validate() = nil, want an error", c.name)
		}
	}
}

func TestHiddenCountSolutions(t *testing.T) {
	fixtures := []struct {
		name  string
		reg   hiddenLayout
		count int
	}{
		{"no solution", hiddenNoSolutionLayout(), 0},
		{"exactly one solution", hiddenForcedLayout(), 1},
		{"rows as regions", hiddenRowsLayout(), 479306},
	}
	for _, f := range fixtures {
		if why := hiddenLayoutProblem(f.reg); why != "" {
			t.Fatalf("bad fixture %q: %s", f.name, why)
		}
		if n, _ := hiddenSolutions(f.reg, 0); n != f.count {
			t.Fatalf("bad fixture %q: oracle counts %d solutions, want %d", f.name, n, f.count)
		}
	}
	for _, f := range fixtures {
		for _, limit := range []int{-1, 0, 1, 2, 5, 1000, 1 << 30} {
			want := max(0, min(f.count, limit))
			p := hiddenPuzzle(f.reg)
			got := hiddenCall(t, fmt.Sprintf("CountSolutions(%s, %d)", f.name, limit),
				func() int { return engine.CountSolutions(p, limit) })
			if got != want {
				t.Errorf("CountSolutions(%s, %d) = %d, want %d", f.name, limit, got, want)
			}
		}
	}
}

type hiddenSolveResult struct {
	cols [engine.Size]int
	ok   bool
}

func hiddenSolve(t *testing.T, what string, reg hiddenLayout) hiddenSolveResult {
	t.Helper()
	p := hiddenPuzzle(reg)
	return hiddenCall(t, "Solve("+what+")", func() hiddenSolveResult {
		cols, ok := engine.Solve(p)
		return hiddenSolveResult{cols, ok}
	})
}

// "Solve returns a solution": the result must satisfy the rules; when the oracle counts exactly
// one solution, that also makes it equal to the oracle's.
func TestHiddenSolve(t *testing.T) {
	for _, seed := range hiddenSeedRange(1, 10) {
		g, ok := hiddenGenerated(t, seed)
		if !ok {
			continue
		}
		reg := g.p.Regions
		if why := hiddenLayoutProblem(reg); why != "" {
			t.Errorf("Generate(%d): invalid regions, cannot solve: %s", seed, why)
			continue
		}
		n, want := hiddenSolutions(reg, 2)
		got := hiddenSolve(t, fmt.Sprintf("Generate(%d)", seed), reg)
		switch {
		case n == 0 && got.ok:
			t.Errorf("Solve(Generate(%d)) = %v, true; the puzzle has no solution", seed, got.cols)
		case n > 0 && !got.ok:
			t.Errorf("Solve(Generate(%d)) returned ok == false; the oracle found %v", seed, want)
		case got.ok && !hiddenIsSolution(reg, got.cols):
			t.Errorf("Solve(Generate(%d)) = %v, which breaks the rules", seed, got.cols)
		case got.ok && n == 1 && got.cols != want:
			t.Errorf("Solve(Generate(%d)) = %v, want %v", seed, got.cols, want)
		}
	}

	forced := hiddenForcedLayout()
	if n, sol := hiddenSolutions(forced, 0); n != 1 || sol != hiddenForcedSolution {
		t.Fatalf("bad fixture: forced layout has %d solutions, first %v", n, sol)
	}
	if got := hiddenSolve(t, "forced layout", forced); !got.ok || got.cols != hiddenForcedSolution {
		t.Errorf("Solve(forced layout) = %v, %v; want %v, true", got.cols, got.ok, hiddenForcedSolution)
	}

	none := hiddenNoSolutionLayout()
	if n, _ := hiddenSolutions(none, 0); n != 0 {
		t.Fatalf("bad fixture: no-solution layout has %d solutions", n)
	}
	if got := hiddenSolve(t, "no-solution layout", none); got.ok {
		t.Errorf("Solve(no-solution layout) = %v, true; want ok == false", got.cols)
	}
}

func hiddenBoard(cats, marks []hiddenCell) engine.Board {
	var b engine.Board
	for _, m := range marks {
		b[m.R][m.C] = engine.Mark
	}
	for _, c := range cats {
		b[c.R][c.C] = engine.Cat
	}
	return b
}

func TestHiddenCheck(t *testing.T) {
	reg := hiddenBlocksLayout()
	if why := hiddenLayoutProblem(reg); why != "" {
		t.Fatalf("bad fixture: %s", why)
	}
	n, sol := hiddenSolutions(reg, 1)
	if n != 1 {
		t.Fatalf("bad fixture: the blocks layout has no solution")
	}
	var solution, others []hiddenCell
	for r := range hiddenN {
		for c := range hiddenN {
			if sol[r] == c {
				solution = append(solution, hiddenCell{r, c})
			} else {
				others = append(others, hiddenCell{r, c})
			}
		}
	}
	// Ten cats, one of them moved within its row so it breaks a rule.
	var broken []hiddenCell
	for c := 0; c < hiddenN && broken == nil; c++ {
		moved := append(append([]hiddenCell{}, solution[:9]...), hiddenCell{9, c})
		if c != sol[9] && len(hiddenRuleConflicts(reg, moved)) > 0 {
			broken = moved
		}
	}
	if broken == nil {
		t.Fatalf("bad fixture: no broken board found")
	}

	type cell = hiddenCell
	cases := []struct {
		name  string
		cats  []cell
		marks []cell
		want  []cell // nil: compute with the oracle
		won   bool
	}{
		{"empty board", nil, nil, []cell{}, false},
		{"marks only", nil, []cell{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {5, 5}, {9, 9}}, []cell{}, false},
		{"one cat", []cell{{4, 4}}, nil, []cell{}, false},
		{"same row", []cell{{0, 7}, {0, 0}}, nil, []cell{{0, 0}, {0, 7}}, false},
		{"same column", []cell{{6, 3}, {0, 3}}, nil, []cell{{0, 3}, {6, 3}}, false},
		{"same region", []cell{{3, 4}, {2, 0}}, nil, []cell{{2, 0}, {3, 4}}, false},
		{"touching orthogonally", []cell{{4, 5}, {4, 4}}, nil, []cell{{4, 4}, {4, 5}}, false},
		{"touching diagonally", []cell{{5, 5}, {4, 4}}, nil, []cell{{4, 4}, {5, 5}}, false},
		{"touching anti-diagonally", []cell{{2, 4}, {1, 5}}, nil, []cell{{1, 5}, {2, 4}}, false},
		{"one cat in three conflicts", []cell{{8, 2}, {4, 8}, {4, 2}, {3, 3}}, []cell{{0, 0}},
			[]cell{{3, 3}, {4, 2}, {4, 8}, {8, 2}}, false},
		{"nine correct cats", solution[:9], nil, []cell{}, false},
		{"full solution", solution, nil, []cell{}, true},
		{"full solution plus marks", solution, others, []cell{}, true},
		{"ten cats with a conflict", broken, nil, nil, false},
	}
	for _, c := range cases {
		oracle := hiddenRuleConflicts(reg, c.cats)
		want := c.want
		if want == nil {
			want = oracle
		} else if hiddenCellsString(oracle) != hiddenCellsString(want) {
			t.Fatalf("bad fixture %q: oracle gives %s, table says %s", c.name, hiddenCellsString(oracle), hiddenCellsString(want))
		}
		board := hiddenBoard(c.cats, c.marks)
		p := hiddenPuzzle(reg)
		type checkResult struct {
			conflicts []engine.Pos
			won       bool
		}
		res := hiddenCall(t, "Check("+c.name+")", func() checkResult {
			conflicts, won := engine.Check(p, board)
			return checkResult{conflicts, won}
		})
		got := make([]hiddenCell, len(res.conflicts))
		for i, pos := range res.conflicts {
			got[i] = hiddenCell{pos.Row, pos.Col}
		}
		if hiddenCellsString(got) != hiddenCellsString(want) {
			t.Errorf("%s: conflicts = %s, want %s", c.name, hiddenCellsString(got), hiddenCellsString(want))
		}
		if res.won != c.won {
			t.Errorf("%s: won = %v, want %v", c.name, res.won, c.won)
		}
	}
}
