package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"linkbox/internal/auth"
	"linkbox/internal/store"
)

const (
	annKey = "annkey-0123456789"
	bobKey = "bobkey-0123456789"
)

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	keys, err := auth.Parse(strings.NewReader(annKey + ":ann\n" + bobKey + ":bob\n"))
	if err != nil {
		t.Fatal(err)
	}
	st := store.New()
	srv := httptest.NewServer(NewServer(st, keys, slog.New(slog.DiscardHandler)).Handler())
	t.Cleanup(srv.Close)
	return srv, st
}

func do(t *testing.T, srv *httptest.Server, method, path, key, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("%s %s: body %q is not a JSON object", method, path, data)
		}
	}
	return resp.StatusCode, out
}

func TestHealthzNeedsNoKey(t *testing.T) {
	srv, _ := newTestServer(t)
	if status, _ := do(t, srv, "GET", "/healthz", "", ""); status != http.StatusOK {
		t.Errorf("status = %d", status)
	}
}

func TestUnknownKey(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, key := range []string{"", "nobody-0123456789"} {
		if status, _ := do(t, srv, "GET", "/links", key, ""); status != http.StatusUnauthorized {
			t.Errorf("key %q: status = %d", key, status)
		}
	}
}

func TestCreateGetDelete(t *testing.T) {
	srv, _ := newTestServer(t)
	status, body := do(t, srv, "POST", "/links", annKey, `{"url":"https://example.com","title":"Example","tags":["Go"]}`)
	if status != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %v", status, body)
	}
	id := body["id"].(string)
	if status, body := do(t, srv, "GET", "/links/"+id, annKey, ""); status != http.StatusOK || body["title"] != "Example" {
		t.Errorf("get: status = %d, body = %v", status, body)
	}
	if status, _ := do(t, srv, "GET", "/links/"+id, bobKey, ""); status != http.StatusNotFound {
		t.Errorf("bob's get: status = %d", status)
	}
	if status, _ := do(t, srv, "DELETE", "/links/"+id, bobKey, ""); status != http.StatusNotFound {
		t.Errorf("bob's delete: status = %d", status)
	}
	if status, _ := do(t, srv, "DELETE", "/links/"+id, annKey, ""); status != http.StatusNoContent {
		t.Errorf("delete: status = %d", status)
	}
	if status, _ := do(t, srv, "GET", "/links/"+id, annKey, ""); status != http.StatusNotFound {
		t.Errorf("get after delete: status = %d", status)
	}
}

func TestCreateRejects(t *testing.T) {
	srv, _ := newTestServer(t)
	cases := map[string]int{
		`not json`:                          http.StatusBadRequest,
		`{"url":"https://a.example"}{}`:     http.StatusBadRequest,
		`{"url":"https://a.example","x":1}`: http.StatusBadRequest,
		`{"url":"ftp://a.example"}`:         http.StatusUnprocessableEntity,
	}
	for body, want := range cases {
		if status, _ := do(t, srv, "POST", "/links", annKey, body); status != want {
			t.Errorf("body %s: status = %d, want %d", body, status, want)
		}
	}
}

func TestListFiltersByTagAndOwner(t *testing.T) {
	srv, st := newTestServer(t)
	for _, tags := range [][]string{{"go"}, {"web"}} {
		if _, err := st.Add("ann", "https://example.com", "", tags); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.Add("bob", "https://example.org", "", []string{"go"}); err != nil {
		t.Fatal(err)
	}
	_, body := do(t, srv, "GET", "/links?tag=go", annKey, "")
	links := body["links"].([]any)
	if len(links) != 1 || links[0].(map[string]any)["owner"] != "ann" {
		t.Errorf("links = %v", links)
	}
	_, body = do(t, srv, "GET", "/links", bobKey, "")
	if n := len(body["links"].([]any)); n != 1 {
		t.Errorf("bob sees %d links", n)
	}
}
