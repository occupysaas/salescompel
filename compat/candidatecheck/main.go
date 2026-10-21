// candidatecheck performs read-only connectivity/version checks; it never opens the platform catalog.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"os"
	"time"
)

type result struct {
	Reachable bool `json:"reachable"`
	Major     int  `json:"major"`
}

func major(version int) int { return version / 10000 }
func check(ctx context.Context, url string) (result, error) {
	c, e := pgx.Connect(ctx, url)
	if e != nil {
		return result{}, errors.New("DATABASE_UNAVAILABLE")
	}
	defer c.Close(ctx)
	var version int
	if e = c.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); e != nil {
		return result{}, errors.New("DATABASE_UNAVAILABLE")
	}
	return result{true, major(version)}, nil
}
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, e := check(ctx, os.Getenv("DATABASE_URL"))
	if e != nil || r.Major != 17 {
		json.NewEncoder(os.Stdout).Encode(result{})
		os.Exit(2)
	}
	json.NewEncoder(os.Stdout).Encode(r)
}
