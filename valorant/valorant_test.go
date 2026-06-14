package valorant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Tests hit httptest servers that mimic valorant-api.com, so they are fast
// and offline. Since buildURL is unexported we construct request URLs manually.

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestAgents(t *testing.T) {
	agents := []wireAgent{
		{
			UUID:                "uuid-1",
			DisplayName:         "Gekko",
			Description:         "Angeleno",
			IsPlayableCharacter: true,
			Role: &struct {
				DisplayName string `json:"displayName"`
			}{"Initiator"},
			Abilities: []struct {
				Slot        string `json:"slot"`
				DisplayName string `json:"displayName"`
			}{
				{"Ability1", "Wingman"},
				{"Ultimate", "Thrash"},
			},
		},
		{
			UUID:                "uuid-2",
			DisplayName:         "Jett",
			Description:         "Duelist agent",
			IsPlayableCharacter: true,
			Role: &struct {
				DisplayName string `json:"displayName"`
			}{"Duelist"},
			Abilities: []struct {
				Slot        string `json:"slot"`
				DisplayName string `json:"displayName"`
			}{
				{"Ability1", "Cloudburst"},
				{"Ultimate", "Blade Storm"},
			},
		},
	}
	resp := wireResp[[]wireAgent]{Status: 200, Data: agents}
	payload, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	// Fetch via direct URL since we can't override BaseURL without it being exported.
	body, err := c.Get(context.Background(), srv.URL+"/agents?language=en-US&isPlayableCharacter=true")
	if err != nil {
		t.Fatal(err)
	}
	var decoded wireResp[[]wireAgent]
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 2 {
		t.Fatalf("got %d agents, want 2", len(decoded.Data))
	}
	if decoded.Data[0].DisplayName != "Gekko" {
		t.Errorf("agent[0] = %q, want Gekko", decoded.Data[0].DisplayName)
	}
	if decoded.Data[0].Role.DisplayName != "Initiator" {
		t.Errorf("agent[0].role = %q, want Initiator", decoded.Data[0].Role.DisplayName)
	}
}

func TestWeapons(t *testing.T) {
	weapons := []wireWeapon{
		{
			UUID:        "w-uuid-1",
			DisplayName: "Vandal",
			ShopData: &struct {
				Cost     int    `json:"cost"`
				Category string `json:"category"`
			}{2900, "Rifles"},
			WeaponStats: &struct {
				FireRate          float64 `json:"fireRate"`
				MagazineSize      int     `json:"magazineSize"`
				ReloadTimeSeconds float64 `json:"reloadTimeSeconds"`
			}{9.75, 25, 2.5},
		},
		{
			UUID:        "w-uuid-2",
			DisplayName: "Classic",
			ShopData: &struct {
				Cost     int    `json:"cost"`
				Category string `json:"category"`
			}{0, "Sidearms"},
			WeaponStats: &struct {
				FireRate          float64 `json:"fireRate"`
				MagazineSize      int     `json:"magazineSize"`
				ReloadTimeSeconds float64 `json:"reloadTimeSeconds"`
			}{6.75, 12, 1.75},
		},
	}
	resp := wireResp[[]wireWeapon]{Status: 200, Data: weapons}
	payload, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL+"/weapons?language=en-US")
	if err != nil {
		t.Fatal(err)
	}
	var decoded wireResp[[]wireWeapon]
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 2 {
		t.Fatalf("got %d weapons, want 2", len(decoded.Data))
	}
	if decoded.Data[0].DisplayName != "Vandal" {
		t.Errorf("weapon[0] = %q, want Vandal", decoded.Data[0].DisplayName)
	}
	if decoded.Data[0].ShopData.Category != "Rifles" {
		t.Errorf("weapon[0].category = %q, want Rifles", decoded.Data[0].ShopData.Category)
	}
}

func TestMaps(t *testing.T) {
	maps := []wireMap{
		{
			UUID:                 "m-uuid-1",
			DisplayName:          "Ascent",
			NarrativeDescription: "A sunlit map in Venice.",
			TacticalDescription:  "5v5 Bomb Defuse",
			Coordinates:          "45°26'BF'N,12°20'Q'E",
		},
		{
			UUID:                 "m-uuid-2",
			DisplayName:          "Bind",
			NarrativeDescription: "A map with teleporters.",
			TacticalDescription:  "5v5 Bomb Defuse",
			Coordinates:          "34°01'N,6°50'W",
		},
	}
	resp := wireResp[[]wireMap]{Status: 200, Data: maps}
	payload, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL+"/maps?language=en-US")
	if err != nil {
		t.Fatal(err)
	}
	var decoded wireResp[[]wireMap]
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 2 {
		t.Fatalf("got %d maps, want 2", len(decoded.Data))
	}
	if decoded.Data[0].DisplayName != "Ascent" {
		t.Errorf("map[0] = %q, want Ascent", decoded.Data[0].DisplayName)
	}
	if decoded.Data[0].TacticalDescription != "5v5 Bomb Defuse" {
		t.Errorf("map[0].tactical = %q, want '5v5 Bomb Defuse'", decoded.Data[0].TacticalDescription)
	}
}

func TestRanks(t *testing.T) {
	oldEpisode := wireTierSet{
		Tiers: []struct {
			Tier         int    `json:"tier"`
			TierName     string `json:"tierName"`
			DivisionName string `json:"divisionName"`
		}{
			{0, "Unranked", "Unranked"},
		},
	}
	newEpisode := wireTierSet{
		Tiers: []struct {
			Tier         int    `json:"tier"`
			TierName     string `json:"tierName"`
			DivisionName string `json:"divisionName"`
		}{
			{0, "Unranked", "Unranked"},
			{3, "Iron 1", "Iron"},
			{4, "Iron 2", "Iron"},
			{5, "Iron 3", "Iron"},
			{21, "Radiant", "Radiant"},
		},
	}
	resp := wireResp[[]wireTierSet]{Status: 200, Data: []wireTierSet{oldEpisode, newEpisode}}
	payload, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL+"/competitivetiers?language=en-US")
	if err != nil {
		t.Fatal(err)
	}
	var decoded wireResp[[]wireTierSet]
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Data) != 2 {
		t.Fatalf("got %d tier sets, want 2", len(decoded.Data))
	}
	// The last episode is the one we care about.
	last := decoded.Data[len(decoded.Data)-1]
	if len(last.Tiers) != 5 {
		t.Fatalf("last episode has %d tiers, want 5", len(last.Tiers))
	}
	if last.Tiers[1].TierName != "Iron 1" {
		t.Errorf("tier[1] = %q, want Iron 1", last.Tiers[1].TierName)
	}
	if last.Tiers[4].TierName != "Radiant" {
		t.Errorf("tier[4] = %q, want Radiant", last.Tiers[4].TierName)
	}
}
