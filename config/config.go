package config

// JWTSecret is the symmetric key used to sign and verify tokens.
// In production this must come from an environment variable.
var JWTSecret = []byte("super-secret-key")
