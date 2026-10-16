package web_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"math/rand/v2"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"meowdoku/engine"
	"meowdoku/web"
)

// Watchdogs, not contract limits: they only stop a hung handler from stalling the package. Game
// creation runs Generate (up to 2 s by contract, several times slower under -race), so it and the
// tests' own Generate calls get more room.
const (
	hiddenRequestLimit = 20 * time.Second
	hiddenCreateLimit  = 60 * time.Second
)

// hiddenTry runs f in its own goroutine, so a panic or a hang in the code under test fails the
// calling test instead of crashing or stalling the whole package.
func hiddenTry[T any](limit time.Duration, f func() T) (v T, problem string) {
	type result struct {
		v        T
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
		res.v = f()
		res.returned = true
	}()
	select {
	case res := <-ch:
		if !res.returned {
			return v, "panicked: " + res.panicked
		}
		return res.v, ""
	case <-time.After(limit):
		return v, fmt.Sprintf("did not return within %v", limit)
	}
}

func hiddenCall[T any](t *testing.T, what string, limit time.Duration, f func() T) T {
	t.Helper()
	v, problem := hiddenTry(limit, f)
	if problem != "" {
		t.Fatalf("%s %s", what, problem)
	}
	return v
}

func hiddenHandler(t *testing.T) http.Handler {
	t.Helper()
	h := hiddenCall(t, "NewHandler()", hiddenRequestLimit, web.NewHandler)
	if h == nil {
		t.Fatalf("NewHandler() returned nil")
	}
	return h
}

func hiddenRegionsFor(t *testing.T, seed int64) hiddenLayout {
	t.Helper()
	p := hiddenCall(t, fmt.Sprintf("engine.Generate(%d)", seed), hiddenCreateLimit, func() engine.Puzzle { return engine.Generate(seed) })
	return p.Regions
}

type hiddenResp struct {
	code   int
	header http.Header
	body   string
}

func (r hiddenResp) hiddenMediaType() string {
	mt, _, err := mime.ParseMediaType(r.header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return mt
}

// hiddenFiresWon reports whether the HX-Trigger response headers fire an event named won, in
// either form htmx reads: a JSON object keyed by event name, or comma-separated event names.
func (r hiddenResp) hiddenFiresWon() bool {
	for _, v := range r.header.Values("HX-Trigger") {
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "{") {
			var events map[string]json.RawMessage
			if json.Unmarshal([]byte(v), &events) == nil {
				if _, ok := events["won"]; ok {
					return true
				}
			}
			continue
		}
		for _, name := range strings.Split(v, ",") {
			if strings.TrimSpace(name) == "won" {
				return true
			}
		}
	}
	return false
}

// hiddenDo serves one request through ServeHTTP, without sockets. It returns a problem instead
// of failing, so goroutines can use it.
func hiddenDo(h http.Handler, method, target string, form url.Values, hx bool) (hiddenResp, string) {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	limit := hiddenRequestLimit
	if method == "POST" && target == "/games" {
		limit = hiddenCreateLimit
	}
	rec := httptest.NewRecorder()
	_, problem := hiddenTry(limit, func() bool { h.ServeHTTP(rec, req); return true })
	if problem != "" {
		return hiddenResp{}, fmt.Sprintf("%s %s %s", method, target, problem)
	}
	return hiddenResp{rec.Code, rec.Header().Clone(), rec.Body.String()}, ""
}

// hiddenRequestAllowWon serves a request whose response may fire won: only the winning move.
func hiddenRequestAllowWon(t *testing.T, h http.Handler, method, target string, form url.Values, hx bool) hiddenResp {
	t.Helper()
	r, problem := hiddenDo(h, method, target, form, hx)
	if problem != "" {
		t.Fatalf("%s", problem)
	}
	return r
}

// hiddenRequest serves a request whose response must not fire won.
func hiddenRequest(t *testing.T, h http.Handler, method, target string, form url.Values, hx bool) hiddenResp {
	t.Helper()
	r := hiddenRequestAllowWon(t, h, method, target, form, hx)
	if r.hiddenFiresWon() {
		t.Fatalf("%s %s: HX-Trigger %v fires won; only the winning move may", method, target, r.header.Values("HX-Trigger"))
	}
	return r
}

func hiddenExpect(t *testing.T, r hiddenResp, what string, code int, mediaType string) {
	t.Helper()
	if r.code != code {
		t.Fatalf("%s: status %d, want %d; body: %.300s", what, r.code, code, r.body)
	}
	if mediaType != "" && r.hiddenMediaType() != mediaType {
		t.Fatalf("%s: Content-Type %q, want %s", what, r.header.Get("Content-Type"), mediaType)
	}
}

var hiddenGamePath = regexp.MustCompile(`^/games/([A-Za-z0-9_-]{1,64})$`)

func hiddenCreate(t *testing.T, h http.Handler, form url.Values) string {
	t.Helper()
	r := hiddenRequest(t, h, "POST", "/games", form, false)
	hiddenExpect(t, r, "POST /games "+form.Encode(), http.StatusSeeOther, "")
	id, why := hiddenGameID(r)
	if why != "" {
		t.Fatalf("POST /games: %s", why)
	}
	return id
}

// hiddenGameID reads the game id from a 303's Location, relative or absolute.
func hiddenGameID(r hiddenResp) (string, string) {
	loc, err := url.Parse(r.header.Get("Location"))
	if err != nil {
		return "", fmt.Sprintf("bad Location %q: %v", r.header.Get("Location"), err)
	}
	base, _ := url.Parse("http://example.com/games")
	m := hiddenGamePath.FindStringSubmatch(base.ResolveReference(loc).Path)
	if m == nil {
		return "", fmt.Sprintf("Location %q is not /games/{id} with id matching [A-Za-z0-9_-]{1,64}", r.header.Get("Location"))
	}
	return m[1], ""
}

