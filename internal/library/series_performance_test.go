package library

import (
	"context"
	"fmt"
	theiadb "github.com/Benitoow/theia-media/internal/db"
	"path/filepath"
	"testing"
)

func BenchmarkContinue12From1000Episodes(b *testing.B)         { benchmarkContinue(b, false) }
func BenchmarkContinue12From1000EpisodesNPlusOne(b *testing.B) { benchmarkContinue(b, true) }
func benchmarkContinue(b *testing.B, reference bool) {
	database, err := theiadb.Open(context.Background(), filepath.Join(b.TempDir(), "series.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()
	tx, err := database.Begin()
	if err != nil {
		b.Fatal(err)
	}
	for series := 1; series <= 100; series++ {
		if _, err := tx.Exec(`INSERT INTO series(id,title,added_at,updated_at) VALUES (?,?,1,1)`, series, fmt.Sprint("Series ", series)); err != nil {
			b.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO seasons(id,series_id,season_number,added_at,updated_at) VALUES (?,?,1,1,1)`, series, series); err != nil {
			b.Fatal(err)
		}
		for number := 1; number <= 10; number++ {
			id := (series-1)*10 + number
			for _, statement := range []struct {
				sql  string
				args []any
			}{
				{`INSERT INTO episodes(id,season_id,episode_number,name) VALUES (?,?,?,?)`, []any{id, series, number, "Episode"}},
				{`INSERT INTO episode_items(id,season_id,episode_key,first_episode,last_episode,duration_seconds,added_at,updated_at) VALUES (?,?,?,?,?,1200,1,1)`, []any{id, series, fmt.Sprint(number), number, number}},
				{`INSERT INTO episode_item_members(episode_item_id,episode_id,ordinal) VALUES (?,?,0)`, []any{id, id}},
				{`INSERT INTO episode_progress(profile_id,episode_item_id,position_seconds,watched_at,finished) VALUES (1,?,420,?,0)`, []any{id, id}},
			} {
				if _, err := tx.Exec(statement.sql, statement.args...); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	store := NewStore(database)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var items []EpisodeItem
		var err error
		if reference {
			items, err = store.continueEpisodeRows(context.Background(), 1, 12)
			if err == nil {
				for index, item := range items {
					detail, detailErr := store.GetEpisodeItem(context.Background(), 1, item.ID)
					if detailErr != nil {
						b.Fatal(detailErr)
					}
					items[index] = detail
				}
			}
		} else {
			items, err = store.ContinueEpisodes(context.Background(), 1, 12)
		}
		if err != nil || len(items) != 12 {
			b.Fatalf("continue: %d %v", len(items), err)
		}
	}
}
