package library

import (
	"context"
	"fmt"
)

// maxCollectionParts is the ceiling on one saga row.
//
// A trilogy is three and the longest thing anybody actually shelves -- Bond,
// Godzilla -- is under thirty. The cap exists so that a mis-set collection id
// cannot turn a film page into the whole library; it is not paging, and a row
// that hits it is not truncated in any way the user would notice.
const maxCollectionParts = 24

// CollectionParts returns the other films of one film's TMDB collection that
// this library actually holds, in release order.
//
// It lists what can be played, and nothing else. TMDB knows all six parts of a
// saga and would happily say so, but a row of cards for films that are not on
// the disk is a shop window: the home screen is a personal surface, not a second
// catalogue (decision 29), and the same rule holds one page down. A household
// that owns parts one and three sees two films, not one film and two absences.
//
// The film itself is excluded rather than rendered as the current card, because
// the row sits directly under that film's own title.
func (s *Store) CollectionParts(ctx context.Context, profileID, movieID int64, collectionID int) ([]Movie, error) {
	if collectionID <= 0 {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+movieSource+`
		WHERE m.collection_id = ? AND m.id != ?
		ORDER BY
			CASE WHEN m.release_date IS NULL OR m.release_date = '' THEN 1 ELSE 0 END,
			m.release_date,
			m.year,
			m.title COLLATE NOCASE
		LIMIT ?`, profileID, collectionID, movieID, maxCollectionParts)
	if err != nil {
		return nil, fmt.Errorf("reading collection %d: %w", collectionID, err)
	}
	return collectMovies(rows, 0)
}
