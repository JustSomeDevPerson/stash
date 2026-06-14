// Package session provides session authentication and management for the application.
package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/stashapp/stash/pkg/logger"
)

type key int

const (
	contextUser key = iota
	contextVisitedPlugins
	contextLocalRequest
)

const (
	userIDKey             = "userID"
	visitedPluginHooksKey = "visitedPluginsHooks"
	restrictedKey         = "restricted"
	unrestrictedAtKey     = "unrestrictedAt"
)

const (
	ApiKeyHeader    = "ApiKey"
	ApiKeyParameter = "apikey"
)

const (
	cookieName      = "session"
	usernameFormKey = "username"
	passwordFormKey = "password"
)

type InvalidCredentialsError struct {
	Username string
}

func (e InvalidCredentialsError) Error() string {
	// don't leak the username
	return "invalid credentials"
}

var ErrUnauthorized = errors.New("unauthorized")

type InvalidRestrictedPasswordError struct{}

func (e InvalidRestrictedPasswordError) Error() string {
	return "invalid restricted password"
}

type Store struct {
	sessionStore *sessions.CookieStore
	config       SessionConfig
}

func NewStore(c SessionConfig) *Store {
	ret := &Store{
		sessionStore: sessions.NewCookieStore(c.GetSessionStoreKey()),
		config:       c,
	}

	ret.sessionStore.MaxAge(c.GetMaxSessionAge())
	ret.sessionStore.Options.SameSite = http.SameSiteLaxMode

	return ret
}

func (s *Store) Login(w http.ResponseWriter, r *http.Request) error {
	// ignore error - we want a new session regardless
	newSession, _ := s.sessionStore.Get(r, cookieName)

	username := r.FormValue(usernameFormKey)
	password := r.FormValue(passwordFormKey)

	// authenticate the user
	if !s.config.ValidateCredentials(username, password) {
		return &InvalidCredentialsError{Username: username}
	}

	// since we only have one user, don't leak the name
	logger.Info("User logged in")

	newSession.Values[userIDKey] = username
	newSession.Values[restrictedKey] = true
	delete(newSession.Values, unrestrictedAtKey)

	err := newSession.Save(r, w)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) Logout(w http.ResponseWriter, r *http.Request) error {
	session, err := s.sessionStore.Get(r, cookieName)
	if err != nil {
		return err
	}

	delete(session.Values, userIDKey)
	session.Options.MaxAge = -1

	err = session.Save(r, w)
	if err != nil {
		return err
	}

	// since we only have one user, don't leak the name
	logger.Infof("User logged out")

	return nil
}

func (s *Store) GetSessionUserID(w http.ResponseWriter, r *http.Request) (string, error) {
	session, err := s.sessionStore.Get(r, cookieName)
	// ignore errors and treat as an empty user id, so that we handle expired
	// cookie
	if err != nil {
		return "", nil
	}

	if !session.IsNew {
		val := session.Values[userIDKey]
		s.applyRestrictedTimeout(session)

		// refresh the cookie
		err = session.Save(r, w)
		if err != nil {
			return "", err
		}

		ret, _ := val.(string)

		return ret, nil
	}

	return "", nil
}

func (s *Store) applyRestrictedTimeout(session *sessions.Session) {
	restricted, ok := session.Values[restrictedKey].(bool)
	if !ok {
		session.Values[restrictedKey] = true
		delete(session.Values, unrestrictedAtKey)
		return
	}

	if restricted {
		delete(session.Values, unrestrictedAtKey)
		return
	}

	v, ok := session.Values[unrestrictedAtKey].(int64)
	if !ok {
		session.Values[restrictedKey] = true
		delete(session.Values, unrestrictedAtKey)
		return
	}

	now := time.Now().Unix()
	timeout := int64(s.config.GetRestrictedSessionTimeout())
	if timeout > 0 && now-v >= timeout {
		session.Values[restrictedKey] = true
		delete(session.Values, unrestrictedAtKey)
		return
	}

	session.Values[unrestrictedAtKey] = now
}

func (s *Store) IsRestricted(w http.ResponseWriter, r *http.Request) (bool, error) {
	session, err := s.sessionStore.Get(r, cookieName)
	if err != nil {
		return true, nil
	}

	if session.IsNew {
		session.Values[restrictedKey] = true
		delete(session.Values, unrestrictedAtKey)
		if err := session.Save(r, w); err != nil {
			return true, err
		}
		return true, nil
	}

	s.applyRestrictedTimeout(session)

	restricted, ok := session.Values[restrictedKey].(bool)
	if !ok {
		restricted = true
		session.Values[restrictedKey] = true
		delete(session.Values, unrestrictedAtKey)
	}

	if err := session.Save(r, w); err != nil {
		return true, err
	}

	return restricted, nil
}

func (s *Store) Restrict(w http.ResponseWriter, r *http.Request) error {
	session, err := s.sessionStore.Get(r, cookieName)
	if err != nil {
		return err
	}

	if session.IsNew {
		return ErrUnauthorized
	}

	session.Values[restrictedKey] = true
	delete(session.Values, unrestrictedAtKey)

	return session.Save(r, w)
}

func (s *Store) Unrestrict(w http.ResponseWriter, r *http.Request, password string) error {
	if !s.config.ValidateRestrictedPassword(password) {
		return InvalidRestrictedPasswordError{}
	}

	session, err := s.sessionStore.Get(r, cookieName)
	if err != nil {
		return err
	}

	if session.IsNew {
		return ErrUnauthorized
	}

	session.Values[restrictedKey] = false
	session.Values[unrestrictedAtKey] = time.Now().Unix()

	return session.Save(r, w)
}

func SetCurrentUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextUser, userID)
}

// GetCurrentUserID gets the current user id from the provided context
func GetCurrentUserID(ctx context.Context) *string {
	userCtxVal := ctx.Value(contextUser)
	if userCtxVal != nil {
		currentUser := userCtxVal.(string)
		return &currentUser
	}

	return nil
}

func (s *Store) Authenticate(w http.ResponseWriter, r *http.Request) (userID string, err error) {
	c := s.config

	// translate api key into current user, if present
	apiKey := r.Header.Get(ApiKeyHeader)

	// try getting the api key as a query parameter
	if apiKey == "" {
		apiKey = r.URL.Query().Get(ApiKeyParameter)
	}

	if apiKey != "" {
		// match against configured API and set userID to the
		// configured username. In future, we'll want to
		// get the username from the key.
		if c.GetAPIKey() != apiKey {
			return "", ErrUnauthorized
		}

		userID = c.GetUsername()
	} else {
		// handle session
		userID, err = s.GetSessionUserID(w, r)
	}

	if err != nil {
		return "", err
	}

	return
}
