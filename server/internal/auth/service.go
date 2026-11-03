package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"pier/internal/store"
)

type session struct{ Expires time.Time }
type attempt struct {
	Count int
	Until time.Time
}
type Service struct {
	Store    *store.Store
	Secure   bool
	mu       sync.Mutex
	sessions map[[32]byte]session
	attempts map[string]attempt
}

func New(s *store.Store, secure bool) *Service {
	return &Service{Store: s, Secure: secure, sessions: map[[32]byte]session{}, attempts: map[string]attempt{}}
}
func (s *Service) Initialized() bool { return s.Store.Snapshot().Admin != nil }
func (s *Service) Setup(username, password string) error {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 40 || len(password) < 12 || len(password) > 72 {
		return errors.New("用户名需 3–40 个字符，密码需 12–72 字节")
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), 12)
	if e != nil {
		return e
	}
	return s.Store.Update(func(d *store.Data) error {
		if d.Admin != nil {
			return errors.New("管理员已经创建")
		}
		d.Admin = &store.Admin{Username: username, PasswordHash: string(hash)}
		return nil
	})
}
func (s *Service) Allow(remote string) bool {
	host, _, e := net.SplitHostPort(remote)
	if e != nil {
		host = remote
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, a := range s.attempts {
		if now.After(a.Until) {
			delete(s.attempts, k)
		}
	}
	a := s.attempts[host]
	if a.Until.IsZero() {
		a.Until = now.Add(time.Minute)
	}
	a.Count++
	s.attempts[host] = a
	return a.Count <= 10
}
func (s *Service) Login(w http.ResponseWriter, username, password string) error {
	a := s.Store.Snapshot().Admin
	// Always perform bcrypt work, including for an unknown account.
	hash := "$2a$12$8qu1MPYjQvWYm5Ejh4gLoePQXKwXpSHTMG0aRVJGdZR0GTrOzZFAe"
	if a != nil {
		hash = a.PasswordHash
	}
	valid := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	if a == nil || username != a.Username || !valid {
		return errors.New("用户名或密码错误")
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return e
	}
	token := hex.EncodeToString(b)
	key := sha256.Sum256([]byte(token))
	expiry := time.Now().Add(12 * time.Hour)
	s.mu.Lock()
	for k, v := range s.sessions {
		if time.Now().After(v.Expires) {
			delete(s.sessions, k)
		}
	}
	s.sessions[key] = session{Expires: expiry}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "pier_session", Value: token, Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, Expires: expiry})
	return nil
}
func (s *Service) Valid(r *http.Request) bool {
	c, e := r.Cookie("pier_session")
	if e != nil {
		return false
	}
	key := sha256.Sum256([]byte(c.Value))
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[key]
	if !ok {
		return false
	}
	if time.Now().After(v.Expires) {
		delete(s.sessions, key)
		return false
	}
	return true
}
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("pier_session"); e == nil {
		s.mu.Lock()
		delete(s.sessions, sha256.Sum256([]byte(c.Value)))
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "pier_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode})
}
func (s *Service) ChangePassword(old, next string) error {
	if len(next) < 12 || len(next) > 72 {
		return errors.New("新密码需 12–72 字节")
	}
	a := s.Store.Snapshot().Admin
	if a == nil || bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(old)) != nil {
		return errors.New("原密码错误")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(next), 12)
	if e != nil {
		return e
	}
	e = s.Store.Update(func(d *store.Data) error { d.Admin.PasswordHash = string(h); return nil })
	if e == nil {
		s.mu.Lock()
		s.sessions = map[[32]byte]session{}
		s.mu.Unlock()
	}
	return e
}