func hiddenSeedForm(seed string) url.Values { return url.Values{"seed": {seed}} }

func hiddenMove(t *testing.T, h http.Handler, id string, row, col int, state string) hiddenResp {
	t.Helper()
	return hiddenRequest(t, h, "POST", fmt.Sprintf("/games/%s/cells/%d/%d", id, row, col), url.Values{"state": {state}}, true)
}

// State JSON.

type hiddenState struct {
	ID        string
	Seed      int64
	Regions   hiddenLayout
	Board     [hiddenN][hiddenN]string
	Conflicts []hiddenCell
	Won       bool
}

var hiddenJSONInt = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// hiddenParseState checks the field types as well as the values; extra fields and any field
// order are fine.
func hiddenParseState(body string) (hiddenState, error) {
	var st hiddenState
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return st, fmt.Errorf("not a JSON object: %v", err)
	}
	for _, k := range []string{"id", "seed", "regions", "board", "conflicts", "won"} {
		if _, ok := raw[k]; !ok {
			return st, fmt.Errorf("field %q is missing", k)
		}
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw["id"]), []byte(`"`)) {
		return st, fmt.Errorf("id is %s, want a JSON string", raw["id"])
	}
	if err := json.Unmarshal(raw["id"], &st.ID); err != nil {
		return st, fmt.Errorf("id: %v", err)
	}
	seed := string(bytes.TrimSpace(raw["seed"]))
	if !hiddenJSONInt.MatchString(seed) {
		return st, fmt.Errorf("seed is %s, want a JSON integer", seed)
	}
	s, err := strconv.ParseInt(seed, 10, 64)
	if err != nil {
		return st, fmt.Errorf("seed %s is not an int64", seed)
	}
	st.Seed = s
	switch won := string(bytes.TrimSpace(raw["won"])); won {
	case "true", "false":
		st.Won = won == "true"
	default:
		return st, fmt.Errorf("won is %s, want true or false", won)
	}
	// json.Unmarshal turns a null element into 0, so the integer arrays are checked for null first.
	for _, k := range []string{"regions", "conflicts"} {
		if bytes.Contains(raw[k], []byte("null")) {
			return st, fmt.Errorf("%s is %s, want integers, not null", k, raw[k])
		}
	}
	var regions [][]int
	if err := json.Unmarshal(raw["regions"], &regions); err != nil {
		return st, fmt.Errorf("regions: %v", err)
	}
	var board [][]string
	if err := json.Unmarshal(raw["board"], &board); err != nil {
		return st, fmt.Errorf("board: %v", err)
	}
	if len(regions) != hiddenN || len(board) != hiddenN {
		return st, fmt.Errorf("regions has %d rows and board %d, want 10 each", len(regions), len(board))
	}
	for r := range hiddenN {
		if len(regions[r]) != hiddenN || len(board[r]) != hiddenN {
			return st, fmt.Errorf("row %d of regions or board does not have 10 entries", r)
		}
		copy(st.Regions[r][:], regions[r])
		copy(st.Board[r][:], board[r])
	}
	var conflicts [][]int
	if err := json.Unmarshal(raw["conflicts"], &conflicts); err != nil {
		return st, fmt.Errorf("conflicts: %v", err)
	}
	if conflicts == nil {
		return st, fmt.Errorf("conflicts is %s, want an array ([] when there are none)", raw["conflicts"])
	}
	st.Conflicts = []hiddenCell{}
	for _, p := range conflicts {
		if len(p) != 2 {
			return st, fmt.Errorf("conflict %v is not a [row, col] pair", p)
		}
		st.Conflicts = append(st.Conflicts, hiddenCell{p[0], p[1]})
	}
	return st, nil
}

func hiddenGetState(t *testing.T, h http.Handler, id string) hiddenState {
	t.Helper()
	r := hiddenRequest(t, h, "GET", "/games/"+id+"/state", nil, false)
	hiddenExpect(t, r, "GET /games/{id}/state", http.StatusOK, "application/json")
	st, err := hiddenParseState(r.body)
	if err != nil {
		t.Fatalf("GET /games/{id}/state: %v; body: %.300s", err, r.body)
	}
	return st
}

func hiddenConflictSet(cs []hiddenCell) map[hiddenCell]bool {
	m := map[hiddenCell]bool{}
	for _, p := range cs {
		m[p] = true
	}
	return m
}

// Markup. A small tokenizer reads start and end tags in order, skipping comments, doctypes and
// the content of raw-text elements; attribute values may use any quoting and come in any order.

type hiddenTag struct {
	name        string
	attrs       map[string]string
	quotes      map[string]byte // the quote character of each attribute value, 0 when unquoted
	end         bool
	selfClosing bool
}

var (
	hiddenVoid    = map[string]bool{"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true, "input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true}
	hiddenRawText = map[string]bool{"script": true, "style": true, "textarea": true, "title": true}
)

func hiddenIsLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

func hiddenIsSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' }

