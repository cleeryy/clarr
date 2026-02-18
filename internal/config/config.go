package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Jellyfin    JellyfinConfig    `yaml:"jellyfin"`
	Radarr      RadarrConfig      `yaml:"radarr"`
	Sonarr      SonarrConfig      `yaml:"sonarr"`
	Qbittorrent QbittorrentConfig `yaml:"qbittorrent"`
	Cleaner     CleanerConfig     `yaml:"cleaner"`
}

type ServerConfig struct {
	Host string `yaml:"host" env:"CLARR_SERVER_HOST" env-default:"0.0.0.0"`
	Port string `yaml:"port" env:"CLARR_SERVER_PORT" env-default:"8090"`
}

type JellyfinConfig struct {
	WebhookSecret string `yaml:"webhook_secret" env:"CLARR_JELLYFIN_WEBHOOK_SECRET" env-required:"true"`
}

type RadarrConfig struct {
	URL    string `yaml:"url"     env:"CLARR_RADARR_URL"     env-required:"true"`
	APIKey string `yaml:"api_key" env:"CLARR_RADARR_API_KEY" env-required:"true"`
}

type SonarrConfig struct {
	URL    string `yaml:"url"     env:"CLARR_SONARR_URL"     env-required:"true"`
	APIKey string `yaml:"api_key" env:"CLARR_SONARR_API_KEY" env-required:"true"`
}

type QbittorrentConfig struct {
	URL      string `yaml:"url"      env:"CLARR_QBITTORRENT_URL"      env-required:"true"`
	Username string `yaml:"username" env:"CLARR_QBITTORRENT_USERNAME" env-default:"admin"`
	Password string `yaml:"password" env:"CLARR_QBITTORRENT_PASSWORD" env-required:"true"`
}

type CleanerConfig struct {
	DownloadDir string `yaml:"download_dir" env:"CLARR_CLEANER_DOWNLOAD_DIR" env-required:"true"`
	DryRun      bool   `yaml:"dry_run"      env:"CLARR_CLEANER_DRY_RUN"      env-default:"true"`
	Schedule    string `yaml:"schedule"     env:"CLARR_CLEANER_SCHEDULE"     env-default:"0 3 * * *"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
