package sonarr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ─── Models ──────────────────────────────────────────────────────────

type Series struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Monitored  bool   `json:"monitored"`
	Path       string `json:"path"`
	Statistics struct {
		EpisodeFileCount int `json:"episodeFileCount"`
	} `json:"statistics"`
}

type Episode struct {
	ID            int    `json:"id"`
	SeriesID      int    `json:"seriesId"`
	Title         string `json:"title"`
	Monitored     bool   `json:"monitored"`
	HasFile       bool   `json:"hasFile"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
}

type Command struct {
	Name     string `json:"name"`
	SeriesID int    `json:"seriesId,omitempty"`
}

// ─── HTTP Helper ──────────────────────────────────────────────────────

func (c *Client) do(method, endpoint string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("sonarr: encode body: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("sonarr: build request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sonarr: request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sonarr: unexpected status %d on %s %s", resp.StatusCode, method, endpoint)
	}

	return resp, nil
}

// ─── Series Methods ───────────────────────────────────────────────────

// GetAllSeries retourne toutes les séries
func (c *Client) GetAllSeries() ([]Series, error) {
	resp, err := c.do(http.MethodGet, "/api/v3/series", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var series []Series
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("sonarr: decode series: %w", err)
	}
	return series, nil
}

// GetEmptySeries retourne les séries sans aucun fichier sur le disque
func (c *Client) GetEmptySeries() ([]Series, error) {
	series, err := c.GetAllSeries()
	if err != nil {
		return nil, err
	}

	var empty []Series
	for _, s := range series {
		if s.Statistics.EpisodeFileCount == 0 {
			empty = append(empty, s)
		}
	}
	return empty, nil
}

// UnmonitorSeries désactive le monitoring d'une série
func (c *Client) UnmonitorSeries(seriesID int) error {
	series, err := c.getSeriesByID(seriesID)
	if err != nil {
		return err
	}

	series.Monitored = false
	_, err = c.do(http.MethodPut, fmt.Sprintf("/api/v3/series/%d", seriesID), series)
	return err
}

// DeleteSeries supprime une série de Sonarr
func (c *Client) DeleteSeries(seriesID int, deleteFiles bool) error {
	endpoint := fmt.Sprintf("/api/v3/series/%d?deleteFiles=%v&addImportExclusion=false", seriesID, deleteFiles)
	_, err := c.do(http.MethodDelete, endpoint, nil)
	return err
}

// RescanSeries force un rescan d'une série spécifique
func (c *Client) RescanSeries(seriesID int) error {
	_, err := c.do(http.MethodPost, "/api/v3/command", Command{
		Name:     "RescanSeries",
		SeriesID: seriesID,
	})
	return err
}

// RescanAll force un rescan complet de toutes les séries
func (c *Client) RescanAll() error {
	_, err := c.do(http.MethodPost, "/api/v3/command", Command{
		Name: "RescanSeries",
	})
	return err
}

// ─── Episode Methods ──────────────────────────────────────────────────

// GetMissingEpisodes retourne les épisodes sans fichier d'une série
func (c *Client) GetMissingEpisodes(seriesID int) ([]Episode, error) {
	resp, err := c.do(http.MethodGet, fmt.Sprintf("/api/v3/episode?seriesId=%d", seriesID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var episodes []Episode
	if err := json.NewDecoder(resp.Body).Decode(&episodes); err != nil {
		return nil, fmt.Errorf("sonarr: decode episodes: %w", err)
	}

	var missing []Episode
	for _, e := range episodes {
		if !e.HasFile {
			missing = append(missing, e)
		}
	}
	return missing, nil
}

// ─── Private ──────────────────────────────────────────────────────────

func (c *Client) getSeriesByID(seriesID int) (*Series, error) {
	resp, err := c.do(http.MethodGet, fmt.Sprintf("/api/v3/series/%d", seriesID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var series Series
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("sonarr: decode series: %w", err)
	}
	return &series, nil
}