func hiddenTokens(s string) []hiddenTag {
	var out []hiddenTag
	lower := strings.ToLower(s)
	for i := 0; i < len(s); {
		lt := strings.IndexByte(s[i:], '<')
		if lt < 0 {
			break
		}
		i += lt
		switch {
		case strings.HasPrefix(s[i:], "<!--"):
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				return out
			}
			i += 4 + end + 3
		case i+1 < len(s) && (s[i+1] == '!' || s[i+1] == '?'):
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				return out
			}
			i += end + 1
		case i+1 < len(s) && s[i+1] == '/':
			j := i + 2
			for j < len(s) && !hiddenIsSpace(s[j]) && s[j] != '>' {
				j++
			}
			end := strings.IndexByte(s[j:], '>')
			if end < 0 {
				return out
			}
			if name := strings.ToLower(s[i+2 : j]); name != "" {
				out = append(out, hiddenTag{name: name, end: true})
			}
			i = j + end + 1
		case i+1 < len(s) && hiddenIsLetter(s[i+1]):
			tag, next := hiddenStartTag(s, i)
			out = append(out, tag)
			i = next
			if hiddenRawText[tag.name] && !tag.selfClosing {
				end := strings.Index(lower[i:], "</"+tag.name)
				if end < 0 {
					return out
				}
				i += end
			}
		default:
			i++
		}
	}
	return out
}

// hiddenStartTag reads the start tag at s[i] and returns it with the index after its '>'.
func hiddenStartTag(s string, i int) (hiddenTag, int) {
	j := i + 1
	for j < len(s) && !hiddenIsSpace(s[j]) && s[j] != '>' && s[j] != '/' {
		j++
	}
	tag := hiddenTag{name: strings.ToLower(s[i+1 : j]), attrs: map[string]string{}, quotes: map[string]byte{}}
	for j < len(s) {
		for j < len(s) && (hiddenIsSpace(s[j]) || s[j] == '/') {
			j++
		}
		if j >= len(s) || s[j] == '>' {
			break
		}
		k := j
		for k < len(s) && !hiddenIsSpace(s[k]) && s[k] != '=' && s[k] != '>' && s[k] != '/' {
			k++
		}
		name := strings.ToLower(s[j:k])
		value, quote := "", byte(0)
		for k < len(s) && hiddenIsSpace(s[k]) {
			k++
		}
		if k < len(s) && s[k] == '=' {
			k++
			for k < len(s) && hiddenIsSpace(s[k]) {
				k++
			}
			if k < len(s) && (s[k] == '"' || s[k] == '\'') {
				quote = s[k]
				end := strings.IndexByte(s[k+1:], quote)
				if end < 0 {
					end = len(s) - k - 1
				}
				value = s[k+1 : k+1+end]
				k += end + 2
			} else {
				v := k
				for k < len(s) && !hiddenIsSpace(s[k]) && s[k] != '>' {
					k++
				}
				value = s[v:k]
			}
		}
		if name != "" {
			if _, dup := tag.attrs[name]; !dup {
				tag.attrs[name] = html.UnescapeString(value)
				tag.quotes[name] = quote
			}
		}
		if k == j {
			k++
		}
		j = k
	}
	if j > len(s) {
		j = len(s)
	}
	tag.selfClosing = j > 0 && j < len(s) && s[j-1] == '/'
	if j < len(s) {
		j++ // past '>'
	}
	return tag, j
}

type hiddenCellView struct {
	region   int
	state    string
	conflict bool
}

type hiddenBoardView struct {
	won   string
	cells map[hiddenCell]hiddenCellView
}

func (v hiddenBoardView) hiddenRegions() hiddenLayout {
	var reg hiddenLayout
	for p, c := range v.cells {
		reg[p.R][p.C] = c.region
	}
	return reg
}

var hiddenCellHooks = []string{"data-row", "data-col", "data-region", "data-state", "data-conflict"}

// hiddenParseBoard finds the single #board element and the cells inside it: elements nested in
// #board that carry data-row, data-col and data-state. Elements elsewhere are ignored, and end
// tags close any elements left open inside them, as browsers do.
func hiddenParseBoard(t *testing.T, what, body string) hiddenBoardView {
	t.Helper()
	var stack []string
	boardDepth := 0 // len(stack) once #board is open; 0 before it opens and after it closes
	boards := 0
	var board hiddenTag
	var cellTags []hiddenTag
	for _, tag := range hiddenTokens(body) {
		if tag.end {
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k] == tag.name {
					stack = stack[:k]
					break
				}
			}
			if boardDepth > 0 && len(stack) < boardDepth {
				boardDepth = 0
			}
			continue
		}
		_, hasRow := tag.attrs["data-row"]
		_, hasCol := tag.attrs["data-col"]
		_, hasState := tag.attrs["data-state"]
		if hasRow && hasCol && hasState && boardDepth > 0 {
			cellTags = append(cellTags, tag)
		}
		isBoard := tag.attrs["id"] == "board"
		if isBoard {
			boards++
			board = tag
		}
		if !hiddenVoid[tag.name] && !tag.selfClosing {
			stack = append(stack, tag.name)
			if isBoard && boards == 1 {
				boardDepth = len(stack)
			}
		}
	}
	if boards != 1 {
		t.Fatalf("%s: %d elements with id=\"board\", want 1; body: %.300s", what, boards, body)
	}
	view := hiddenBoardView{won: board.attrs["data-won"], cells: map[hiddenCell]hiddenCellView{}}
	if view.won != "true" && view.won != "false" {
		t.Fatalf("%s: #board has data-won=%q, want \"true\" or \"false\"", what, view.won)
	}
	if len(cellTags) != hiddenN*hiddenN {
		t.Fatalf("%s: %d cell elements inside #board (with data-row, data-col and data-state), want 100", what, len(cellTags))
	}
	digit := func(tag hiddenTag, name string) int {
		v, err := strconv.Atoi(tag.attrs[name])
		if err != nil || v < 0 || v >= hiddenN {
			t.Fatalf("%s: cell with %s=%q, want 0-9", what, name, tag.attrs[name])
		}
		return v
	}
	for _, tag := range cellTags {
		for _, hook := range hiddenCellHooks {
			if _, ok := tag.attrs[hook]; ok && tag.quotes[hook] != '"' {
				t.Fatalf("%s: a cell's %s is not in double quotes", what, hook)
			}
		}
		p := hiddenCell{digit(tag, "data-row"), digit(tag, "data-col")}
		if _, dup := view.cells[p]; dup {
			t.Fatalf("%s: two cell elements for row %d col %d", what, p.R, p.C)
		}
		state := tag.attrs["data-state"]
		if state != "empty" && state != "mark" && state != "cat" {
			t.Fatalf("%s: cell (%d,%d) has data-state=%q", what, p.R, p.C, state)
		}
		conflict, has := tag.attrs["data-conflict"]
		if has && conflict != "true" && conflict != "false" {
			t.Fatalf("%s: cell (%d,%d) has data-conflict=%q", what, p.R, p.C, conflict)
		}
		view.cells[p] = hiddenCellView{region: digit(tag, "data-region"), state: state, conflict: conflict == "true"}
	}
	return view
}

