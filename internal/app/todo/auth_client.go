package apptodo

import (
	authusecase "auth_service/internal/usecase/auth"
	"context"
)

type LocalAuthGrpcClient struct {
	Verifier authusecase.AccessTokenVerifier
}

func (c LocalAuthGrpcClient) ValidateAccessToken(_ context.Context, token string) (string, error) {
	id, err := c.Verifier.VerifyAccessToken(token)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
