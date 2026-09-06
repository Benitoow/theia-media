package library

import "context"

func (s *Service) Watchlist(ctx context.Context, profileID int64) ([]int64, error) {
	rows, err := s.store.db.QueryContext(ctx, "SELECT movie_id FROM profile_watchlist WHERE profile_id = ? ORDER BY added_at DESC, movie_id", profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) SetWatchlist(ctx context.Context, profileID, id int64, saved bool) error {
	if _, err := s.Get(ctx, profileID, id); err != nil {
		return err
	}
	if saved {
		_, err := s.store.db.ExecContext(ctx, "INSERT INTO profile_watchlist (profile_id,movie_id) VALUES (?,?) ON CONFLICT DO NOTHING", profileID, id)
		return err
	}
	_, err := s.store.db.ExecContext(ctx, "DELETE FROM profile_watchlist WHERE profile_id=? AND movie_id=?", profileID, id)
	return err
}
