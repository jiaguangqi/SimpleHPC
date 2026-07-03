package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	passwordResetTTL      = 10 * time.Minute
	passwordResetCooldown = 60 * time.Second
	passwordResetAttempts = 5
)

var (
	ErrInvalidCredentials = errors.New("账号或密码错误")
	ErrAccountFrozen      = errors.New("账号已冻结，请联系管理员")
	ErrResetCodeInvalid   = errors.New("验证码错误或已失效")
	ErrResetTooFrequent   = errors.New("验证码发送过于频繁，请稍后重试")
)

type AuthUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
	Type        string `json:"type"`
}

type AuthProfile struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	AccountType string `json:"accountType"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	Unit        string `json:"unit,omitempty"`
	Team        string `json:"team,omitempty"`
	LeaderName  string `json:"leaderName,omitempty"`
	UIDNumber   string `json:"uidNumber,omitempty"`
	GIDNumber   string `json:"gidNumber,omitempty"`
	HomeDir     string `json:"homeDir,omitempty"`
	LDAPDN      string `json:"ldapDN,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
	LastLogin   string `json:"lastLogin,omitempty"`
	SyncedAt    string `json:"syncedAt,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type PasswordResetChallenge struct {
	ID        string `json:"id"`
	Account   string `json:"account"`
	Type      string `json:"type"`
	Email     string `json:"email"`
	CodeHash  string `json:"codeHash"`
	Attempts  int    `json:"attempts"`
	CreatedAt string `json:"createdAt"`
}

type PasswordResetResult struct {
	Username string `json:"username"`
	Type     string `json:"type"`
	Email    string `json:"email"`
}

func (s *Services) Authenticate(ctx context.Context, accountType, username, password string) (AuthUser, string, error) {
	accountType = normalizeAccountType(accountType)
	username = strings.TrimSpace(username)
	if accountType == "" || username == "" || password == "" {
		return AuthUser{}, "", ErrInvalidCredentials
	}
	var user AuthUser
	var err error
	if accountType == "admin" {
		user, err = s.authenticateAdmin(ctx, username, password)
	} else {
		user, err = s.authenticateLDAP(ctx, username, password)
	}
	if err != nil {
		return AuthUser{}, "", err
	}
	token, err := randomHex(32)
	if err != nil {
		return AuthUser{}, "", err
	}
	if s.Redis != nil {
		raw, _ := json.Marshal(user)
		if err := s.Redis.Set(ctx, "auth:session:"+token, raw, 12*time.Hour).Err(); err != nil {
			return AuthUser{}, "", err
		}
	}
	return user, token, nil
}

func (s *Services) Logout(ctx context.Context, token string) error {
	if s.Redis == nil || strings.TrimSpace(token) == "" {
		return nil
	}
	return s.Redis.Del(ctx, "auth:session:"+strings.TrimSpace(token)).Err()
}

func (s *Services) SessionUser(ctx context.Context, token string) (AuthUser, error) {
	if s.Redis == nil {
		return AuthUser{}, errNotConfigured("redis")
	}
	raw, err := s.Redis.Get(ctx, "auth:session:"+strings.TrimSpace(token)).Bytes()
	if err != nil {
		return AuthUser{}, ErrInvalidCredentials
	}
	var user AuthUser
	if json.Unmarshal(raw, &user) != nil || user.Username == "" {
		return AuthUser{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Services) authenticateAdmin(ctx context.Context, username, password string) (AuthUser, error) {
	if s.DB == nil {
		return AuthUser{}, errNotConfigured("postgres")
	}
	var email, role, status, hash string
	err := s.DB.QueryRowContext(ctx, `
SELECT email, role_name, status, password_hash
FROM admin_users WHERE username = $1
`, username).Scan(&email, &role, &status, &hash)
	if err != nil || hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return AuthUser{}, ErrInvalidCredentials
	}
	if !strings.EqualFold(status, "active") {
		return AuthUser{}, ErrAccountFrozen
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE admin_users SET last_login = $2, updated_at = now() WHERE username = $1`,
		username, time.Now().Format("2006-01-02 15:04:05"))
	return AuthUser{Username: username, DisplayName: username, Email: email, Role: role, Type: "admin"}, nil
}

func (s *Services) authenticateLDAP(ctx context.Context, username, password string) (AuthUser, error) {
	if s.DB == nil {
		return AuthUser{}, errNotConfigured("postgres")
	}
	var displayName, email, status string
	err := s.DB.QueryRowContext(ctx, `
SELECT display_name, email, status FROM platform_users WHERE username = $1 AND status <> 'deleted'
`, username).Scan(&displayName, &email, &status)
	if err != nil {
		return AuthUser{}, ErrInvalidCredentials
	}
	if status == "frozen" || status == "disabled" || status == "locked" {
		return AuthUser{}, ErrAccountFrozen
	}
	ldapUser, err := s.LDAP.Authenticate(username, password)
	if err != nil {
		return AuthUser{}, ErrInvalidCredentials
	}
	if ldapUser.LoginShell == "/sbin/nologin" || ldapUser.LoginShell == "/bin/false" {
		return AuthUser{}, ErrAccountFrozen
	}
	if displayName == "" {
		displayName = ldapUser.DisplayName
	}
	if displayName == "" {
		displayName = ldapUser.CN
	}
	if email == "" {
		email = ldapUser.Mail
	}
	return AuthUser{Username: username, DisplayName: displayName, Email: email, Role: "user", Type: "ldap"}, nil
}

