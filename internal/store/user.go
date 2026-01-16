package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"consult_app.cedrickewi/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

type UserStore struct {
	db *sql.DB
}

type User struct {
	ID           int64    `json:"id"`
	Name         string   `json:"username"`
	Email        string   `json:"email"`
	Password     password `json:"-"`
	Phone        string   `json:"phone"`
	AuthProvider string   `json:"auth_provider"`
	GoogleID     int64    `json:"google_id"`
	IsActivated  bool     `json:"is_activated"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	Version      int64    `json:"version"`
	Role         string   `json:"role"`
	ImageURL     string   `json:"image_url"`
	IsExpert     bool     `json:"is_expert"`
}

var AnonymousUser = &User{}

type password struct {
	plaintext *string
	hash      []byte
}

// Check if a User instance is the AnonymousUser.
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func (s *UserStore) Create(ctx context.Context, user *User) error {
	query := `
	INSERT INTO users (username, email, password_hash,is_activated,phone, auth_provider )
	VALUES ($1, $2, $3, $4, $5, 'local')
	RETURNING id, created_at 
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query, user.Name, user.Email, user.Password.hash, user.IsActivated, user.Phone).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case err.Error() == `pq: duplicate key value violates unique constraint "users_username_key"`:
			return ErrDuplicateUsername
		default:
			return err
		}
	}

	return nil
}

func (s *UserStore) UpdateUserImage(ctx context.Context, userID int64, imageurl string) error {
	query := `
	UPDATE users
	SET image_url = $1
	WHERE id = $2
	RETURNING image_url
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query, imageurl, userID).Scan(&imageurl)
	if err != nil {
		return err
	}
	return nil
}

// Get user by email
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
	SELECT id, created_at, username, email, password_hash, is_activated, phone, version
	FROM users
	WHERE email = $1`

	var user User

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.IsActivated,
		&user.Phone,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

func (s *UserStore) Update(ctx context.Context, user *User) error {
	query := `
	UPDATE users
	SET username = $1, email = $2, password_hash = $3, is_activated = $4, version = version + 1
	WHERE id = $5
	RETURNING username`

	args := []interface{}{
		user.Name,
		user.Email,
		user.Password.hash,
		user.IsActivated,
		user.ID,
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query, args...).Scan(&user.Name)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (s *UserStore) GetForToken(ctx context.Context, tokenScope, tokenPlainText string) (*User, error) {

	tokenHash := sha256.Sum256([]byte(tokenPlainText))

	// Set up the SQL query.
	query := `
	SELECT users.id, users.created_at, users.username, users.email, users.password_hash, users.is_activated
	FROM users
	INNER JOIN tokens ON users.id = tokens.user_id
	WHERE tokens.hash = $1
	AND tokens.scope = $2
	AND tokens.expiry > $3`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	// Execute the query, scanning the return values into a User struct. If no matching
	// record is found we return an ErrRecordNotFound error.
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.IsActivated,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")
	// Call the standalone ValidateEmail() helper.
	ValidateEmail(v, user.Email)
	// If the plaintext password is not nil, call the standalone
	// ValidatePasswordPlaintext() helper.
	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}
	// If the password hash is ever nil, this will be due to a logic error in our
	// codebase (probably because we forgot to set a password for the user). It's a
	// useful sanity check to include here, but it's not a problem with the data
	// provided by the client. So rather than adding an error to the validation map we
	// raise a panic instead.
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `
	SELECT id, created_at, username, email, phone, role, is_activated, version
	FROM users
	WHERE id = $1
	`

	var user User

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Phone,
		&user.Role,
		&user.IsActivated,
		&user.Version,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UserStore) Delete(ctx context.Context, id int64) error {
	query := `
	DELETE FROM users
	WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}
