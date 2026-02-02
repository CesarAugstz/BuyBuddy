package utils

import (
	"context"
	"errors"

	"google.golang.org/api/idtoken"
)

type GoogleTokenInfo struct {
	Email   string
	Name    string
	Picture string
	Subject string
}

func VerifyGoogleIDToken(ctx context.Context, idToken, clientID string) (*GoogleTokenInfo, error) {
	if clientID == "" {
		return nil, errors.New("google client ID not configured")
	}

	payload, err := idtoken.Validate(ctx, idToken, clientID)
	if err != nil {
		return nil, err
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	subject := payload.Subject

	return &GoogleTokenInfo{
		Email:   email,
		Name:    name,
		Picture: picture,
		Subject: subject,
	}, nil
}
