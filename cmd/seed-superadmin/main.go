// cmd/seed-superadmin creates a platform_admins row directly against the
// database. This is intentionally a CLI tool, not an HTTP endpoint —
// platform admin accounts should only ever be provisioned by someone with
// direct database/deploy access, never via a public-facing API.
//
// Usage:
//
//	go run ./cmd/seed-superadmin -name "Zeus" -email "zeus@els.dev"
//	(you will be prompted for the password — it is never accepted as a
//	CLI flag, since flag values are visible in shell history and to
//	anyone who can inspect the process list while it's running)
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"mezzani_backend/internal/config"
	db "mezzani_backend/internal/database/sqlc"
)

const pgUniqueViolation = "23505"

// readPassword reads a password from stdin without echoing it back to the
// terminal when stdin is an interactive TTY (via golang.org/x/term).
// When stdin is NOT a terminal (piped/redirected input, e.g. in a script),
// it falls back to a plain buffered read — term.ReadPassword only works
// on a real TTY file descriptor and would fail or hang otherwise.
func readPassword() (string, error) {
	fmt.Print("Password (min 8 chars): ")

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println() // ReadPassword suppresses the newline the user typed
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	// Non-interactive stdin — read a line manually. ReadString('\n')
	// returns the bytes read so far ALONGSIDE io.EOF when input ends
	// without a trailing newline (e.g. `printf '%s' "$PW" | ...`) — that
	// is valid input, not an error, so it must not be rejected. Only the
	// newline delimiter itself is stripped, not general whitespace:
	// TrimSpace would also eat leading/trailing spaces that could
	// legitimately be part of the password.
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

func main() {
	name := flag.String("name", "", "admin's display name")
	email := flag.String("email", "", "admin's login email")
	flag.Parse()

	if *name == "" || *email == "" {
		log.Fatal("usage: go run ./cmd/seed-superadmin -name \"...\" -email \"...\"  (password is prompted)")
	}

	rawEmail := strings.TrimSpace(*email)
	parsedEmail, err := mail.ParseAddress(rawEmail)
	if err != nil {
		log.Fatalf("invalid email address %q: %v", rawEmail, err)
	}
	if parsedEmail.Name != "" {
		// mail.ParseAddress happily accepts "Zeus <zeus@els.dev>" and
		// would otherwise leave us storing the display-name form instead
		// of the bare address the login endpoint actually compares
		// against — silently provisioning an admin who can never log in.
		log.Fatalf("enter a bare email address, not %q — use just the address, e.g. zeus@els.dev", rawEmail)
	}
	normalizedEmail := strings.ToLower(parsedEmail.Address)

	password, err := readPassword()
	if err != nil {
		log.Fatalf("failed to read password: %v", err)
	}
	if len(password) < 8 {
		log.Fatal("password must be at least 8 characters")
	}

	ctx := context.Background()
	cfg := config.LoadConfig()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	// Fail closed: only a genuine "no such row" means it's safe to
	// proceed. Any other error (timeout, connection drop, permission
	// issue) must abort — silently falling through to CreatePlatformAdmin
	// on an error we don't understand could mask a real infra problem.
	// This check is also inherently racy by itself (two people running
	// this concurrently with the same email); platform_admins.email has
	// a UNIQUE constraint backing it, which is the real guard — this is
	// just a friendlier error message for the common case, and the
	// insert below still handles the constraint violation explicitly.
	_, lookupErr := queries.GetPlatformAdminByEmail(ctx, normalizedEmail)
	switch {
	case lookupErr == nil:
		log.Fatalf("a platform admin with email %s already exists", normalizedEmail)
	case !errors.Is(lookupErr, pgx.ErrNoRows):
		log.Fatalf("failed to check for existing platform admin: %v", lookupErr)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	admin, err := queries.CreatePlatformAdmin(ctx, db.CreatePlatformAdminParams{
		ID:           uuid.New(),
		Name:         *name,
		Email:        normalizedEmail,
		PasswordHash: string(hash),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			log.Fatalf("a platform admin with email %s already exists", normalizedEmail)
		}
		log.Fatalf("failed to create platform admin: %v", err)
	}

	fmt.Printf("Created platform admin: %s <%s> (id: %s)\n", admin.Name, admin.Email, admin.ID)
}
