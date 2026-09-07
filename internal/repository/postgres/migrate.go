package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	migrate "github.com/rubenv/sql-migrate"

	"github.com/belyaevedu/remote-code-service/internal/config"
)

func Migrate(ctx context.Context, cfg config.DBConfig) error {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return fmt.Errorf("migrate open: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error raised closing migration db: %v\n", err)
		}
	}()

	source := &migrate.FileMigrationSource{Dir: cfg.MigrationsDir}

	applied, err := migrate.ExecContext(ctx, db, "postgres", source, migrate.Up)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}

	if applied > 0 {
		log.Printf("applied %d migration(s) from %s", applied, cfg.MigrationsDir)
	}
	return nil
}
