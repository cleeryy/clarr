package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cleeryy/clarr/internal/radarr"
	"github.com/cleeryy/clarr/internal/sonarr"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ─── Models ───────────────────────────────────────────────────────────

type JellyfinEvent struct {
	Event        string `json:"Event"`
	Title        string `json:"Title"`
	ItemID       string `json:"ItemId"`
	ItemType     string `json:"ItemType"` // "Movie" | "Episode" | "Series"
	SeriesName   string `json:"SeriesName"`
	SeasonNumber int    `json:"SeasonNumber"`
}

// ─── Handler ──────────────────────────────────────────────────────────

type Handler struct {
	secret string
	radarr *radarr.Client
	sonarr *sonarr.Client
	logger *zap.Logger
}

func New(secret string, radarr *radarr.Client, sonarr *sonarr.Client, logger *zap.Logger) *Handler {
	return &Handler{
		secret: secret,
		radarr: radarr,
		sonarr: sonarr,
		logger: logger,
	}
}

// Register enregistre les routes webhook sur le router Gin
func (h *Handler) Register(r *gin.Engine) {
	r.POST("/webhook/jellyfin", h.handleJellyfin)
}

// ─── Routes ───────────────────────────────────────────────────────────

func (h *Handler) handleJellyfin(c *gin.Context) {
	// Vérification de la signature HMAC si un secret est configuré
	if h.secret != "" {
		if err := h.verifySignature(c); err != nil {
			h.logger.Warn("webhook signature invalid", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
	}

	var event JellyfinEvent
	if err := json.NewDecoder(c.Request.Body).Decode(&event); err != nil {
		h.logger.Error("failed to decode jellyfin event", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	h.logger.Info("jellyfin event received",
		zap.String("event", event.Event),
		zap.String("item_type", event.ItemType),
		zap.String("title", event.Title),
	)

	// On ne traite que les suppressions
	if !isDeleteEvent(event.Event) {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	go h.dispatch(event)

	c.JSON(http.StatusOK, gin.H{"status": "processing"})
}

// ─── Dispatch ─────────────────────────────────────────────────────────

func (h *Handler) dispatch(event JellyfinEvent) {
	switch strings.ToLower(event.ItemType) {
	case "movie":
		h.handleMovieDeleted(event)
	case "episode", "series":
		h.handleSeriesDeleted(event)
	default:
		h.logger.Warn("unknown item type",
			zap.String("item_type", event.ItemType),
			zap.String("title", event.Title),
		)
	}
}

func (h *Handler) handleMovieDeleted(event JellyfinEvent) {
	h.logger.Info("processing deleted movie",
		zap.String("title", event.Title),
	)

	// Force rescan Radarr pour détecter hasFile == false
	if err := h.radarr.RescanAll(); err != nil {
		h.logger.Error("radarr rescan failed",
			zap.String("title", event.Title),
			zap.Error(err),
		)
		return
	}

	// Récupère les films sans fichier et les unmonitor
	missing, err := h.radarr.GetMissingMovies()
	if err != nil {
		h.logger.Error("radarr get missing movies failed", zap.Error(err))
		return
	}

	for _, m := range missing {
		if err := h.radarr.UnmonitorMovie(m.ID); err != nil {
			h.logger.Error("radarr unmonitor failed",
				zap.String("title", m.Title),
				zap.Error(err),
			)
			continue
		}
		h.logger.Info("radarr movie unmonitored",
			zap.String("title", m.Title),
			zap.Int("id", m.ID),
		)
	}
}

func (h *Handler) handleSeriesDeleted(event JellyfinEvent) {
	h.logger.Info("processing deleted series/episode",
		zap.String("title", event.Title),
		zap.String("series", event.SeriesName),
	)

	// Force rescan Sonarr
	if err := h.sonarr.RescanAll(); err != nil {
		h.logger.Error("sonarr rescan failed",
			zap.String("title", event.Title),
			zap.Error(err),
		)
		return
	}

	// Récupère les séries vides et les unmonitor
	empty, err := h.sonarr.GetEmptySeries()
	if err != nil {
		h.logger.Error("sonarr get empty series failed", zap.Error(err))
		return
	}

	for _, s := range empty {
		if err := h.sonarr.UnmonitorSeries(s.ID); err != nil {
			h.logger.Error("sonarr unmonitor failed",
				zap.String("title", s.Title),
				zap.Error(err),
			)
			continue
		}
		h.logger.Info("sonarr series unmonitored",
			zap.String("title", s.Title),
			zap.Int("id", s.ID),
		)
	}
}

// ─── Security ─────────────────────────────────────────────────────────

func (h *Handler) verifySignature(c *gin.Context) error {
	signature := c.GetHeader("X-Jellyfin-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Jellyfin-Signature header")
	}

	body, err := c.GetRawData()
	if err != nil {
		return fmt.Errorf("cannot read request body: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}

	// Remettre le body dans le contexte pour le décodage JSON après
	c.Request.Body = http.NoBody
	c.Set("rawBody", body)

	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────

func isDeleteEvent(event string) bool {
	deleteEvents := []string{
		"library.deleted",
		"item.deleted",
		"playback.stop", // Certains plugins envoient ça à la suppression
	}
	for _, e := range deleteEvents {
		if strings.EqualFold(event, e) {
			return true
		}
	}
	return false
}