func hiddenExpectStates(t *testing.T, what string, view hiddenBoardView, st hiddenState, want map[hiddenCell]string) {
	t.Helper()
	for r := range hiddenN {
		for c := range hiddenN {
			w := want[hiddenCell{r, c}]
			if w == "" {
				w = "empty"
			}
			if got := view.cells[hiddenCell{r, c}].state; got != w {
				t.Fatalf("%s: markup cell (%d,%d) data-state=%q, want %q", what, r, c, got, w)
			}
			if got := st.Board[r][c]; got != w {
				t.Fatalf("%s: JSON board[%d][%d]=%q, want %q", what, r, c, got, w)
			}
		}
	}
}

// hiddenDocBase is the URL relative references on a page resolve against: the page URL, or
// the first <base href> on it.
func hiddenDocBase(pageURL string, tokens []hiddenTag) *url.URL {
	base, _ := url.Parse(pageURL)
	for _, tag := range tokens {
		if href, ok := tag.attrs["href"]; ok && tag.name == "base" && !tag.end {
			if b, err := base.Parse(href); err == nil {
				return b
			}
			break
		}
	}
	return base
}

// hiddenLocal resolves ref against the document base and returns its path when it stays on the
// page's own server: a <base href> that points elsewhere does not make another server local.
func hiddenLocal(pageURL string, base *url.URL, ref string) (string, bool) {
	page, perr := url.Parse(pageURL)
	u, err := base.Parse(ref)
	if perr != nil || err != nil || u.Scheme != page.Scheme || u.Host != page.Host {
		return "", false
	}
	return u.Path, true
}

// hiddenScriptPaths returns the paths of the scripts a page loads from this server.
func hiddenScriptPaths(pageURL, body string) map[string]bool {
	tokens := hiddenTokens(body)
	base := hiddenDocBase(pageURL, tokens)
	paths := map[string]bool{}
	for _, tag := range tokens {
		if src, ok := tag.attrs["src"]; ok && tag.name == "script" && src != "" {
			if p, local := hiddenLocal(pageURL, base, src); local {
				paths[p] = true
			}
		}
	}
	return paths
}

// hiddenStartsGame reports whether a page offers a way to start a game: a form that posts to
// /games, or an htmx element that does.
func hiddenStartsGame(pageURL, body string) bool {
	tokens := hiddenTokens(body)
	base := hiddenDocBase(pageURL, tokens)
	toGames := func(ref string) bool {
		p, local := hiddenLocal(pageURL, base, ref)
		return local && p == "/games"
	}
	// htmx reads hx-<name> and, when that is absent, data-hx-<name> (its default prefix).
	hx := func(tag hiddenTag, name string) string {
		if v, ok := tag.attrs["hx-"+name]; ok {
			return v
		}
		return tag.attrs["data-hx-"+name]
	}
	for _, tag := range tokens {
		switch {
		case tag.end:
		case tag.name == "form" && strings.EqualFold(tag.attrs["method"], "post") && toGames(tag.attrs["action"]):
			return true
		case hx(tag, "post") != "" && toGames(hx(tag, "post")):
			return true
		case hx(tag, "action") != "" && strings.EqualFold(hx(tag, "method"), "post") && toGames(hx(tag, "action")):
			return true
		}
	}
	return false
}

func hiddenExpectScripts(t *testing.T, what, pageURL, body string) {
	t.Helper()
	scripts := hiddenScriptPaths(pageURL, body)
	for _, s := range []string{"/static/htmx.min.js", "/static/alpine.min.js"} {
		if !scripts[s] {
			t.Errorf("%s: the page does not load %s from this server (local scripts: %v)", what, s, scripts)
		}
	}
}

func TestHiddenIndex(t *testing.T) {
	h := hiddenHandler(t)
	r := hiddenRequest(t, h, "GET", "/", nil, false)
	hiddenExpect(t, r, "GET /", http.StatusOK, "text/html")
	hiddenExpectScripts(t, "GET /", "http://example.com/", r.body)
	if !hiddenStartsGame("http://example.com/", r.body) {
		t.Errorf("GET /: no form or htmx element that posts to /games")
	}
}

