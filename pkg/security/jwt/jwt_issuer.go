package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Issuer struct {
	secret []byte
	issuer string
}

func New(secret, issuer string) *Issuer {
	return &Issuer{
		secret: []byte(secret),
		issuer: issuer,
	}
}

type AccessClaims struct {
	jwt.RegisteredClaims
}

func (i *Issuer) IssueAccessToken(userID uuid.UUID, ttl time.Duration) (string, error) {
	now := time.Now()

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(i.secret)
}

func (i *Issuer) VerifyAccessToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return i.secret, nil
	}, jwt.WithIssuer(i.issuer))
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return uuid.Nil, jwt.ErrTokenInvalidClaims
	}

	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, jwt.ErrTokenInvalidClaims
	}

	return uid, nil
}
