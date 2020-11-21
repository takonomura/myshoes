package mysql

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/whywaita/myshoes/pkg/datastore"

	"github.com/jmoiron/sqlx"

	// mysql driver
	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	Conn *sqlx.DB
}

// New create mysql connection
func New(dsn string) (*MySQL, error) {
	conn, err := sqlx.Open("mysql", dsn+"?parseTime=true")
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connection: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to conn.Ping: %w", err)
	}

	return &MySQL{
		Conn: conn,
	}, nil
}

func (m *MySQL) CreateTarget(target datastore.Target) error {
	query := `INSERT INTO targets(uuid, scope, ghe_domain, github_personal_token) VALUES (?, ?, ?, ?)`
	if _, err := m.Conn.Exec(query, target.UUID, target.Scope, target.GHEDomain, target.GitHubPersonalToken); err != nil {
		return fmt.Errorf("failed to execute INSERT query: %w", err)
	}

	return nil
}

func (m *MySQL) GetTarget(uuid string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, ghe_domain, github_personal_token, created_at, updated_at FROM targets WHERE uuid = "%s"`, uuid)
	if err := m.Conn.Get(&t, query); err != nil {
		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) GetTargetByScope(gheDomain, scope string) (*datastore.Target, error) {
	var t datastore.Target
	query := fmt.Sprintf(`SELECT uuid, scope, ghe_domain, github_personal_token, created_at, updated_at FROM targets WHERE ghe_domain = "%s" AND scope = "%s"`, gheDomain, scope)
	if err := m.Conn.Get(&t, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrNotFound
		}

		return nil, fmt.Errorf("failed to execute SELECT query: %w", err)
	}

	return &t, nil
}

func (m *MySQL) DeleteTarget(uuid string) error {
	query := fmt.Sprintf(`DELETE FROM targets WHERE uuid = "%s"`, uuid)
	if _, err := m.Conn.Exec(query, uuid); err != nil {
		return fmt.Errorf("failed to execute DELETE query: %w", err)
	}

	return nil
}
