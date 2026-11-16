// Command seed prepares the demo catalog. It has two independent steps:
//
//	go run ./cmd/seed fetch          Steam store -> seed/steam_games.json (needs internet)
//	go run ./cmd/seed load           seed/steam_games.json -> database in DATABASE_URL
//	go run ./cmd/seed load -reset    same, but first empties the catalog and order tables
//
// Run it from the backend/ folder. "fetch" is run rarely and its result is
// committed; "load" works offline from that committed file, so seeding a database
// never depends on Steam being reachable.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/config"
	"github.com/Jokubas-J/Eneba/backend/internal/database"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
)

const (
	appIDsFile   = "seed/app_ids.txt"
	snapshotFile = "seed/steam_games.json"

	// The generated data (which seller sells what, at which price, with how many
	// keys) comes from a random number generator started from this fixed number, so
	// every run produces exactly the same catalog.
	randomSeed = 2027
)

func main() {
	log.SetFlags(0) // a command-line tool does not need timestamps on every line

	if len(os.Args) < 2 {
		log.Fatal("usage: seed fetch | seed load [-reset]")
	}

	var err error
	switch os.Args[1] {
	case "fetch":
		err = runFetch()
	case "load":
		err = runLoad(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q (want fetch or load)", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

// runFetch downloads every app listed in app_ids.txt and writes the snapshot.
func runFetch() error {
	appIDs, err := readAppIDs(appIDsFile)
	if err != nil {
		return err
	}

	ctx := context.Background()
	client := &http.Client{Timeout: 30 * time.Second}

	var games []seed.Game
	skipped := 0
	for i, appID := range appIDs {
		// Steam allows roughly 200 requests per 5 minutes; one request every 1.6 s
		// stays below that without any cleverness.
		if i > 0 {
			time.Sleep(1600 * time.Millisecond)
		}

		game, err := fetchWithRetry(ctx, client, appID)
		switch {
		case errors.Is(err, seed.ErrSkip):
			skipped++
			log.Printf("[%d/%d] %d: %v", i+1, len(appIDs), appID, err)
		case err != nil:
			// One broken id must not throw away several minutes of downloads.
			skipped++
			log.Printf("[%d/%d] %d: ERROR %v", i+1, len(appIDs), appID, err)
		default:
			games = append(games, game)
			log.Printf("[%d/%d] %d: %s", i+1, len(appIDs), appID, game.Name)
		}
	}

	// Sorted output keeps the committed file stable: re-fetching changes only the
	// lines whose data really changed, which makes the git diff readable.
	sort.Slice(games, func(a, b int) bool { return games[a].SteamAppID < games[b].SteamAppID })

	data, err := json.MarshalIndent(games, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(snapshotFile, append(data, '\n'), 0o644); err != nil {
		return err
	}
	log.Printf("wrote %d games to %s (%d ids skipped)", len(games), snapshotFile, skipped)
	return nil
}

// fetchWithRetry waits and tries again when Steam says "too many requests".
func fetchWithRetry(ctx context.Context, client *http.Client, appID int) (seed.Game, error) {
	for attempt := 1; ; attempt++ {
		game, err := seed.FetchGame(ctx, client, appID)
		if !seed.IsRateLimited(err) || attempt == 4 {
			return game, err
		}
		log.Printf("rate limited by Steam, waiting 90 seconds (attempt %d)", attempt)
		time.Sleep(90 * time.Second)
	}
}

// readAppIDs reads one id per line; "#" starts a comment. Duplicates are dropped.
func readAppIDs(path string) ([]int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ids []int
	seen := map[int]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text, _, _ := strings.Cut(scanner.Text(), "#")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		id, err := strconv.Atoi(text)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not an app id", path, text)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, scanner.Err()
}

// runLoad generates the catalog from the snapshot and writes it to the database.
func runLoad(args []string) error {
	flags := flag.NewFlagSet("load", flag.ExitOnError)
	reset := flags.Bool("reset", false, "empty the catalog, key and order tables first")
	if err := flags.Parse(args); err != nil {
		return err
	}

	games, err := readSnapshot(snapshotFile)
	if err != nil {
		return err
	}
	dataset := seed.Generate(games, randomSeed)

	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The tables must exist before rows can be inserted; this is the same call the
	// server makes at start-up.
	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}

	counts, err := seed.Load(ctx, pool, dataset, *reset)
	if err != nil {
		return err
	}
	log.Printf("loaded %d products, %d offers, %d keys, %d sellers, %d genres",
		counts.Products, counts.Offers, counts.Keys, counts.Sellers, counts.Genres)
	return nil
}

func readSnapshot(path string) ([]seed.Game, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read snapshot (run \"seed fetch\" first?): %w", err)
	}
	var games []seed.Game
	if err := json.Unmarshal(data, &games); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return games, nil
}
