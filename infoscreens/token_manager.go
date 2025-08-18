package infoscreens

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"maps"
	"sync"
	"time"

	. "github.com/ezBadminton/ezBadmintonServer/generated"
	"github.com/ezBadminton/ezBadmintonServer/store"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

var (
	Tokens          *TokenManager
	ErrInvalidToken error = errors.New("invalid token")
)

// The infoscreen API accepts requests when a token
// is present in the Authorization header.
// Every screen has an initialization token that
// refreshes every 90 seconds to prevent users from
// saving a token for later.
// The token is displayed on the screen as a QR code.
// As soon as the first request is made,
// the initialization token becomes the controller token
// which stays valid for 5 minutes and can be used to
// control the info screen. If during the 5 minutes
// another request with the current init token is made,
// the current controller token also becomes invalid
// because it is overridden.
type TokenManager struct {
	app                    core.App
	refreshInterval        time.Duration
	tokenLifetime          time.Duration
	activeControllerTokens map[string]chan any
	// When the init tokens get refreshed the old ones get a short
	// grace period where both the old and the new one can be used.
	// This is for avoiding situations where someone scans a
	// QR code with a controller token right before it gets
	// refreshed, leaving the scanner with an invalid token.
	gracePeriodTokens map[string]string
	gracePeriod       time.Duration
	mu                sync.Mutex
}

func InitTokenManager(app core.App) {
	Tokens = &TokenManager{
		app:                    app,
		refreshInterval:        90 * time.Second,
		tokenLifetime:          5 * time.Minute,
		activeControllerTokens: make(map[string]chan any),
		gracePeriodTokens:      make(map[string]string),
		gracePeriod:            10 * time.Second,
	}

	infoscreenUserCName := CName[InfoscreenUser]()
	app.OnRecordCreate(infoscreenUserCName).BindFunc(Tokens.handleInfoscreenUserCreation)
	app.OnRecordAfterDeleteSuccess(infoscreenUserCName).BindFunc(Tokens.handleInfoscreenUserDeletion)

	infoscreenUserStore, _ := store.FindRecordStore[InfoscreenUser]()
	infoscreenUsers := infoscreenUserStore.RecordList

	for _, infoscreenUser := range infoscreenUsers {
		infoscreenUser = Clone(infoscreenUser)
		infoscreenUser.SetInitToken(newToken())
		infoscreenUser.SetControllerToken("")
		app.Save(infoscreenUser)
	}

	go Tokens.startInitTokenRefreshTimer()
}

func (m *TokenManager) startInitTokenRefreshTimer() {
	timer := time.NewTimer(m.refreshInterval)
	for {
		<-timer.C
		m.refreshInitTokens()
		timer.Reset(m.refreshInterval)
	}
}

func (m *TokenManager) startControllerTokenTimer(token string, cancel chan any) {
	timer := time.NewTimer(m.tokenLifetime)
	select {
	case <-timer.C:
		m.invalidateControllerToken(token)
	case <-cancel:
		timer.Stop()
		m.mu.Lock()
		delete(m.activeControllerTokens, token)
		m.mu.Unlock()
	}
}

func (m *TokenManager) ValidateToken(token string) (*InfoscreenUser, error) {
	lookupToken := token

	m.mu.Lock()
	graceToken, isGraceToken := m.gracePeriodTokens[lookupToken]
	if isGraceToken {
		lookupToken = graceToken
	}
	m.mu.Unlock()

	infoscreenUserCName := CName[InfoscreenUser]()
	infoscreenUserRecord := &core.Record{}
	err := m.app.RecordQuery(infoscreenUserCName).
		AndWhere(dbx.HashExp{"initToken": lookupToken}).
		OrWhere(dbx.HashExp{"controllerToken": token}).
		Limit(1).
		One(infoscreenUserRecord)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidToken
	} else if err != nil || infoscreenUserRecord.Id == "" {
		return nil, errors.New("could not fetch infoscreen user")
	}

	infoscreenUser, _ := WrapRecord[InfoscreenUser](infoscreenUserRecord)

	if infoscreenUser.InitToken() == token || isGraceToken {
		if err := m.useInitToken(infoscreenUser, token); err != nil {
			return nil, err
		}
	}

	return infoscreenUser, nil
}

func (m *TokenManager) invalidateControllerToken(token string) {
	m.mu.Lock()
	delete(m.activeControllerTokens, token)
	m.mu.Unlock()

	infoscreenUserCName := CName[InfoscreenUser]()
	infoscreenUserRecord := &core.Record{}
	err := m.app.RecordQuery(infoscreenUserCName).
		AndWhere(dbx.HashExp{"controllerToken": token}).
		Limit(1).
		One(infoscreenUserRecord)

	if err != nil || infoscreenUserRecord.Id == "" {
		return
	}

	infoscreenUser, _ := WrapRecord[InfoscreenUser](infoscreenUserRecord)
	infoscreenUser.SetControllerToken("")
	m.app.Save(infoscreenUser)
}

func (m *TokenManager) refreshInitTokens() {
	infoscreenUserStore, _ := store.FindRecordStore[InfoscreenUser]()
	infoscreenUsers := infoscreenUserStore.RecordList

	oldToNewTokens := make(map[string]string)

	for _, infoscreenUser := range infoscreenUsers {
		old := infoscreenUser.InitToken()
		new := newToken()
		infoscreenUser = Clone(infoscreenUser)
		infoscreenUser.SetInitToken(new)
		oldToNewTokens[old] = new
		m.app.Save(infoscreenUser)
	}
	m.startTokenGracePeriod(oldToNewTokens)
}

// Moves the init token to being the controller token and
// creates a new init token
func (m *TokenManager) useInitToken(infoscreenUser *InfoscreenUser, initToken string) error {
	m.mu.Lock()
	_, ok := m.activeControllerTokens[initToken]
	if ok {
		m.mu.Unlock()
		return errors.New("this init token was already used")
	}
	cancel := make(chan any)
	m.activeControllerTokens[initToken] = cancel
	m.mu.Unlock()

	controllerToken := infoscreenUser.ControllerToken()
	if controllerToken != "" {
		close(m.activeControllerTokens[controllerToken])
	}

	infoscreenUser.SetControllerToken(initToken)
	infoscreenUser.SetInitToken(newToken())
	m.app.Save(infoscreenUser)

	go m.startControllerTokenTimer(initToken, cancel)
	return nil
}

func (m *TokenManager) handleInfoscreenUserCreation(e *core.RecordEvent) error {
	infoscreenUser, _ := WrapRecord[InfoscreenUser](e.Record)
	infoscreenUser.SetInitToken(newToken())
	return e.Next()
}

func (m *TokenManager) handleInfoscreenUserDeletion(e *core.RecordEvent) error {
	infoscreenUser, _ := WrapRecord[InfoscreenUser](e.Record)
	controllerToken := infoscreenUser.ControllerToken()
	if controllerToken != "" {
		m.mu.Lock()
		delete(m.activeControllerTokens, controllerToken)
		m.mu.Unlock()
	}
	return e.Next()
}

// The tokens map maps old token (that goes into grace period) -> new token
func (m *TokenManager) startTokenGracePeriod(tokens map[string]string) {
	m.mu.Lock()
	maps.Copy(m.gracePeriodTokens, tokens)
	m.mu.Unlock()
	go func() {
		timer := time.NewTimer(m.gracePeriod)
		<-timer.C
		m.mu.Lock()
		for token := range tokens {
			delete(m.gracePeriodTokens, token)
		}
		m.mu.Unlock()
	}()
}

func newToken() string {
	b := make([]byte, 10)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
