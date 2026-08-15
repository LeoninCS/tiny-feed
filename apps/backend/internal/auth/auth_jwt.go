// internal/auth/jwt.go
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwtSecret 用 sync.Once 保护懒加载，避免多个 goroutine 第一次访问时
// 都各自生成一次随机密钥互相覆盖的问题。
var (
	once   sync.Once
	secret []byte
)

// jwtSecret 返回进程内统一的 JWT 签名密钥。
// 优先用环境变量 JWT_SECRET；如果没设，则懒生成一个随机密钥并 log 警告。
// 随机密钥情况下，服务重启会让所有已签发的 token 失效——这是有意为之。
func jwtSecret() []byte {
	once.Do(func() {
		s := os.Getenv("JWT_SECRET")
		if s == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				log.Printf("严重错误：无法生成 JWT 密钥：%v", err)
				secret = []byte("fallback-unsafe-key-change-me")
				return
			}
			s = hex.EncodeToString(b)
			log.Printf("警告：未设置 JWT_SECRET，已生成随机密钥，服务重启后所有令牌将失效。")
		}
		secret = []byte(s)
	})
	return secret
}

type Claims struct {
	AccountID uint   `json:"account_id"`
	Username  string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(accountID uint, username string) (string, error) {
	now := time.Now()

	claims := Claims{
		AccountID: accountID,
		Username:  username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtSecret())
}

func GenerateRefreshToken(accountID uint) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return jwtSecret(), nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
