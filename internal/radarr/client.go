package radarr

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

// ─── Models ─────────────────────────────────────────────────────────

type Movie struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	HasFile     bool   `json:"hasFile"`
	Monitored   bool   `json:"monitored"`
	TmdbID      int    `json:"tmdbId"`
	Path        string `json:"path"`
}

type Command struct {
	Name    string `json:"name"`
	MovieID int    `json:"movieId,omitempty"`
}

// ─── HTTP Helper ─────────────────────────────────────────────────────

func (c *Client) do(method, endpoint string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("radarr: encode body: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("radarr: build request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("radarr: request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("radarr: unexpected status %d on %s %s", resp.StatusCode, method, endpoint)
	}

	return resp, nil
}

// ─── Methods ─────────────────────────────────────────────────────────

// GetAllMovies retourne tous les films de la bibliothèque Radarr
func (c *Client) GetAllMovies() ([]Movie, error) {
	resp, err := c.do(http.MethodGet, "/api/v3/movie", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var movies []Movie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return nil, fmt.Errorf("radarr: decode movies: %w", err)
	}
	return movies, nil
}

// GetMissingMovies retourne les films où hasFile == false
func (c *Client) GetMissingMovies() ([]Movie, error) {
	movies, err := c.GetAllMovies()
	if err != nil {
		return nil, err
	}

	var missing []Movie
	for _, m := range movies {
		if !m.HasFile {
			missing = append(missing, m)
		}
	}
	return missing, nil
}

// UnmonitorMovie désactive le monitoring d'un film par son ID
func (c *Client) UnmonitorMovie(movieID int) error {
	movie, err := c.getMovieByID(movieID)
	if err != nil {
		return err
	}

	movie.Monitored = false
	_, err = c.do(http.MethodPut, fmt.Sprintf("/api/v3/movie/%d", movieID), movie)
	return err
}

// DeleteMovie supprime un film de Radarr (et optionnellement ses fichiers)
func (c *Client) DeleteMovie(movieID int, deleteFiles bool) error {
	endpoint := fmt.Sprintf("/api/v3/movie/%d?deleteFiles=%v&addImportExclusion=false", movieID, deleteFiles)
	_, err := c.do(http.MethodDelete, endpoint, nil)
	return err
}

// RescanMovie force un rescan du fichier d'un film
func (c *Client) RescanMovie(movieID int) error {
	_, err := c.do(http.MethodPost, "/api/v3/command", Command{
		Name:    "RescanMovie",
		MovieID: movieID,
	})
	return err
}

// RescanAll force un rescan complet de toute la bibliothèque
func (c *Client) RescanAll() error {
	_, err := c.do(http.MethodPost, "/api/v3/command", Command{
		Name: "RescanMovie",
	})
	return err
}

// ─── Private ─────────────────────────────────────────────────────────

func (c *Client) getMovieByID(movieID int) (*Movie, error) {
	resp, err := c.do(http.MethodGet, fmt.Sprintf("/api/v3/movie/%d", movieID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var movie Movie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("radarr: decode movie: %w", err)
	}
	return &movie, nil
}
