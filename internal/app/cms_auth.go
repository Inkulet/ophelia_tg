package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	cmsJWTIssuer   = "ophelia-cms"
	cmsJWTLifetime = 24 * time.Hour
)

type cmsJWTClaims struct {
	UserID  int64  `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
	Issuer  string `json:"iss"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
}

type cmsAuthContextKey struct{}

func generateCMSJWT(userID int64, isAdmin bool) (string, error) {
	secret := cmsJWTSecret()
	if secret == "" {
		return "", errors.New("cms jwt secret is not configured")
	}
	if userID <= 0 {
		return "", errors.New("invalid user id")
	}

	now := time.Now().UTC()
	claims := cmsJWTClaims{
		UserID:  userID,
		IsAdmin: isAdmin,
		Issuer:  cmsJWTIssuer,
		Issued:  now.Unix(),
		Expires: now.Add(cmsJWTLifetime).Unix(),
	}

	headerPart, err := encodeJWTPart(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	payloadPart, err := encodeJWTPart(claims)
	if err != nil {
		return "", err
	}

	unsigned := headerPart + "." + payloadPart
	signature := signJWT(unsigned, secret)
	return unsigned + "." + signature, nil
}

func parseAndValidateCMSJWT(token string) (*cmsJWTClaims, error) {
	secret := cmsJWTSecret()
	if secret == "" {
		return nil, errors.New("cms jwt secret is not configured")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	unsigned := parts[0] + "." + parts[1]
	expected := signJWT(unsigned, secret)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("invalid token signature")
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(header.Alg), "HS256") {
		return nil, errors.New("unsupported token algorithm")
	}

	var claims cmsJWTClaims
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}
	if claims.UserID <= 0 {
		return nil, errors.New("token user is invalid")
	}
	if claims.Expires <= 0 || time.Now().UTC().Unix() >= claims.Expires {
		return nil, errors.New("token is expired")
	}
	if claims.Issuer != "" && claims.Issuer != cmsJWTIssuer {
		return nil, errors.New("invalid token issuer")
	}

	return &claims, nil
}

func requireCMSJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := authorizeCMSRequest(r)
		if err != nil {
			writeCMSError(w, http.StatusUnauthorized, err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), cmsAuthContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireCMSAdminJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := authorizeCMSRequest(r)
		if err != nil {
			writeCMSError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if !claims.IsAdmin && !isAdmin(claims.UserID) {
			writeCMSError(w, http.StatusForbidden, "admin role is required")
			return
		}
		ctx := context.WithValue(r.Context(), cmsAuthContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authorizeCMSRequest(r *http.Request) (*cmsJWTClaims, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}
	claims, err := parseAndValidateCMSJWT(token)
	if err != nil {
		return nil, errors.New("valid bearer token is required")
	}
	return claims, nil
}

func cmsUserIDFromContext(ctx context.Context) (int64, bool) {
	claims, ok := cmsClaimsFromContext(ctx)
	if !ok || claims.UserID <= 0 {
		return 0, false
	}
	return claims.UserID, true
}

func cmsClaimsFromContext(ctx context.Context) (*cmsJWTClaims, bool) {
	if ctx == nil {
		return nil, false
	}
	claims, ok := ctx.Value(cmsAuthContextKey{}).(*cmsJWTClaims)
	if !ok || claims == nil {
		return nil, false
	}
	return claims, true
}

func extractBearerToken(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("valid bearer token is required")
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("valid bearer token is required")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("valid bearer token is required")
	}
	return token, nil
}

func buildCMSSiteURLWithToken(baseURL, token string) (string, error) {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		raw = "http://site.com/"
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(u.Host) == "" {
		return "", errors.New("invalid cms site url")
	}

	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func encodeJWTPart(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeJWTPart(src string, dst any) error {
	raw, err := base64.RawURLEncoding.DecodeString(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func signJWT(unsigned, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func cmsJWTSecret() string {
	if secret := strings.TrimSpace(config.CMSJWTSecret); secret != "" {
		return secret
	}
	return strings.TrimSpace(config.Token)
}
