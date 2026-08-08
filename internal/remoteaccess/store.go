package remoteaccess

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
)

type store struct {
	db *sql.DB
}

func newStore(database *sql.DB) *store { return &store{db: database} }

func (s *store) config(ctx context.Context) (Config, error) {
	var cfg Config
	var enabled, automatic int
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled, listen_port, endpoint, updated_at,
		       automatic, mapped_method, mapped_port
		FROM remote_access_config
		WHERE id = 1`).Scan(&enabled, &cfg.ListenPort, &cfg.Endpoint, &cfg.UpdatedAt,
		&automatic, &cfg.MappedMethod, &cfg.MappedPort)
	if err != nil {
		return Config{}, fmt.Errorf("reading remote access configuration: %w", err)
	}
	cfg.Enabled = enabled != 0
	cfg.Automatic = automatic != 0
	return cfg, nil
}

func (s *store) saveConfig(ctx context.Context, cfg Config) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE remote_access_config
		SET enabled = ?, listen_port = ?, endpoint = ?, updated_at = unixepoch(),
		    automatic = ?, mapped_method = ?, mapped_port = ?
		WHERE id = 1`, cfg.Enabled, cfg.ListenPort, cfg.Endpoint,
		cfg.Automatic, cfg.MappedMethod, cfg.MappedPort)
	if err != nil {
		return fmt.Errorf("saving remote access configuration: %w", err)
	}
	return nil
}

func (s *store) peers(ctx context.Context) ([]Peer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, public_key, address, created_at
		FROM remote_access_peers
		WHERE revoked_at IS NULL
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing remote access peers: %w", err)
	}
	defer rows.Close()

	peers := []Peer{}
	for rows.Next() {
		var peer Peer
		if err := rows.Scan(&peer.ID, &peer.Name, &peer.PublicKey, &peer.Address, &peer.CreatedAt); err != nil {
			return nil, fmt.Errorf("reading a remote access peer: %w", err)
		}
		peers = append(peers, peer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing remote access peers: %w", err)
	}
	return peers, nil
}

func (s *store) createPeer(ctx context.Context, name, publicKey string) (Peer, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Peer{}, fmt.Errorf("starting remote peer creation: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM remote_access_peers WHERE revoked_at IS NULL`,
	).Scan(&count); err != nil {
		return Peer{}, fmt.Errorf("counting remote access peers: %w", err)
	}
	if count >= MaxPeers {
		return Peer{}, ErrPeerLimit
	}

	used := map[string]bool{}
	rows, err := tx.QueryContext(ctx,
		`SELECT address FROM remote_access_peers WHERE revoked_at IS NULL`)
	if err != nil {
		return Peer{}, fmt.Errorf("reading remote peer addresses: %w", err)
	}
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			rows.Close()
			return Peer{}, fmt.Errorf("reading a remote peer address: %w", err)
		}
		used[address] = true
	}
	if err := rows.Close(); err != nil {
		return Peer{}, fmt.Errorf("closing remote peer addresses: %w", err)
	}
	if err := rows.Err(); err != nil {
		return Peer{}, fmt.Errorf("reading remote peer addresses: %w", err)
	}

	address := ""
	next := netip.MustParseAddr(TunnelServerAddress).Next()
	for i := 0; i < 253; i++ {
		candidate := next.String()
		if !used[candidate] {
			address = candidate
			break
		}
		next = next.Next()
	}
	if address == "" {
		return Peer{}, ErrPeerLimit
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO remote_access_peers (name, public_key, address)
		VALUES (?, ?, ?)`, name, publicKey, address)
	if err != nil {
		return Peer{}, fmt.Errorf("creating remote access peer: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Peer{}, fmt.Errorf("reading remote access peer id: %w", err)
	}

	var peer Peer
	if err := tx.QueryRowContext(ctx, `
		SELECT id, name, public_key, address, created_at
		FROM remote_access_peers WHERE id = ?`, id,
	).Scan(&peer.ID, &peer.Name, &peer.PublicKey, &peer.Address, &peer.CreatedAt); err != nil {
		return Peer{}, fmt.Errorf("reading created remote access peer: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Peer{}, fmt.Errorf("committing remote access peer: %w", err)
	}
	return peer, nil
}

func (s *store) revokePeer(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE remote_access_peers
		SET revoked_at = unixepoch()
		WHERE id = ? AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoking remote access peer: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking revoked remote access peer: %w", err)
	}
	if count == 0 {
		return ErrPeerNotFound
	}
	return nil
}

func (s *store) undoRevoke(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE remote_access_peers SET revoked_at = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("restoring remote access peer: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking restored remote access peer: %w", err)
	}
	if count == 0 {
		return errors.New("remote access peer disappeared during rollback")
	}
	return nil
}

func (s *store) discardPeer(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM remote_access_peers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("discarding remote access peer: %w", err)
	}
	return nil
}