func TestHiddenStatic(t *testing.T) {
	h := hiddenHandler(t)
	for _, name := range []string{"htmx.min.js", "alpine.min.js"} {
		want, err := os.ReadFile(filepath.Join("static", name))
		if err != nil {
			t.Fatalf("vendored file: %v", err)
		}
		r := hiddenRequest(t, h, "GET", "/static/"+name, nil, false)
		hiddenExpect(t, r, "GET /static/"+name, http.StatusOK, "")
		if r.body != string(want) {
			t.Errorf("GET /static/%s: %d bytes that differ from the vendored file (%d bytes)", name, len(r.body), len(want))
		}
	}
}

func TestHiddenCreateGame(t *testing.T) {
	h := hiddenHandler(t)
	id := hiddenCreate(t, h, hiddenSeedForm("42"))
	r := hiddenRequest(t, h, "GET", "/games/"+id, nil, false)
	hiddenExpect(t, r, "GET /games/{id}", http.StatusOK, "text/html")
	hiddenExpectScripts(t, "GET /games/{id}", "http://example.com/games/"+id, r.body)
	view := hiddenParseBoard(t, "GET /games/{id}", r.body)
	if view.won != "false" {
		t.Errorf("new game: #board data-won=%q, want \"false\"", view.won)
	}
	for p, c := range view.cells {
		if c.state != "empty" {
			t.Errorf("new game: cell (%d,%d) is %q, want empty", p.R, p.C, c.state)
		}
	}
	reg := view.hiddenRegions()
	if why := hiddenLayoutProblem(reg); why != "" {
		t.Errorf("new game: invalid regions in the markup: %s", why)
	}
	// data-region may number the regions differently; the partition must be the puzzle's.
	if hiddenCanonical(reg) != hiddenCanonical(hiddenRegionsFor(t, 42)) {
		t.Errorf("new game: the data-region partition differs from engine.Generate(42).Regions")
	}
}

func TestHiddenState(t *testing.T) {
	h := hiddenHandler(t)
	id := hiddenCreate(t, h, hiddenSeedForm("42"))
	st := hiddenGetState(t, h, id)
	if st.ID != id {
		t.Errorf("state id = %q, want %q", st.ID, id)
	}
	if st.Seed != 42 {
		t.Errorf("state seed = %d, want 42", st.Seed)
	}
	if st.Regions != hiddenRegionsFor(t, 42) {
		t.Errorf("state regions differ from engine.Generate(42).Regions")
	}
	for r := range hiddenN {
		for c := range hiddenN {
			if st.Board[r][c] != "empty" {
				t.Errorf("state board[%d][%d] = %q, want empty", r, c, st.Board[r][c])
			}
		}
	}
	if len(st.Conflicts) != 0 {
		t.Errorf("state conflicts = %v, want []", st.Conflicts)
	}
	if st.Won {
		t.Errorf("state won = true for a new game")
	}
	// A negative seed at the end of the int64 range must come back exactly.
	s := strconv.FormatInt(math.MinInt64, 10)
	if got := hiddenGetState(t, h, hiddenCreate(t, h, hiddenSeedForm(s))).Seed; got != math.MinInt64 {
		t.Errorf("seed=%s: state seed = %d", s, got)
	}
}

func TestHiddenRandomSeed(t *testing.T) {
	h := hiddenHandler(t)
	ids := map[string]bool{}
	seeds := map[int64]bool{}
	for _, form := range []url.Values{{}, hiddenSeedForm("")} {
		id := hiddenCreate(t, h, form)
		if ids[id] {
			t.Errorf("two games got the same id %q", id)
		}
		ids[id] = true
		st := hiddenGetState(t, h, id)
		seeds[st.Seed] = true
		if st.Regions != hiddenRegionsFor(t, st.Seed) {
			t.Errorf("random game (form %q): regions differ from engine.Generate(%d).Regions", form.Encode(), st.Seed)
		}
	}
	if len(seeds) != 2 {
		t.Errorf("two games without a seed got the same seed: it is not random")
	}
}

func TestHiddenMoves(t *testing.T) {
	h := hiddenHandler(t)
	id := hiddenCreate(t, h, hiddenSeedForm("42"))
	for _, state := range []string{"mark", "mark", "cat", "cat", "empty", "empty"} {
		what := "set (3,4) to " + state
		r := hiddenMove(t, h, id, 3, 4, state)
		hiddenExpect(t, r, what, http.StatusOK, "text/html")
		view := hiddenParseBoard(t, what, r.body)
		if view.won != "false" {
			t.Errorf("%s: data-won=%q", what, view.won)
		}
		want := map[hiddenCell]string{{3, 4}: state}
		st := hiddenGetState(t, h, id)
		hiddenExpectStates(t, what, view, st, want)
		page := hiddenRequest(t, h, "GET", "/games/"+id, nil, false)
		hiddenExpect(t, page, what+": GET /games/{id}", http.StatusOK, "text/html")
		hiddenExpectStates(t, what+": page", hiddenParseBoard(t, what+": page", page.body), st, want)
	}
}

// hiddenDiagonalPair finds two diagonal neighbours in different regions, so only the touching
// rule joins them.
func hiddenDiagonalPair(reg hiddenLayout) (hiddenCell, hiddenCell, bool) {
	for r := range hiddenN - 1 {
		for c := range hiddenN - 1 {
			if reg[r][c] != reg[r+1][c+1] {
				return hiddenCell{r, c}, hiddenCell{r + 1, c + 1}, true
			}
		}
	}
	return hiddenCell{}, hiddenCell{}, false
}

