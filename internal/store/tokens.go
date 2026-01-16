package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"fmt"
	"time"

	"consult_app.cedrickewi/internal/validator"
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

type TokenStore struct {
	db *sql.DB
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]
	return token, nil

}

// Check that the plaintext token has been provided and is exactly 52 bytes long.
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// The New() method is a shortcut which creates a new Token struct and then inserts the
// data in the tokens table.
func (s *TokenStore) New(ctx context.Context, userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = s.Insert(ctx, token)
	return token, err

}

func (s *TokenStore) Insert(ctx context.Context, token *Token) error {
	query := `
	INSERT INTO tokens (hash, user_id, expiry, scope)
	VALUES ($1, $2, $3, $4)`

	args := []interface{}{token.Hash, token.UserID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, args...)

	if err != nil {
		return err
	}

	return nil
}

// DeleteAllForUser() deletes all tokens for a specific user and scope.
func (s *TokenStore) DeleteAllForUser(ctx context.Context, scope string, userID int64) error {
	query := `
	DELETE FROM tokens
	WHERE scope = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, scope, userID)
	if err != nil {
		return fmt.Errorf("failed to delete tokens for user %d with scope %s: %w", userID, scope, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Log or return a custom error if necessary
		return fmt.Errorf("no tokens found for user %d with scope %s", userID, scope)
	}

	return nil
}
