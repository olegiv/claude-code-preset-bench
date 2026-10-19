package store

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	t := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return func() time.Time {
		t = t.Add(time.Second)
		return t
	}
}

func TestAddNormalizes(t *testing.T) {
	s := NewWithClock(fixedClock())
	l, err := s.Add("ann", " https://example.com/a#frag ", "  A page ", []string{"Go", " go", "", "web"})
	if err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://example.com/a" {
		t.Errorf("URL = %q", l.URL)
	}
	if l.Title != "A page" {
		t.Errorf("Title = %q", l.Title)
	}
	if !slices.Equal(l.Tags, []string{"go", "web"}) {
		t.Errorf("Tags = %q", l.Tags)
	}
	if l.Owner != "ann" || l.ID != "l1" {
		t.Errorf("Owner, ID = %q, %q", l.Owner, l.ID)
	}
}

func TestAddRejects(t *testing.T) {
	s := New()
	cases := []struct {
		name  string
		url   string
		title string
		tags  []string
		field string
	}{
		{"empty url", "", "", nil, "url"},
		{"ftp", "ftp://example.com", "", nil, "url"},
		{"no host", "https://", "", nil, "url"},
		{"long title", "https://example.com", strings.Repeat("x", MaxTitleLen+1), nil, "title"},
		{"long tag", "https://example.com", "", []string{strings.Repeat("t", MaxTagLen+1)}, "tags"},
		{"many tags", "https://example.com", "", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, "tags"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.Add("ann", c.url, c.title, c.tags)
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("err = %v, want a ValidationError on %s", err, c.field)
			}
		})
	}
}

func TestOwnersAreSeparate(t *testing.T) {
	s := New()
	l, err := s.Add("ann", "https://example.com", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("bob", l.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob's Get: err = %v", err)
	}
	if err := s.Delete("bob", l.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob's Delete: err = %v", err)
	}
	if got := s.List("bob", ""); len(got) != 0 {
		t.Errorf("bob's List = %v", got)
	}
	if err := s.Delete("ann", l.ID); err != nil {
		t.Errorf("ann's Delete: %v", err)
	}
	if _, err := s.Get("ann", l.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: err = %v", err)
	}
}

func TestListOrderAndTag(t *testing.T) {
	s := NewWithClock(fixedClock())
	for i, tags := range [][]string{{"go"}, {"web"}, {"go", "web"}} {
		if _, err := s.Add("ann", "https://example.com/"+string(rune('a'+i)), "", tags); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(links []Link) []string {
		var out []string
		for _, l := range links {
			out = append(out, l.ID)
		}
		return out
	}
	if got := ids(s.List("ann", "")); !slices.Equal(got, []string{"l3", "l2", "l1"}) {
		t.Errorf("List = %v", got)
	}
	if got := ids(s.List("ann", " GO ")); !slices.Equal(got, []string{"l3", "l1"}) {
		t.Errorf("List(go) = %v", got)
	}
}

func TestReturnsCopies(t *testing.T) {
	s := New()
	l, err := s.Add("ann", "https://example.com", "", []string{"go"})
	if err != nil {
		t.Fatal(err)
	}
	l.Tags[0] = "changed"
	got, err := s.Get("ann", l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tags[0] != "go" {
		t.Errorf("the store shares its tags slice: %q", got.Tags)
	}
}

func TestConcurrentAdds(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := s.Add("ann", "https://example.com", "", nil); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := len(s.List("ann", "")); n != 20 {
		t.Errorf("%d links, want 20", n)
	}
}
