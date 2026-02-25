package auth

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// ErrDuplicateEmail is returned when registering with an email that already exists.
var ErrDuplicateEmail = errors.New("email already registered")

type Account struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	Role         string
	BalanceCents int64
	HoldCents    int64
}

type Service interface {
	Register(ctx context.Context, email, password, displayName, role string) (*Account, error)
	Login(ctx context.Context, email, password string) (string, error)
	ValidateToken(ctx context.Context, token string) (uuid.UUID, string, error)
}

type service struct {
	repo   *Repository
	secret []byte
}

func NewService(repo *Repository) *service {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "supersecretmvp"
	}
	return &service{repo: repo, secret: []byte(secret)}
}

// Ensure service implements Service at compile time.
var _ Service = (*service)(nil)

type claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

func (s *service) Register(ctx context.Context, email, password, displayName, role string) (*Account, error) {
	if role != "requester" && role != "provider" {
		return nil, errors.New("invalid role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	acc, err := s.repo.Create(ctx, email, string(hash), displayName, role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	return acc, nil
}

func (s *service) Login(ctx context.Context, email, password string) (string, error) {
	acc, hash, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if acc == nil {
		return "", errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	return s.issueToken(acc.ID, acc.Role)
}

func (s *service) issueToken(userID uuid.UUID, role string) (string, error) {
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Role: role,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(s.secret)
}

func (s *service) ValidateToken(ctx context.Context, token string) (uuid.UUID, string, error) {
	tok, err := jwt.ParseWithClaims(token, &claims{}, func(t *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	c, ok := tok.Claims.(*claims)
	if !ok || !tok.Valid {
		return uuid.Nil, "", errors.New("invalid token")
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, "", err
	}
	return id, c.Role, nil
}
