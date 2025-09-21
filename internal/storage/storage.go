package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Storage struct {
	db *sql.DB
}

type User struct {
	ID         int
	TelegramID int64
	Username   sql.NullString
	KeyID      sql.NullString
	ExpiresAt  sql.NullTime
	Status     string
}

type Payment struct {
	ID            int
	UserID        int
	ScreenshotURL string
	Status        string
	Comment       sql.NullString
	CreatedAt     time.Time
}

func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

func (s *Storage) UpsertUser(ctx context.Context, telegramID int64, username string) (*User, error) {
	query := `INSERT INTO users (telegram_id, username)
VALUES ($1, $2)
ON CONFLICT (telegram_id) DO UPDATE SET username = EXCLUDED.username
RETURNING id, telegram_id, username, key_id, expires_at, status`
	row := s.db.QueryRowContext(ctx, query, telegramID, username)
	var u User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.KeyID, &u.ExpiresAt, &u.Status); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Storage) GetUserByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, telegram_id, username, key_id, expires_at, status FROM users WHERE telegram_id=$1`, telegramID)
	var u User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.KeyID, &u.ExpiresAt, &u.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (s *Storage) GetUserByID(ctx context.Context, id int) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, telegram_id, username, key_id, expires_at, status FROM users WHERE id=$1`, id)
	var u User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.KeyID, &u.ExpiresAt, &u.Status); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Storage) UpdateUserKey(ctx context.Context, userID int, keyID string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET key_id=$1, expires_at=$2 WHERE id=$3`, keyID, expiresAt, userID)
	return err
}

func (s *Storage) UpdateUserStatus(ctx context.Context, userID int, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET status=$1 WHERE id=$2`, status, userID)
	return err
}

func (s *Storage) CreatePayment(ctx context.Context, userID int, screenshotURL string) (*Payment, error) {
	query := `INSERT INTO payments (user_id, screenshot_url) VALUES ($1, $2) RETURNING id, user_id, screenshot_url, status, comment, created_at`
	row := s.db.QueryRowContext(ctx, query, userID, screenshotURL)
	var p Payment
	if err := row.Scan(&p.ID, &p.UserID, &p.ScreenshotURL, &p.Status, &p.Comment, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Storage) UpdatePaymentStatus(ctx context.Context, paymentID int, status string, comment *string) error {
	if comment != nil {
		_, err := s.db.ExecContext(ctx, `UPDATE payments SET status=$1, comment=$2 WHERE id=$3`, status, *comment, paymentID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE payments SET status=$1 WHERE id=$2`, status, paymentID)
	return err
}

func (s *Storage) GetPayment(ctx context.Context, paymentID int) (*Payment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, user_id, screenshot_url, status, comment, created_at FROM payments WHERE id=$1`, paymentID)
	var p Payment
	if err := row.Scan(&p.ID, &p.UserID, &p.ScreenshotURL, &p.Status, &p.Comment, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Storage) ListUsersExpiringBetween(ctx context.Context, from, to time.Time) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, telegram_id, username, key_id, expires_at, status FROM users WHERE expires_at BETWEEN $1 AND $2`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &u.KeyID, &u.ExpiresAt, &u.Status); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
