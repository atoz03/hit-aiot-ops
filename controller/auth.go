package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type sessionPayload struct {
	Username string `json:"u"`
	Role     string `json:"r"`
	ExpUnix  int64  `json:"exp"`
	Nonce    string `json:"n"`
}

func newNonce() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func signSession(secret string, payload sessionPayload) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("auth_secret 不能为空")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	bodyB64 := base64.RawURLEncoding.EncodeToString(body)
	sig := hmacSHA256B64(secret, bodyB64)
	return bodyB64 + "." + sig, nil
}

func verifySession(secret string, token string, now time.Time) (sessionPayload, error) {
	if strings.TrimSpace(secret) == "" {
		return sessionPayload{}, errors.New("auth_secret 不能为空")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return sessionPayload{}, errors.New("token 格式不合法")
	}
	bodyB64 := parts[0]
	sigB64 := parts[1]
	expect := hmacSHA256B64(secret, bodyB64)
	if !hmac.Equal([]byte(expect), []byte(sigB64)) {
		return sessionPayload{}, errors.New("token 签名不匹配")
	}

	body, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return sessionPayload{}, errors.New("token 解码失败")
	}
	var p sessionPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return sessionPayload{}, errors.New("token 解析失败")
	}
	if p.Username == "" || p.Role == "" || p.ExpUnix <= 0 {
		return sessionPayload{}, errors.New("token 字段缺失")
	}
	if now.Unix() > p.ExpUnix {
		return sessionPayload{}, errors.New("token 已过期")
	}
	return p, nil
}

func hmacSHA256B64(secret string, data string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (p sessionPayload) String() string {
	return fmt.Sprintf("user=%s role=%s exp=%d", p.Username, p.Role, p.ExpUnix)
}
