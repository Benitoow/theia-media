package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEmptyHomeContractFixture(t *testing.T) {
	service, _ := newTestService(t)
	home, err := service.HomeScreen(t.Context(), defaultProfileID, 12)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(home, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("..", "..", "player", "contract-fixtures", "home-empty.json")
	if os.Getenv("THEIA_WRITE_CONTRACT") == "1" {
		if err := os.WriteFile(fixture, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	if home.Total != 0 || home.Hero != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("empty home contract changed: %s", raw)
	}
}
