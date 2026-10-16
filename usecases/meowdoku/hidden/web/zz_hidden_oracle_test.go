package web_test

// Test-only oracle, written from the rules in the prompt. It never calls the code under test.
// The same helpers are duplicated in the web package's hidden tests.

import (
	"fmt"
	"sort"
	"strings"
)

const hiddenN = 10

type hiddenLayout = [hiddenN][hiddenN]int

type hiddenCell struct{ R, C int }

// hiddenLayoutProblem returns "" when every id is in 0..9, all ten ids are used and every
// region is 4-connected, and a description of the first problem otherwise.
func hiddenLayoutProblem(reg hiddenLayout) string {
	var count [hiddenN]int
	for r := range hiddenN {
		for c := range hiddenN {
			id := reg[r][c]
			if id < 0 || id >= hiddenN {
				return fmt.Sprintf("cell (%d,%d) has region id %d", r, c, id)
			}
			count[id]++
		}
	}
	for id := range hiddenN {
		if count[id] == 0 {
			return fmt.Sprintf("region %d is unused", id)
		}
	}
	var seen [hiddenN][hiddenN]bool
	for id := range hiddenN {
		var stack []hiddenCell
		for r := 0; r < hiddenN && stack == nil; r++ {
			for c := range hiddenN {
				if reg[r][c] == id {
					stack = []hiddenCell{{r, c}}
					seen[r][c] = true
					break
				}
			}
		}
		reached := 0
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			reached++
			for _, d := range [4]hiddenCell{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				q := hiddenCell{p.R + d.R, p.C + d.C}
				if q.R >= 0 && q.R < hiddenN && q.C >= 0 && q.C < hiddenN && !seen[q.R][q.C] && reg[q.R][q.C] == id {
					seen[q.R][q.C] = true
					stack = append(stack, q)
				}
			}
		}
		if reached != count[id] {
			return fmt.Sprintf("region %d is not 4-connected (%d of %d cells reachable)", id, reached, count[id])
		}
	}
	return ""
}

// hiddenSolutions counts solutions, stopping at limit (limit <= 0 counts all), and returns the
// first one found as the column of the cat in each row. Call it only on valid layouts.
func hiddenSolutions(reg hiddenLayout, limit int) (count int, first [hiddenN]int) {
	var cols [hiddenN]int
	var rec func(row int, usedCols, usedRegions uint32) bool
	rec = func(row int, usedCols, usedRegions uint32) bool {
		if row == hiddenN {
			if count == 0 {
				first = cols
			}
			count++
			return limit > 0 && count >= limit
		}
		for c := range hiddenN {
			if usedCols&(1<<c) != 0 || usedRegions&(1<<reg[row][c]) != 0 {
				continue
			}
			if row > 0 && c >= cols[row-1]-1 && c <= cols[row-1]+1 {
				continue
			}
			cols[row] = c
			if rec(row+1, usedCols|1<<c, usedRegions|1<<reg[row][c]) {
				return true
			}
		}
		return false
	}
	rec(0, 0, 0)
	return count, first
}

// hiddenRuleConflicts returns every cat that shares a row, column or region with another cat,
// or touches one (8 neighbours), sorted by row then column.
func hiddenRuleConflicts(reg hiddenLayout, cats []hiddenCell) []hiddenCell {
	bad := map[hiddenCell]bool{}
	for i := 0; i < len(cats); i++ {
		for j := i + 1; j < len(cats); j++ {
			a, b := cats[i], cats[j]
			dr, dc := a.R-b.R, a.C-b.C
			if dr < 0 {
				dr = -dr
			}
			if dc < 0 {
				dc = -dc
			}
			if a.R == b.R || a.C == b.C || reg[a.R][a.C] == reg[b.R][b.C] || (dr <= 1 && dc <= 1) {
				bad[a], bad[b] = true, true
			}
		}
	}
	out := make([]hiddenCell, 0, len(bad))
	for p := range bad {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].R != out[j].R {
			return out[i].R < out[j].R
		}
		return out[i].C < out[j].C
	})
	return out
}

// hiddenIsSolution reports whether cols places one cat per row, column and region with no two touching.
func hiddenIsSolution(reg hiddenLayout, cols [hiddenN]int) bool {
	cats := make([]hiddenCell, 0, hiddenN)
	for r, c := range cols {
		if c < 0 || c >= hiddenN {
			return false
		}
		cats = append(cats, hiddenCell{r, c})
	}
	return len(hiddenRuleConflicts(reg, cats)) == 0
}

func hiddenCellsString(cs []hiddenCell) string {
	parts := make([]string, len(cs))
	for i, p := range cs {
		parts[i] = fmt.Sprintf("(%d,%d)", p.R, p.C)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// Fixtures. Each test that uses one checks it with the oracle first.

// hiddenRowsLayout: region id = row. Its solutions are the permutations of 0..9 with no two
// neighbouring values adjacent: OEIS A002464, 479 306 for n = 10.
func hiddenRowsLayout() hiddenLayout {
	var reg hiddenLayout
	for r := range hiddenN {
		for c := range hiddenN {
			reg[r][c] = r
		}
	}
	return reg
}

func hiddenColsLayout() hiddenLayout {
	var reg hiddenLayout
	for r := range hiddenN {
		for c := range hiddenN {
			reg[r][c] = c
		}
	}
	return reg
}

// hiddenBlocksLayout: ten 2x5 blocks, two per pair of rows.
func hiddenBlocksLayout() hiddenLayout {
	var reg hiddenLayout
	for r := range hiddenN {
		for c := range hiddenN {
			reg[r][c] = (r/2)*2 + c/5
		}
	}
	return reg
}

// hiddenForcedSolution is a valid placement; hiddenForcedLayout makes it the only solution by
// giving the cats of rows 0-8 one-cell regions and putting every other cell in region 9.
var hiddenForcedSolution = [hiddenN]int{0, 2, 4, 6, 8, 1, 3, 5, 7, 9}

func hiddenForcedLayout() hiddenLayout {
	var reg hiddenLayout
	for r := range hiddenN {
		for c := range hiddenN {
			reg[r][c] = 9
		}
	}
	for r := 0; r < 9; r++ {
		reg[r][hiddenForcedSolution[r]] = r
	}
	return reg
}

// hiddenNoSolutionLayout: regions 0 and 1 are the single cells (0,0) and (1,1), which touch
// diagonally, so no placement exists. Every region is still connected.
func hiddenNoSolutionLayout() hiddenLayout {
	var reg hiddenLayout
	for r := range hiddenN {
		for c := range hiddenN {
			switch {
			case r == 0 && c == 0:
				reg[r][c] = 0
			case r == 1 && c == 1:
				reg[r][c] = 1
			case r == 0 || (r == 1 && c >= 2):
				reg[r][c] = 2
			case r == 1 && c == 0, r == 2:
				reg[r][c] = 3
			case r >= 8:
				reg[r][c] = 9
			default:
				reg[r][c] = r + 1 // rows 3..7 -> regions 4..8
			}
		}
	}
	return reg
}

// hiddenCanonical relabels regions in order of first appearance, so two layouts compare equal
// when they are the same partition under different ids.
func hiddenCanonical(reg hiddenLayout) hiddenLayout {
	relabel := map[int]int{}
	var out hiddenLayout
	for r := range hiddenN {
		for c := range hiddenN {
			id, ok := relabel[reg[r][c]]
			if !ok {
				id = len(relabel)
				relabel[reg[r][c]] = id
			}
			out[r][c] = id
		}
	}
	return out
}
