package token

import "errors"

var (
	ErrTokenNotFound = errors.New("refresh token not found")
	ErrTokenRevoked  = errors.New("refresh token revoked")
	ErrTokenExpired  = errors.New("refresh token expired")
	ErrTokenInvalid  = errors.New("token invalid")
)
