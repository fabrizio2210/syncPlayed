package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type ItemsResponse struct {
	Items            []Item `json:"Items"`
	TotalRecordCount int    `json:"TotalRecordCount"`
	StartIndex       int    `json:"StartIndex"`
}

type Item struct {
	Name         string    `json:"Name"`
	Id           string    `json:"Id"`
	RunTimeTicks int64     `json:"RunTimeTicks"`
	UserData     *UserData `json:"UserData"`
}

type UserData struct {
	PlaybackPositionTicks int64 `json:"PlaybackPositionTicks"`
	PlayCount             int   `json:"PlayCount"`
	Played                bool  `json:"Played"`
}

type ServerConfig struct {
	Host   string
	UserID string
	Token  string
}

func ensureScheme(h string) string {
	if strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://") {
		return strings.TrimRight(h, "/")
	}
	return "https://" + strings.TrimRight(h, "/")
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func fetchPlayedItems(cfg ServerConfig) ([]Item, error) {
	client := httpClient()
	host := ensureScheme(cfg.Host)
	var results []Item
	limit := 100
	start := 0

	for {
		u := fmt.Sprintf("%s/Users/%s/Items?IncludeItemTypes=Movie&Recursive=true&Limit=%d&StartIndex=%d", host, url.PathEscape(cfg.UserID), limit, start)
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=\"%s\"", cfg.Token))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetch items failed: %s -> %s", resp.Status, string(body))
		}

		var ir ItemsResponse
		if err := json.Unmarshal(body, &ir); err != nil {
			return nil, err
		}

		for _, it := range ir.Items {
			if it.UserData != nil && (it.UserData.Played || it.UserData.PlayCount > 0) {
				results = append(results, it)
			}
		}

		if len(ir.Items) < limit {
			break
		}
		start += limit
	}

	return results, nil
}

func searchItems(cfg ServerConfig, term string) ([]Item, error) {
	client := httpClient()
	host := ensureScheme(cfg.Host)
	u := fmt.Sprintf("%s/Items?userId=%s&limit=100&recursive=true&searchTerm=%s", host, url.QueryEscape(cfg.UserID), url.QueryEscape(term))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=\"%s\"", cfg.Token))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search items failed: %s -> %s", resp.Status, string(body))
	}

	var ir ItemsResponse
	if err := json.Unmarshal(body, &ir); err != nil {
		return nil, err
	}
	return ir.Items, nil
}

func markPlayed(cfg ServerConfig, itemId string) error {
	client := httpClient()
	host := ensureScheme(cfg.Host)
	u := fmt.Sprintf("%s/UserPlayedItems/%s?userId=%s", host, url.PathEscape(itemId), url.QueryEscape(cfg.UserID))
	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("MediaBrowser Token=\"%s\"", cfg.Token))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mark played failed: %s", resp.Status)
	}
	return nil
}

func findMatchingItem(src Item, candidates []Item) *Item {
	// Prefer same Id
	for _, c := range candidates {
		if strings.EqualFold(c.Id, src.Id) {
			return &c
		}
	}
	// Then prefer exact name
	for _, c := range candidates {
		if c.Name == src.Name {
			return &c
		}
	}
	// Fallback: try runtime match
	for _, c := range candidates {
		if src.RunTimeTicks != 0 && src.RunTimeTicks == c.RunTimeTicks {
			return &c
		}
	}
	return nil
}

func syncDirection(fromCfg, toCfg ServerConfig, dryRun bool) (int, error) {
	played, err := fetchPlayedItems(fromCfg)
	if err != nil {
		return 0, err
	}
	if len(played) == 0 {
		return 0, nil
	}

	marked := 0
	for _, it := range played {
		candidates, err := searchItems(toCfg, it.Name)
		if err != nil {
			// continue on search errors to avoid stopping whole run
			fmt.Fprintf(os.Stderr, "search error for %s: %v\n", it.Name, err)
			continue
		}
		match := findMatchingItem(it, candidates)
		if match == nil {
			// not found
			continue
		}
		if match.UserData != nil && (match.UserData.Played || match.UserData.PlayCount > 0) {
			// already played
			continue
		}
		if dryRun {
			fmt.Printf("DRY-RUN: would mark played: %s -> %s (item %s)\n", fromCfg.Host, toCfg.Host, match.Id)
			marked++
			continue
		}
		if err := markPlayed(toCfg, match.Id); err != nil {
			fmt.Fprintf(os.Stderr, "failed to mark %s as played on %s: %v\n", match.Id, toCfg.Host, err)
			continue
		}
		fmt.Printf("Marked played: %s on %s (item %s)\n", match.Name, toCfg.Host, match.Id)
		marked++
	}

	return marked, nil
}

func main() {
	var aHost, aUser, aToken string
	var bHost, bUser, bToken string
	var dryRun bool

	flag.StringVar(&aHost, "a-host", "", "Host for server A (e.g. jellyfin.no-ip.dynu.net or https://...)")
	flag.StringVar(&aUser, "a-user", "", "User ID on server A")
	flag.StringVar(&aToken, "a-token", "", "API token for server A")
	flag.StringVar(&bHost, "b-host", "", "Host for server B")
	flag.StringVar(&bUser, "b-user", "", "User ID on server B")
	flag.StringVar(&bToken, "b-token", "", "API token for server B")
	flag.BoolVar(&dryRun, "dry-run", true, "If true, do not actually mark items; only print actions")
	flag.Parse()

	if aHost == "" || aUser == "" || aToken == "" || bHost == "" || bUser == "" || bToken == "" {
		fmt.Fprintln(os.Stderr, "Missing required flags. See -help for usage.")
		flag.Usage()
		os.Exit(2)
	}

	a := ServerConfig{Host: aHost, UserID: aUser, Token: aToken}
	b := ServerConfig{Host: bHost, UserID: bUser, Token: bToken}

	fmt.Println("Starting sync (dry-run:", dryRun, ")")

	// A -> B
	fmt.Printf("Syncing played from %s -> %s\n", a.Host, b.Host)
	n1, err := syncDirection(a, b, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error syncing A->B: %v\n", err)
	}

	// B -> A
	fmt.Printf("Syncing played from %s -> %s\n", b.Host, a.Host)
	n2, err := syncDirection(b, a, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error syncing B->A: %v\n", err)
	}

	if n1 == 0 && n2 == 0 {
		fmt.Println("No items to mark.")
	} else {
		fmt.Printf("Done. Marked %d items A->B and %d items B->A (dry-run=%v)\n", n1, n2, dryRun)
	}
}
