package valorant

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes valorant as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/valorant-cli/valorant"
//
// exactly as a database/sql program enables a driver with
// `import _ "github.com/lib/pq"`. The init below registers it; the host
// then dereferences valorant:// URIs by routing to the operations
// Register installs. The same Domain also builds the standalone valorant
// binary (see cli.NewApp), so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the valorant driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "valorant",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "valorant",
			Short:  "A command line for the Valorant API.",
			Long: `A command line for the Valorant API.

valorant reads public game data from valorant-api.com over HTTPS, shapes it
into clean records, and prints output that pipes into the rest of your tools.
No API key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/valorant-cli",
		},
	}
}

// Agent is a playable agent (character) in VALORANT.
type Agent struct {
	UUID        string `kit:"id" json:"uuid" table:"uuid"`
	Name        string `json:"name" table:"name"`
	Role        string `json:"role" table:"role"`
	Description string `json:"description" table:"-"`
	Abilities   string `json:"abilities" table:"abilities"`
}

// Weapon is a weapon available in VALORANT.
type Weapon struct {
	UUID       string  `kit:"id" json:"uuid" table:"uuid"`
	Name       string  `json:"name" table:"name"`
	Category   string  `json:"category" table:"category"`
	Cost       int     `json:"cost" table:"cost"`
	FireRate   float64 `json:"fire_rate" table:"fire_rate"`
	MagSize    int     `json:"mag_size" table:"mag_size"`
	ReloadTime float64 `json:"reload_time" table:"reload_time"`
}

// Map is a playable map in VALORANT.
type Map struct {
	UUID                string `kit:"id" json:"uuid" table:"uuid"`
	Name                string `json:"name" table:"name"`
	TacticalDescription string `json:"tactical_description" table:"tactical_description"`
	Coordinates         string `json:"coordinates" table:"coordinates"`
}

// Rank is a competitive rank tier in VALORANT.
type Rank struct {
	Tier     int    `kit:"id" json:"tier" table:"tier"`
	Name     string `json:"name" table:"name"`
	Division string `json:"division" table:"division"`
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "agents",
		Group:   "read",
		List:    true,
		Summary: "List playable agents",
	}, listAgents)

	kit.Handle(app, kit.OpMeta{
		Name:    "weapons",
		Group:   "read",
		List:    true,
		Summary: "List weapons",
	}, listWeapons)

	kit.Handle(app, kit.OpMeta{
		Name:    "maps",
		Group:   "read",
		List:    true,
		Summary: "List maps",
	}, listMaps)

	kit.Handle(app, kit.OpMeta{
		Name:    "ranks",
		Group:   "read",
		List:    true,
		Summary: "List competitive rank tiers",
	}, listRanks)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type agentsIn struct {
	Role   string  `kit:"flag" help:"filter by role"`
	All    bool    `kit:"flag" help:"include non-playable characters"`
	Client *Client `kit:"inject"`
}

type weaponsIn struct {
	Category string  `kit:"flag" help:"filter by category"`
	Client   *Client `kit:"inject"`
}

type mapsIn struct {
	Client *Client `kit:"inject"`
}

type ranksIn struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listAgents(ctx context.Context, in agentsIn, emit func(*Agent) error) error {
	agents, err := in.Client.Agents(ctx, in.All)
	if err != nil {
		return mapErr(err)
	}
	for _, a := range agents {
		if in.Role != "" && !strings.EqualFold(a.Role, in.Role) {
			continue
		}
		if err := emit(a); err != nil {
			return err
		}
	}
	return nil
}

func listWeapons(ctx context.Context, in weaponsIn, emit func(*Weapon) error) error {
	weapons, err := in.Client.Weapons(ctx, in.Category)
	if err != nil {
		return mapErr(err)
	}
	for _, w := range weapons {
		if err := emit(w); err != nil {
			return err
		}
	}
	return nil
}

func listMaps(ctx context.Context, in mapsIn, emit func(*Map) error) error {
	maps, err := in.Client.Maps(ctx)
	if err != nil {
		return mapErr(err)
	}
	for _, m := range maps {
		if err := emit(m); err != nil {
			return err
		}
	}
	return nil
}

func listRanks(ctx context.Context, in ranksIn, emit func(*Rank) error) error {
	ranks, err := in.Client.Ranks(ctx)
	if err != nil {
		return mapErr(err)
	}
	for _, r := range ranks {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: pure, network-free string functions ---

// Classify is required by the kit.Domain interface. Since valorant-api.com
// URIs are not addressable like web pages (there are no stable resource paths
// users copy-paste), we return a minimal implementation that at least handles
// the scheme itself.
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty valorant reference")
	}
	// Strip valorant:// scheme if present.
	input = strings.TrimPrefix(input, "valorant://")
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "agents", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "agents":
		return "https://valorant-api.com/v1/agents/" + id, nil
	case "weapons":
		return "https://valorant-api.com/v1/weapons/" + id, nil
	case "maps":
		return "https://valorant-api.com/v1/maps/" + id, nil
	default:
		return "https://valorant-api.com/v1/" + uriType + "/" + id, nil
	}
}

// mapErr converts a library error into the appropriate kit error kind.
func mapErr(err error) error {
	return err
}
