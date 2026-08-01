package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stevenwilliam/ruuma/internal/adapter/postgres"
	"github.com/stevenwilliam/ruuma/internal/platform/config"
	"github.com/stevenwilliam/ruuma/internal/platform/id"
	"github.com/stevenwilliam/ruuma/internal/platform/security"
)

// runCreateOwner creates the first owner account on a fresh production
// database.
//
// This is the first-run flow that lets ruuma ship with **no default
// credentials at all** (docs/12, A05): there is no seeded admin to forget to
// change, and the command refuses to run once an owner exists.
func runCreateOwner(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	fs := flag.NewFlagSet("create-owner", flag.ExitOnError)
	email := fs.String("email", "", "owner email address (required)")
	name := fs.String("name", "Owner", "full name")
	password := fs.String("password", "", "password; omit to generate one and print it")
	force := fs.Bool("force", false, "create another owner even if one exists")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	if strings.TrimSpace(*email) == "" {
		return fmt.Errorf("create-owner: --email is required")
	}

	a, err := build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer a.close()

	var existing int64
	if err := a.db.WithContext(ctx).Model(&postgres.User{}).
		Where("role = ? AND is_active", "owner").Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 && !*force {
		// Refusing here is the point: a second silent owner is how a
		// forgotten bootstrap account becomes a back door.
		return fmt.Errorf("create-owner: an active owner already exists; pass --force only if you mean it")
	}

	plain := *password
	generated := false
	if plain == "" {
		token, err := id.Token(20)
		if err != nil {
			return err
		}
		plain = "ruuma-" + token
		generated = true
	}
	if err := security.ValidatePassword(plain); err != nil {
		return fmt.Errorf("create-owner: %w", err)
	}

	hash, err := security.HashPassword(plain)
	if err != nil {
		return err
	}

	user := postgres.User{
		ID: uuid.New(), Email: strings.ToLower(strings.TrimSpace(*email)),
		PasswordHash: hash, FullName: *name, Role: "owner",
		IsActive: true, MustChangePassword: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := a.db.WithContext(ctx).Create(&user).Error; err != nil {
		return fmt.Errorf("create-owner: %w", err)
	}

	if err := a.audit.Write(ctx, postgres.Entry{
		ActorType: "system", Action: "owner.bootstrap",
		EntityType: "user", EntityID: &user.ID, ActorEmail: user.Email,
	}); err != nil {
		log.Warn("bootstrap audit entry failed", "error", err)
	}

	fmt.Printf("Owner created: %s\n", user.Email)
	if generated {
		// Printed once, to the operator's terminal, and never stored anywhere
		// else. The account must change it at first sign-in.
		fmt.Printf("Password (shown once, change it at first sign-in): %s\n", plain)
	}
	return nil
}
