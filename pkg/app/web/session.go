package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

const sessionCookieName = "trellis_session"

// sessionSigner issues and verifies tamper-evident session IDs carried in a
// cookie: a random ID plus an HMAC-SHA256 signature over that ID, so a
// forged or edited cookie is rejected rather than trusted. This is the MVP
// session identity mechanism — stdlib only, no external dependency.
type sessionSigner struct {
	key []byte
}

func newConfiguredSigner(configured []byte) (*sessionSigner, error) {
	if len(configured) == 0 {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		return &sessionSigner{key: key}, nil
	}
	if len(configured) < 32 {
		return nil, fmt.Errorf("trellis: signing key must be at least 32 bytes")
	}
	return &sessionSigner{key: append([]byte(nil), configured...)}, nil
}

func (s *sessionSigner) sign(id string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(id))
	return id + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *sessionSigner) verify(value string) (id string, ok bool) {
	id, sig, found := strings.Cut(value, ".")
	if !found {
		return "", false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(id))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}
	return id, true
}

func newSessionID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic("trellis: failed to generate session id: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func csrfForSession(signer *sessionSigner, id string) string {
	mac := hmac.New(sha256.New, signer.key)
	mac.Write([]byte("trellis-csrf:"))
	mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validCSRFToken(expected, supplied string) bool {
	if expected == "" || supplied == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(supplied)) == 1
}

// readOrIssueSessionID reads the signed session cookie from r, verifying it
// against signer. If the cookie is absent or fails verification, a fresh ID
// is minted and set on w.
func readOrIssueSessionID(w http.ResponseWriter, r *http.Request, signer *sessionSigner, secure bool, maxAge int) string {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if id, ok := signer.verify(cookie.Value); ok {
			return id
		}
	}
	id := newSessionID()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signer.sign(id),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
	return id
}
