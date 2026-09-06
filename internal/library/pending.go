package library

import "context"

func (s *Store) preservePendingFile(ctx context.Context, path string, generation int64) error {
	for _, table := range []string{"movie_files", "episode_files"} {
		if _, err := s.db.ExecContext(ctx, "UPDATE "+table+" SET last_seen_scan = ? WHERE path = ?", generation, path); err != nil {
			return err
		}
	}
	return nil
}
