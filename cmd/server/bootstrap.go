package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"legal-roster-api/internal/auth"
	"legal-roster-api/internal/config"
	"legal-roster-api/internal/db"
)

func bootstrapPlatformAdmin(ctx context.Context, cfg config.Config, queries *db.Queries) error {
	email := strings.TrimSpace(strings.ToLower(cfg.BootstrapPlatformAdminEmail))
	password := cfg.BootstrapPlatformAdminPassword
	name := strings.TrimSpace(cfg.BootstrapPlatformAdminName)

	if email == "" && password == "" {
		return nil
	}
	if email == "" || password == "" {
		return fmt.Errorf("BOOTSTRAP_PLATFORM_ADMIN_EMAIL and BOOTSTRAP_PLATFORM_ADMIN_PASSWORD must be set together")
	}
	if len(password) < 12 {
		return fmt.Errorf("BOOTSTRAP_PLATFORM_ADMIN_PASSWORD must be at least 12 characters")
	}
	if name == "" {
		name = "Platform Admin"
	}

	_, err := queries.GetUserByEmail(ctx, email)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check bootstrap platform admin: %w", err)
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		Role:         "platform_admin",
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil
		}
		return fmt.Errorf("create bootstrap platform admin: %w", err)
	}

	return nil
}
