package session

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

func newID(prefix string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(buf[:])
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugPattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "session"
	}
	return s
}

func sessionHandleName(id string, title string) string {
	if len(id) > 8 {
		id = id[:8]
	}
	return id + "-" + slugify(title)
}