// hiddenRegionPair finds two cells in one region that share no row or column and do not touch.
func hiddenRegionPair(reg hiddenLayout) (hiddenCell, hiddenCell, bool) {
	for r1 := range hiddenN {
		for c1 := range hiddenN {
			for r2 := r1 + 2; r2 < hiddenN; r2++ {
				for c2 := range hiddenN {
					if reg[r1][c1] == reg[r2][c2] && (c2 < c1-1 || c2 > c1+1) {
						return hiddenCell{r1, c1}, hiddenCell{r2, c2}, true
					}
				}
			}
		}
	}
	return hiddenCell{}, hiddenCell{}, false
}

func TestHiddenConflicts(t *testing.T) {
	h := hiddenHandler(t)
	id := hiddenCreate(t, h, hiddenSeedForm("42"))
	reg := hiddenGetState(t, h, id).Regions
	if why := hiddenLayoutProblem(reg); why != "" {
		t.Fatalf("game regions are invalid: %s", why)
	}
	scenarios := []struct {
		name string
		cats []hiddenCell
	}{
		{"two cats in one row", []hiddenCell{{2, 1}, {2, 6}}},
		{"two cats in one column", []hiddenCell{{1, 3}, {7, 3}}},
	}
	if p, q, ok := hiddenDiagonalPair(reg); ok {
		scenarios = append(scenarios, struct {
			name string
			cats []hiddenCell
		}{"two cats touching diagonally, in different regions", []hiddenCell{p, q}})
	}
	if p, q, ok := hiddenRegionPair(reg); ok {
		scenarios = append(scenarios, struct {
			name string
			cats []hiddenCell
		}{"two cats in one region", []hiddenCell{p, q}})
	}
	for _, sc := range scenarios {
		var r hiddenResp
		for _, p := range sc.cats {
			r = hiddenMove(t, h, id, p.R, p.C, "cat")
			hiddenExpect(t, r, sc.name, http.StatusOK, "text/html")
		}
		want := hiddenConflictSet(hiddenRuleConflicts(reg, sc.cats))
		for p, c := range hiddenParseBoard(t, sc.name, r.body).cells {
			if c.conflict != want[p] {
				t.Errorf("%s: cell (%d,%d) conflict=%v, want %v", sc.name, p.R, p.C, c.conflict, want[p])
			}
		}
		st := hiddenGetState(t, h, id)
		got := hiddenConflictSet(st.Conflicts)
		if len(st.Conflicts) != len(want) || len(got) != len(want) {
			t.Errorf("%s: JSON conflicts = %v, want %s", sc.name, st.Conflicts, hiddenCellsString(hiddenRuleConflicts(reg, sc.cats)))
		}
		for p := range want {
			if !got[p] {
				t.Errorf("%s: JSON conflicts = %v, missing %v", sc.name, st.Conflicts, p)
			}
		}
		for _, p := range sc.cats {
			r = hiddenMove(t, h, id, p.R, p.C, "empty")
		}
		for p, c := range hiddenParseBoard(t, sc.name+", cleared", r.body).cells {
			if c.conflict {
				t.Errorf("%s, cleared: cell (%d,%d) still marked as a conflict", sc.name, p.R, p.C)
			}
		}
		if st := hiddenGetState(t, h, id); len(st.Conflicts) != 0 {
			t.Errorf("%s, cleared: JSON conflicts = %v, want []", sc.name, st.Conflicts)
		}
	}
}

func TestHiddenWin(t *testing.T) {
	h := hiddenHandler(t)
	id := hiddenCreate(t, h, hiddenSeedForm("7"))
	st := hiddenGetState(t, h, id)
	if why := hiddenLayoutProblem(st.Regions); why != "" {
		t.Fatalf("game regions are invalid: %s", why)
	}
	n, sol := hiddenSolutions(st.Regions, 1)
	if n == 0 {
		t.Fatalf("the game for seed 7 has no solution")
	}
	rng := rand.New(rand.NewPCG(7, 2026))
	var cats, others []hiddenCell
	for r := range hiddenN {
		for c := range hiddenN {
			if sol[r] == c {
				cats = append(cats, hiddenCell{r, c})
			} else {
				others = append(others, hiddenCell{r, c})
			}
		}
	}
	rng.Shuffle(len(cats), func(i, j int) { cats[i], cats[j] = cats[j], cats[i] })
	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	marks := others[:12]
	type move struct {
		p     hiddenCell
		state string
	}
	var moves []move
	for _, p := range marks {
		moves = append(moves, move{p, "mark"})
	}
	for _, p := range cats[:9] {
		moves = append(moves, move{p, "cat"})
	}
	rng.Shuffle(len(moves), func(i, j int) { moves[i], moves[j] = moves[j], moves[i] })
	moves = append(moves, move{cats[9], "cat"})

	for i, m := range moves {
		what := fmt.Sprintf("move %d (%d,%d)=%s", i+1, m.p.R, m.p.C, m.state)
		r := hiddenRequestAllowWon(t, h, "POST", fmt.Sprintf("/games/%s/cells/%d/%d", id, m.p.R, m.p.C), url.Values{"state": {m.state}}, true)
		hiddenExpect(t, r, what, http.StatusOK, "text/html")
		last := i == len(moves)-1
		if r.hiddenFiresWon() != last {
			t.Errorf("%s: HX-Trigger fires won = %v, want %v (headers %v)", what, !last, last, r.header.Values("HX-Trigger"))
		}
		if last {
			if view := hiddenParseBoard(t, "winning move", r.body); view.won != "true" {
				t.Errorf("winning move: #board data-won=%q, want \"true\"", view.won)
			}
		}
	}
	won := hiddenGetState(t, h, id)
	if !won.Won {
		t.Fatalf("after the solution: JSON won = false")
	}
	page := hiddenRequest(t, h, "GET", "/games/"+id, nil, false)
	hiddenExpect(t, page, "GET /games/{id} after the win", http.StatusOK, "text/html")
	if view := hiddenParseBoard(t, "page after the win", page.body); view.won != "true" {
		t.Errorf("page after the win: data-won=%q, want \"true\"", view.won)
	}

	for _, m := range []move{{cats[0], "empty"}, {cats[1], "cat"}, {marks[0], "cat"}, {others[20], "mark"}} {
		what := fmt.Sprintf("after the win, (%d,%d)=%s", m.p.R, m.p.C, m.state)
		if r := hiddenMove(t, h, id, m.p.R, m.p.C, m.state); r.code != http.StatusConflict {
			t.Errorf("%s: status %d, want 409", what, r.code)
		}
	}
	after := hiddenGetState(t, h, id)
	if after.Board != won.Board || after.Won != won.Won || len(after.Conflicts) != len(won.Conflicts) {
		t.Errorf("a cell POST after the win changed the game")
	}
}

