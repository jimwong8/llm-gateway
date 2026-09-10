package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

const (
	sessionIDHeader = "X-Session-ID"
	sessionIDCookie = "opencode_session_id"
)

type sessionIDSource string

const (
	sessionIDSourceExplicit  sessionIDSource = "explicit"
	sessionIDSourceHeader    sessionIDSource = "header"
	sessionIDSourceCookie    sessionIDSource = "cookie"
	sessionIDSourceGenerated sessionIDSource = "generated"
)

func resolveOrCreateSessionID(bodySessionID string, r *http.Request) (string, sessionIDSource) {
	if id := strings.TrimSpace(bodySessionID); id != "" {
		return id, sessionIDSourceExplicit
	}

	if r != nil {
		if id := strings.TrimSpace(r.Header.Get(sessionIDHeader)); id != "" {
			return id, sessionIDSourceHeader
		}
		if id := strings.TrimSpace(r.Header.Get("X-Session-Id")); id != "" {
			return id, sessionIDSourceHeader
		}
		if cookie, err := r.Cookie(sessionIDCookie); err == nil {
			if id := strings.TrimSpace(cookie.Value); id != "" {
				return id, sessionIDSourceCookie
			}
		}
	}

	return "oc_" + newSessionIDSuffix(), sessionIDSourceGenerated
}

func newSessionIDSuffix() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexStr := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}