func (s *Services) RequestPasswordReset(
	ctx context.Context,
	accountType, username string,
	deliver func(email, code string) error,
) (string, error) {
	if s.Redis == nil {
		return "", errNotConfigured("redis")
	}
	accountType = normalizeAccountType(accountType)
	username = strings.TrimSpace(username)
	if accountType == "" || username == "" {
		return "", fmt.Errorf("账号类型和账号不能为空")
	}
	rateKey := "auth:reset:rate:" + accountFingerprint(accountType, username)
	ok, err := s.Redis.SetNX(ctx, rateKey, "1", passwordResetCooldown).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrResetTooFrequent
	}
	id, err := randomHex(24)
	if err != nil {
		return "", err
	}
	email, status, err := s.resetAccountInfo(ctx, accountType, username)
	if err != nil || email == "" || status != "active" {
		// Keep the response indistinguishable for unknown, deleted, or frozen accounts.
		return id, nil
	}
	code, err := randomDigits(6)
	if err != nil {
		return "", err
	}
	challenge := PasswordResetChallenge{
		ID: id, Account: username, Type: accountType, Email: email,
		CodeHash: resetCodeHash(id, code), CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(challenge)
	key := "auth:reset:challenge:" + id
	if err := s.Redis.Set(ctx, key, raw, passwordResetTTL).Err(); err != nil {
		return "", err
	}
	if deliver == nil {
		s.Redis.Del(ctx, key)
		return "", fmt.Errorf("邮件发送服务未配置")
	}
	if err := deliver(email, code); err != nil {
		s.Redis.Del(ctx, key)
		return "", err
	}
	return id, nil
}

func (s *Services) ConfirmPasswordReset(
	ctx context.Context,
	id, code string,
	deliver func(email, password string) error,
) (PasswordResetResult, error) {
	if s.Redis == nil {
		return PasswordResetResult{}, errNotConfigured("redis")
	}
	id = strings.TrimSpace(id)
	code = strings.TrimSpace(code)
	key := "auth:reset:challenge:" + id
	raw, err := s.Redis.Get(ctx, key).Bytes()
	if err != nil {
		return PasswordResetResult{}, ErrResetCodeInvalid
	}
	var challenge PasswordResetChallenge
	if json.Unmarshal(raw, &challenge) != nil || challenge.Attempts >= passwordResetAttempts {
		s.Redis.Del(ctx, key)
		return PasswordResetResult{}, ErrResetCodeInvalid
	}
	expected := []byte(challenge.CodeHash)
	actual := []byte(resetCodeHash(id, code))
	if len(expected) != len(actual) || subtle.ConstantTimeCompare(expected, actual) != 1 {
		challenge.Attempts++
		if challenge.Attempts >= passwordResetAttempts {
			s.Redis.Del(ctx, key)
		} else {
			ttl := s.Redis.TTL(ctx, key).Val()
			updated, _ := json.Marshal(challenge)
			s.Redis.Set(ctx, key, updated, ttl)
		}
		return PasswordResetResult{}, ErrResetCodeInvalid
	}
	s.Redis.Del(ctx, key)

	if challenge.Type == "admin" {
		result, err := s.ResetAdminPassword(ctx, challenge.Account, deliver)
		if err != nil {
			return PasswordResetResult{}, err
		}
		return PasswordResetResult{Username: result.Username, Type: "admin", Email: result.Email}, nil
	}
	result, err := s.resetLDAPPassword(ctx, challenge.Account, deliver)
	if err != nil {
		return PasswordResetResult{}, err
	}
	return PasswordResetResult{Username: result.Username, Type: "ldap", Email: result.Email}, nil
}

func (s *Services) resetLDAPPassword(
	ctx context.Context,
	username string,
	deliver func(email, password string) error,
) (AccountOperationResult, error) {
	email, status, err := s.resetAccountInfo(ctx, "ldap", username)
	if err != nil {
		return AccountOperationResult{}, err
	}
	if status != "active" {
		return AccountOperationResult{}, ErrAccountFrozen
	}
	password, err := randomPassword(18)
	if err != nil {
		return AccountOperationResult{}, err
	}
	if err := s.LDAP.SetUserPassword(username, password); err != nil {
		return AccountOperationResult{}, err
	}
	if _, err := s.DB.ExecContext(ctx, `
UPDATE platform_users
SET password_changed_at = now(), synced_at = now(), updated_at = now()
WHERE username = $1
`, username); err != nil {
		return AccountOperationResult{}, err
	}
	if deliver == nil {
		return AccountOperationResult{}, fmt.Errorf("邮件发送服务未配置")
	}
	if err := deliver(email, password); err != nil {
		return AccountOperationResult{}, err
	}
	return AccountOperationResult{
		Username: username, Status: "password_reset", Email: email,
		LDAPUpdated: true, PGUpdated: true,
	}, nil
}

func (s *Services) resetAccountInfo(ctx context.Context, accountType, username string) (string, string, error) {
	var email, status string
	var err error
	if accountType == "admin" {
		err = s.DB.QueryRowContext(ctx, `SELECT email, status FROM admin_users WHERE username = $1`, username).Scan(&email, &status)
	} else {
		err = s.DB.QueryRowContext(ctx, `SELECT email, status FROM platform_users WHERE username = $1 AND status <> 'deleted'`, username).Scan(&email, &status)
	}
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(email), strings.ToLower(strings.TrimSpace(status)), nil
}

func normalizeAccountType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "admin":
		return "admin"
	case "ldap":
		return "ldap"
	default:
		return ""
	}
}

func accountFingerprint(accountType, username string) string {
	sum := sha256.Sum256([]byte(accountType + ":" + strings.ToLower(username)))
	return hex.EncodeToString(sum[:])
}

func resetCodeHash(id, code string) string {
	sum := sha256.Sum256([]byte(id + ":" + code))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func randomDigits(length int) (string, error) {
	data := make([]byte, length)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, b := range data {
		out[i] = '0' + b%10
	}
	return string(out), nil
}
