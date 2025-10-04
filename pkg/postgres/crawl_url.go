package postgres

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresClient struct {
	pool *pgxpool.Pool
}

func NewClient(dbUrl string) (*PostgresClient, error) {
	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	m, err := migrate.New(
		"file://migrations",
		dbUrl,
	)
	if err != nil {
		return nil, err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return nil, err
	}

	return &PostgresClient{
		pool: pool,
	}, nil
}

func (c *PostgresClient) InsertOne(ctx context.Context, url string, isDownloadable bool, downloadStatus string) error {
	query := `
		INSERT INTO crawl_url (id, url, is_downloadable, download_status)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	id := uuid.NewString()
	err := c.pool.QueryRow(ctx, query, id, url, isDownloadable, downloadStatus).Scan(&id)
	if err != nil {
		return fmt.Errorf("unable to insert crawl URL: %w", err)
	}

	return nil
}

func (c *PostgresClient) UpdateDownloadStatus(ctx context.Context, id string, status string) error {
	query := `
		UPDATE crawl_url
		SET download_status = $1
		WHERE id = $2
	`

	_, err := c.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("unable to update download status: %w", err)
	}

	return nil
}
