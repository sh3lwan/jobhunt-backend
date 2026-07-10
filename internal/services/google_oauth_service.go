package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

const (
	googleAuthEndpoint     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint    = "https://oauth2.googleapis.com/token"
	googleUserInfoEndpoint = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GoogleOAuthService implements the Google OAuth 2.0 authorization-code flow.
// It is optional: if GOOGLE_CLIENT_ID/SECRET are unset, Configured() reports
// false and the handlers return a clean "not configured" response instead of a
// broken redirect.
type GoogleOAuthService struct {
	clientID     string
	clientSecret string
	redirectURL  string
	frontendURL  string
	auth         *AuthService
	queries      *repository.Queries
	client       *http.Client
}

func NewGoogleOAuthService(clientID, clientSecret, redirectURL, frontendURL string, auth *AuthService, queries *repository.Queries) *GoogleOAuthService {
	return &GoogleOAuthService{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		frontendURL:  frontendURL,
		auth:         auth,
		queries:      queries,
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured reports whether Google credentials are present.
func (s *GoogleOAuthService) Configured() bool {
	return s.clientID != "" && s.clientSecret != ""
}

// FrontendURL is where the browser is sent after the flow (success or error).
func (s *GoogleOAuthService) FrontendURL() string {
	return s.frontendURL
}

// NewState returns a random CSRF state token.
func (s *GoogleOAuthService) NewState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// AuthCodeURL builds the Google consent-screen URL.
func (s *GoogleOAuthService) AuthCodeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", s.clientID)
	q.Set("redirect_uri", s.redirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	return googleAuthEndpoint + "?" + q.Encode()
}

type googleUserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// HandleCallback exchanges the code, resolves (or creates) the user, and
// returns a signed JWT for them.
func (s *GoogleOAuthService) HandleCallback(ctx context.Context, code string) (string, error) {
	accessToken, err := s.exchangeCode(ctx, code)
	if err != nil {
		return "", err
	}

	info, err := s.fetchUserInfo(ctx, accessToken)
	if err != nil {
		return "", err
	}

	if info.Email == "" {
		return "", fmt.Errorf("google did not return an email")
	}

	user, err := s.upsertUser(ctx, info)
	if err != nil {
		return "", err
	}

	return s.auth.Generate(user)
}

func (s *GoogleOAuthService) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("redirect_uri", s.redirectURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange returned status %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("no access token in google response")
	}
	return body.AccessToken, nil
}

func (s *GoogleOAuthService) fetchUserInfo(ctx context.Context, accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned status %d", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// upsertUser finds the user by email or creates one. Google accounts have no
// local password, so a random bcrypt hash is stored (blocks password login).
func (s *GoogleOAuthService) upsertUser(ctx context.Context, info *googleUserInfo) (*models.User, error) {
	existing, err := s.queries.GetUserByEmail(ctx, info.Email)
	if err == nil {
		return &models.User{
			ID:       existing.ID,
			Username: existing.Username,
			Email:    existing.Email,
		}, nil
	}

	randomPw := make([]byte, 24)
	if _, rerr := rand.Read(randomPw); rerr != nil {
		return nil, rerr
	}
	hash, herr := bcrypt.GenerateFromPassword(randomPw, bcrypt.DefaultCost)
	if herr != nil {
		return nil, herr
	}

	// Use the email as username (both are UNIQUE); Google guarantees uniqueness.
	if cerr := s.queries.CreateUser(ctx, repository.CreateUserParams{
		Email:    info.Email,
		Username: info.Email,
		Password: string(hash),
	}); cerr != nil {
		return nil, fmt.Errorf("failed to create google user: %w", cerr)
	}

	created, gerr := s.queries.GetUserByEmail(ctx, info.Email)
	if gerr != nil {
		return nil, gerr
	}
	return &models.User{
		ID:       created.ID,
		Username: created.Username,
		Email:    created.Email,
	}, nil
}
