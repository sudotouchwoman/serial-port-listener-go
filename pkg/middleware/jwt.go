package middleware

import (
	"errors"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

var (
	ErrBadToken   error = errors.New("bad token")
	ErrBadMethod  error = errors.New("bad sign method")
	ErrBadPayload error = errors.New("empty/expired payload")
)

type AuthJWT struct {
	secretKey string
	aesMethod string
}

// Validates given raw JWT token
// as far as I can see, there is no apparent usage
// in additional token validation as it
// apparently lies in the signature itself
func (a *AuthJWT) Verify(tokenRaw string) (string, error) {
	token, err := jwt.Parse(tokenRaw, a.secretGetter)
	if err != nil || !token.Valid {
		return "", ErrBadToken
	}
	payload, ok := token.Claims.(jwt.MapClaims)
	if !ok || payload.Valid() != nil {
		return "", ErrBadPayload
	}
	var id interface{}
	if id, ok = payload["username"]; !ok {
		return "", ErrBadPayload
	}
	username := ""
	if username, ok = id.(string); ok {
		return username, nil
	}
	return "", ErrBadPayload
}

// Creates new JWT token string,
// which stores username (uuid) and is signed
// with a secret key
func (a *AuthJWT) NewUser() (string, error) {
	// sensitive data inside JWT can be hashed too
	jwtToken := jwt.NewWithClaims(&jwt.SigningMethodHMAC{}, jwt.MapClaims{
		"username": uuid.NewString(),
	})
	return jwtToken.SignedString(a.secretKey)
}

func (a *AuthJWT) secretGetter(token *jwt.Token) (interface{}, error) {
	method, ok := token.Method.(*jwt.SigningMethodHMAC)
	if !ok || method.Alg() != a.aesMethod {
		return nil, ErrBadMethod
	}
	return a.secretKey, nil
}
