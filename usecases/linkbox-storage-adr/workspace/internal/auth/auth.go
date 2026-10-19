// Package auth maps API keys to users.
package auth

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"io"
	"strings"
)

// Keys maps API keys to user names.
type Keys struct {
	entries []entry
}

type entry struct {
	key  []byte
	user string
}

// MinKeyLen is the shortest key Parse accepts.
const MinKeyLen = 16

// Parse reads one "key:user" pair per line; blank lines and lines starting with # are skipped.
func Parse(r io.Reader) (*Keys, error) {
	k := &Keys{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, user, ok := strings.Cut(line, ":")
		key, user = strings.TrimSpace(key), strings.TrimSpace(user)
		if !ok || user == "" {
			return nil, fmt.Errorf("line %d: want key:user", n)
		}
		if len(key) < MinKeyLen {
			return nil, fmt.Errorf("line %d: key shorter than %d characters", n, MinKeyLen)
		}
		k.entries = append(k.entries, entry{key: []byte(key), user: user})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return k, nil
}

// User returns the user that key belongs to. It compares against every key in constant time, so the time
// taken does not reveal which keys exist.
func (k *Keys) User(key string) (string, bool) {
	user, found := "", 0
	for _, e := range k.entries {
		if subtle.ConstantTimeCompare(e.key, []byte(key)) == 1 {
			user, found = e.user, 1
		}
	}
	return user, found == 1
}
