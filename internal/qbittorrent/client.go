package qbittorrent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func New(baseURL, username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent: create cookie jar: %w", err)
	}

	c := &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}

	if err := c.login(); err != nil {
		return nil, err
	}

	return c, nil
}

// ─── Models ───────────────────────────────────────────────────────────

type Torrent struct {
	Hash        string  `json:"hash"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	ContentPath string  `json:"content_path"`
	SavePath    string  `json:"save_path"`
	Ratio       float64 `json:"ratio"`
	AmountLeft  int64   `json:"amount_left"`
	Completed   int64   `json:"completed"`
}

// ─── Auth ─────────────────────────────────────────────────────────────

func (c *Client) login() error {
	resp, err := c.httpClient.PostForm(c.baseURL+"/api/v2/auth/login", url.Values{
		"username": {c.username},
		"password": {c.password},
	})
	if err != nil {
		return fmt.Errorf("qbittorrent: login request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Ok." {
		return fmt.Errorf("qbittorrent: login failed — check credentials")
	}

	return nil
}

// ─── Methods ──────────────────────────────────────────────────────────

// GetTorrents retourne tous les torrents (optionnellement filtrés par état).
func (c *Client) GetTorrents(filter string) ([]Torrent, error) {
	endpoint := "/api/v2/torrents/info"
	if filter != "" {
		endpoint += "?filter=" + filter
	}

	resp, err := c.httpClient.Get(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent: get torrents: %w", err)
	}
	defer resp.Body.Close()

	var torrents []Torrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("qbittorrent: decode torrents: %w", err)
	}
	return torrents, nil
}

// GetPausedTorrents retourne uniquement les torrents en pause (seeding terminé).
func (c *Client) GetPausedTorrents() ([]Torrent, error) {
	return c.GetTorrents("paused")
}

// DeleteTorrent supprime un torrent par son hash.
// deleteFiles = true supprime aussi les fichiers du disque.
func (c *Client) DeleteTorrent(hash string, deleteFiles bool) error {
	resp, err := c.httpClient.PostForm(c.baseURL+"/api/v2/torrents/delete", url.Values{
		"hashes":      {hash},
		"deleteFiles": {fmt.Sprintf("%v", deleteFiles)},
	})
	if err != nil {
		return fmt.Errorf("qbittorrent: delete torrent: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// DeleteTorrentByPath supprime le torrent dont le path correspond.
func (c *Client) DeleteTorrentByPath(filePath string, deleteFiles bool) error {
	torrents, err := c.GetTorrents("")
	if err != nil {
		return err
	}

	for _, t := range torrents {
		if strings.HasPrefix(filePath, t.SavePath) || strings.HasPrefix(filePath, t.ContentPath) {
			return c.DeleteTorrent(t.Hash, deleteFiles)
		}
	}

	return fmt.Errorf("qbittorrent: no torrent found for path %s", filePath)
}

// PauseTorrent met en pause un torrent par son hash.
func (c *Client) PauseTorrent(hash string) error {
	resp, err := c.httpClient.PostForm(c.baseURL+"/api/v2/torrents/pause", url.Values{
		"hashes": {hash},
	})
	if err != nil {
		return fmt.Errorf("qbittorrent: pause torrent: %w", err)
	}
	defer resp.Body.Close()
	return nil
}
