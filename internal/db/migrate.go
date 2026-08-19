// Package db는 Postgres 커넥션과 마이그레이션 적용을 담당한다.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // goose는 database/sql 핸들을 받는다
	"github.com/pressly/goose/v3"
)

// 마이그레이션 SQL을 바이너리에 심는다. 배포 환경에 파일을 따로 올리지 않기 위함이다.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

// Migrate는 임베드된 마이그레이션을 적용한다.
// goose를 CLI가 아니라 라이브러리로 쓰는 이유는 ADR-017에 있다 —
// 개발자가 따로 설치할 바이너리를 늘리지 않기 위해서다.
func Migrate(ctx context.Context, databaseURL string) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
