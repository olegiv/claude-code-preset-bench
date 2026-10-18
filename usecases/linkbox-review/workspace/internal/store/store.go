// Package store keeps links in memory.
package store

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Limits on what a link may hold.
const (
	MaxTitleLen = 200
	MaxTags     = 10
	MaxTagLen   = 32
)

// ErrNotFound reports a link that does not exist or belongs to someone else.
var ErrNotFound = errors.New("link not found")

// ValidationError reports input that a link cannot hold.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

// Link is a saved bookmark.
type Link struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

// Store holds links in memory. It is safe for concurrent use; every method returns copies, so callers
// cannot change stored links behind its back.
type Store struct {
	mu    sync.RWMutex
	links map[string]*Link
	seq   int
	now   func() time.Time
}

// New returns an empty store that stamps links with the current time.
func New() *Store {
	return NewWithClock(time.Now)
}

// NewWithClock returns an empty store that stamps links with now().
func NewWithClock(now func() time.Time) *Store {
	return &Store{links: map[string]*Link{}, now: now}
}

// Add validates and stores a new link for owner.
func (s *Store) Add(owner, rawURL, title string, tags []string) (Link, error) {
	u, err := normalizeURL(rawURL)
	if err != nil {
		return Link{}, err
	}
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > MaxTitleLen {
		return Link{}, &ValidationError{"title", fmt.Sprintf("longer than %d characters", MaxTitleLen)}
	}
	tags, err = normalizeTags(tags)
	if err != nil {
		return Link{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	l := &Link{
		ID:        fmt.Sprintf("l%d", s.seq),
		Owner:     owner,
		URL:       u,
		Title:     title,
		Tags:      tags,
		CreatedAt: s.now().UTC(),
	}
	s.links[l.ID] = l
	return copyLink(l), nil
}

// Get returns owner's link with the given id.
func (s *Store) Get(owner, id string) (Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.links[id]
	if !ok || l.Owner != owner {
		return Link{}, ErrNotFound
	}
	return copyLink(l), nil
}

// Delete removes owner's link with the given id.
func (s *Store) Delete(owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[id]
	if !ok || l.Owner != owner {
		return ErrNotFound
	}
	delete(s.links, id)
	return nil
}

// List returns owner's links, newest first; with a non-empty tag, only the links that have it.
func (s *Store) List(owner, tag string) []Link {
	tag = strings.ToLower(strings.TrimSpace(tag))
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Link
	for _, l := range s.links {
		if l.Owner != owner {
			continue
		}
		if tag != "" && !slices.Contains(l.Tags, tag) {
			continue
		}
		out = append(out, copyLink(l))
	}
	sortNewestFirst(out)
	return out
}

func sortNewestFirst(links []Link) {
	slices.SortFunc(links, func(a, b Link) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return compareIDs(b.ID, a.ID)
	})
}

// compareIDs orders "l2" before "l10".
func compareIDs(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

func copyLink(l *Link) Link {
	c := *l
	c.Tags = slices.Clone(l.Tags)
	return c
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", &ValidationError{"url", "missing"}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", &ValidationError{"url", "not a URL"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", &ValidationError{"url", "scheme must be http or https"}
	}
	if u.Host == "" {
		return "", &ValidationError{"url", "missing host"}
	}
	u.Fragment = ""
	return u.String(), nil
}

func normalizeTags(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || slices.Contains(out, t) {
			continue
		}
		if utf8.RuneCountInString(t) > MaxTagLen {
			return nil, &ValidationError{"tags", fmt.Sprintf("tag %q is longer than %d characters", t, MaxTagLen)}
		}
		out = append(out, t)
	}
	if len(out) > MaxTags {
		return nil, &ValidationError{"tags", fmt.Sprintf("more than %d tags", MaxTags)}
	}
	slices.Sort(out)
	return out, nil
}