func TestHiddenErrors(t *testing.T) {
	h := hiddenHandler(t)
	for _, req := range []struct{ method, path string }{
		{"GET", "/games/nosuchgame"},
		{"GET", "/games/nosuchgame/state"},
		{"POST", "/games/nosuchgame/cells/0/0"},
	} {
		var form url.Values
		if req.method == "POST" {
			form = url.Values{"state": {"mark"}}
		}
		if r := hiddenRequest(t, h, req.method, req.path, form, req.method == "POST"); r.code != http.StatusNotFound {
			t.Errorf("%s %s: status %d, want 404", req.method, req.path, r.code)
		}
	}

	id := hiddenCreate(t, h, hiddenSeedForm("42"))
	for _, bad := range []string{"10", "-1", "x"} {
		for _, path := range []string{"/cells/" + bad + "/0", "/cells/0/" + bad} {
			r := hiddenRequest(t, h, "POST", "/games/"+id+path, url.Values{"state": {"mark"}}, true)
			if r.code != http.StatusBadRequest {
				t.Errorf("POST /games/{id}%s: status %d, want 400", path, r.code)
			}
		}
	}
	for _, bad := range []string{"bogus", "", "CAT"} {
		r := hiddenRequest(t, h, "POST", "/games/"+id+"/cells/0/0", url.Values{"state": {bad}}, true)
		if r.code != http.StatusBadRequest {
			t.Errorf("POST /games/{id}/cells/0/0 with state=%q: status %d, want 400", bad, r.code)
		}
	}
	st := hiddenGetState(t, h, id)
	for r := range hiddenN {
		for c := range hiddenN {
			if st.Board[r][c] != "empty" {
				t.Errorf("after rejected moves: board[%d][%d] = %q, want empty", r, c, st.Board[r][c])
			}
		}
	}

	for _, bad := range []string{"abc", "1.5", "9223372036854775808"} {
		if r := hiddenRequest(t, h, "POST", "/games", hiddenSeedForm(bad), false); r.code != http.StatusBadRequest {
			t.Errorf("POST /games seed=%s: status %d, want 400", bad, r.code)
		}
	}

	// Any other method answers 405; HEAD is accepted wherever GET is, so it is not tried. These
	// are DELETE and PUT, GET on POST-only routes, and POST on GET-only routes.
	for _, req := range []struct{ method, path string }{
		{"GET", "/games/" + id + "/cells/0/0"},
		{"GET", "/games"},
		{"POST", "/"},
		{"DELETE", "/games/" + id},
		{"PUT", "/games/" + id + "/state"},
		{"POST", "/static/htmx.min.js"},
	} {
		if r := hiddenRequest(t, h, req.method, req.path, nil, false); r.code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: status %d, want 405", req.method, req.path, r.code)
		}
	}
}

func TestHiddenIsolation(t *testing.T) {
	h1, h2 := hiddenHandler(t), hiddenHandler(t)
	a := hiddenCreate(t, h1, hiddenSeedForm("42"))
	if r := hiddenRequest(t, h2, "GET", "/games/"+a+"/state", nil, false); r.code != http.StatusNotFound {
		t.Errorf("a game from one NewHandler() is visible in another: status %d, want 404", r.code)
	}
	b := hiddenCreate(t, h1, hiddenSeedForm("42"))
	if a == b {
		t.Fatalf("two games got the same id %q", a)
	}
	hiddenExpect(t, hiddenMove(t, h1, a, 5, 5, "cat"), "move in game a", http.StatusOK, "text/html")
	if st := hiddenGetState(t, h1, b); st.Board[5][5] != "empty" {
		t.Errorf("a move in one game changed another: board[5][5] = %q", st.Board[5][5])
	}
	if st := hiddenGetState(t, h1, a); st.Board[5][5] != "cat" {
		t.Errorf("game a: board[5][5] = %q, want cat", st.Board[5][5])
	}
}

// hiddenProblems collects problems from goroutines; a worker stops at its first one.
type hiddenProblems struct {
	mu   sync.Mutex
	list []string
}

func (p *hiddenProblems) add(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.list = append(p.list, s)
}

// hiddenWaitFor waits for wg, at most limit, and reports whether it finished.
func hiddenWaitFor(wg *sync.WaitGroup, limit time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(limit):
		return false
	}
}

