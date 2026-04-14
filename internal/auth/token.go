package auth

import (
	"errors"
	"os"
)

const EnvVar = "RAINDROP_TOKEN"

func Token() (string, error) {
	tok := os.Getenv(EnvVar)
	if tok == "" {
		return "", errors.New("RAINDROP_TOKEN not set — create a test token at https://app.raindrop.io/settings/integrations")
	}
	return tok, nil
}