// TestHiddenConcurrent must stay the last test in this file: a store that writes its map
// without a lock can crash the test binary ("concurrent map writes"), which nothing recovers.
func TestHiddenConcurrent(t *testing.T) {
	h := hiddenHandler(t)
	id := hiddenCreate(t, h, hiddenSeedForm("42"))

	// Phase 1: random marks and empties on one game, reads of it, and new games created and
	// read alongside. Every worker stops at its first problem or when the phase is over.
	stopAt := time.Now().Add(60 * time.Second)
	problems := &hiddenProblems{}
	var wg sync.WaitGroup
	for g := range 64 {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(uint64(g), 64))
			for range 20 {
				if time.Now().After(stopAt) {
					return
				}
				state := "mark"
				if rng.IntN(2) == 0 {
					state = "empty"
				}
				path := fmt.Sprintf("/games/%s/cells/%d/%d", id, rng.IntN(hiddenN), rng.IntN(hiddenN))
				r, problem := hiddenDo(h, "POST", path, url.Values{"state": {state}}, true)
				if problem == "" && r.code != http.StatusOK {
					problem = fmt.Sprintf("POST %s state=%s: status %d", path, state, r.code)
				}
				if problem != "" {
					problems.add(problem)
					return
				}
			}
		})
	}
	// The readers keep going until every concurrent creation has finished (and at least 20
	// times), so each insert into the game store overlaps reads of it.
	var creators sync.WaitGroup
	creatorsDone := make(chan struct{})
	for range 16 {
		wg.Go(func() {
			for i := 0; ; i++ {
				if time.Now().After(stopAt) {
					return
				}
				select {
				case <-creatorsDone:
					if i >= 20 {
						return
					}
				default:
				}
				r, problem := hiddenDo(h, "GET", "/games/"+id+"/state", nil, false)
				if problem == "" && r.code != http.StatusOK {
					problem = fmt.Sprintf("GET state: status %d", r.code)
				}
				if problem != "" {
					problems.add(problem)
					return
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
	for g := range 4 {
		creators.Add(1)
		wg.Go(func() {
			defer creators.Done()
			seed := int64(100 + g)
			r, problem := hiddenDo(h, "POST", "/games", hiddenSeedForm(strconv.FormatInt(seed, 10)), false)
			if problem == "" && r.code != http.StatusSeeOther {
				problem = fmt.Sprintf("concurrent POST /games: status %d", r.code)
			}
			if problem != "" {
				problems.add(problem)
				return
			}
			newID, why := hiddenGameID(r)
			if why != "" {
				problems.add("concurrent POST /games: " + why)
				return
			}
			s, problem := hiddenDo(h, "GET", "/games/"+newID+"/state", nil, false)
			if problem == "" && s.code != http.StatusOK {
				problem = fmt.Sprintf("GET a concurrently created game: status %d", s.code)
			}
			if problem != "" {
				problems.add(problem)
				return
			}
			if st, err := hiddenParseState(s.body); err != nil || st.Seed != seed {
				problems.add(fmt.Sprintf("concurrently created game: seed %d, err %v; want seed %d", st.Seed, err, seed))
			}
		})
	}
	go func() { creators.Wait(); close(creatorsDone) }()
	if !hiddenWaitFor(&wg, 60*time.Second+hiddenCreateLimit+5*time.Second) {
		t.Fatalf("concurrent requests did not finish")
	}
	for _, p := range problems.list {
		t.Errorf("concurrent requests: %s", p)
	}
	if len(problems.list) > 0 {
		return
	}

	// Phase 2: ten workers each own one row, change it at random, and leave it in a known
	// state. A lost update shows in the final board.
	want := map[hiddenCell]string{}
	for r := range hiddenN {
		for c := range hiddenN {
			if (r*3+c)%4 == 0 {
				want[hiddenCell{r, c}] = "mark"
			}
		}
	}
	stopAt = time.Now().Add(60 * time.Second)
	for row := range hiddenN {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(uint64(row), 2))
			set := func(c int, state string) bool {
				path := fmt.Sprintf("/games/%s/cells/%d/%d", id, row, c)
				r, problem := hiddenDo(h, "POST", path, url.Values{"state": {state}}, true)
				if problem == "" && r.code != http.StatusOK {
					problem = fmt.Sprintf("POST %s state=%s: status %d", path, state, r.code)
				}
				if problem != "" {
					problems.add(problem)
					return false
				}
				return true
			}
			for range 10 {
				for c := range hiddenN {
					if time.Now().After(stopAt) || !set(c, []string{"mark", "empty"}[rng.IntN(2)]) {
						return
					}
				}
			}
			for c := range hiddenN {
				state := want[hiddenCell{row, c}]
				if state == "" {
					state = "empty"
				}
				if !set(c, state) {
					return
				}
			}
		})
	}
	if !hiddenWaitFor(&wg, 60*time.Second+hiddenRequestLimit+5*time.Second) {
		t.Fatalf("concurrent row writers did not finish")
	}
	for _, p := range problems.list {
		t.Errorf("concurrent row writers: %s", p)
	}
	st := hiddenGetState(t, h, id)
	for r := range hiddenN {
		for c := range hiddenN {
			w := want[hiddenCell{r, c}]
			if w == "" {
				w = "empty"
			}
			if st.Board[r][c] != w {
				t.Errorf("after concurrent row writers: board[%d][%d] = %q, want %q (a lost update)", r, c, st.Board[r][c], w)
			}
		}
	}
	if len(st.Conflicts) != 0 || st.Won {
		t.Errorf("after the concurrent writes: conflicts %v, won %v; want none, false", st.Conflicts, st.Won)
	}
}
