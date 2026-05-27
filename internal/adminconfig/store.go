package adminconfig

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type SiteConfig struct {
	SiteName           string     `json:"site_name"`
	LogoURL            string     `json:"logo_url"`
	JWTSecret          string     `json:"jwt_secret,omitempty"`
	JWTSecretRotatedAt *time.Time `json:"jwt_secret_rotated_at,omitempty"`
	SMTPHost           string     `json:"smtp_host"`
	SMTPPort           int        `json:"smtp_port"`
	SMTPUser           string     `json:"smtp_user"`
	SMTPPass           string     `json:"smtp_pass,omitempty"`
	SMTPFrom           string     `json:"smtp_from"`
	AllowRegistration  bool       `json:"allow_registration"`
	DefaultUserRole    string     `json:"default_user_role"`
	DefaultUserQuota   int64      `json:"default_user_quota"`
	UpdatedAt          time.Time  `json:"updated_at"`
	UpdatedBy          string     `json:"updated_by"`
}

type ConfigSnapshot struct {
	ID             int64      `json:"id"`
	Version        string     `json:"version"`
	Status         string     `json:"status"`
	ConfigSnapshot string     `json:"config_snapshot"`
	Notes          string     `json:"notes"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	RolledBackAt   *time.Time `json:"rolled_back_at,omitempty"`
}

type Store struct {
	mu        sync.RWMutex
	site      SiteConfig
	snapshots []ConfigSnapshot
	nextID    int64
}

func NewStore() *Store {
	return &Store{
		site: SiteConfig{
			SiteName:          "LLM Gateway",
			AllowRegistration: true,
			DefaultUserRole:   "user",
			DefaultUserQuota:  1000000,
			SMTPPort:          587,
			UpdatedAt:         time.Now().UTC(),
		},
		nextID: 1,
	}
}

func (s *Store) GetSiteConfig() SiteConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.site
}

func (s *Store) UpdateSiteConfig(cfg SiteConfig, updatedBy string) SiteConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.SiteName != "" {
		s.site.SiteName = cfg.SiteName
	}
	s.site.LogoURL = cfg.LogoURL
	if cfg.SMTPHost != "" {
		s.site.SMTPHost = cfg.SMTPHost
	}
	if cfg.SMTPPort > 0 {
		s.site.SMTPPort = cfg.SMTPPort
	}
	if cfg.SMTPUser != "" {
		s.site.SMTPUser = cfg.SMTPUser
	}
	if cfg.SMTPPass != "" {
		s.site.SMTPPass = cfg.SMTPPass
	}
	if cfg.SMTPFrom != "" {
		s.site.SMTPFrom = cfg.SMTPFrom
	}
	s.site.AllowRegistration = cfg.AllowRegistration
	if cfg.DefaultUserRole != "" {
		s.site.DefaultUserRole = cfg.DefaultUserRole
	}
	if cfg.DefaultUserQuota > 0 {
		s.site.DefaultUserQuota = cfg.DefaultUserQuota
	}
	s.site.UpdatedAt = time.Now().UTC()
	s.site.UpdatedBy = updatedBy

	cloned := s.site
	return cloned
}

func (s *Store) RotateJWTSecret(updatedBy string) (string, *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	newSecret := hex.EncodeToString(bytes)

	s.site.JWTSecret = newSecret
	now := time.Now().UTC()
	s.site.JWTSecretRotatedAt = &now
	s.site.UpdatedAt = now
	s.site.UpdatedBy = updatedBy

	return newSecret, &now
}

func (s *Store) ListSnapshots() []ConfigSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ConfigSnapshot, len(s.snapshots))
	copy(result, s.snapshots)
	return result
}

func (s *Store) GetSnapshot(id int64) (ConfigSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, snap := range s.snapshots {
		if snap.ID == id {
			return snap, true
		}
	}
	return ConfigSnapshot{}, false
}

func (s *Store) CreateSnapshot(version, configSnapshot, notes, createdBy string) ConfigSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := ConfigSnapshot{
		ID:             s.nextID,
		Version:        version,
		Status:         "draft",
		ConfigSnapshot: configSnapshot,
		Notes:          notes,
		CreatedBy:      createdBy,
		CreatedAt:      time.Now().UTC(),
	}
	s.nextID++
	s.snapshots = append(s.snapshots, snap)
	return snap
}

func (s *Store) PublishSnapshot(id int64) (ConfigSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, snap := range s.snapshots {
		if snap.ID == id {
			if snap.Status != "draft" {
				return ConfigSnapshot{}, fmt.Errorf("snapshot %d is not in draft status", id)
			}
			now := time.Now().UTC()
			s.snapshots[i].Status = "published"
			s.snapshots[i].PublishedAt = &now
			return s.snapshots[i], nil
		}
	}
	return ConfigSnapshot{}, fmt.Errorf("snapshot %d not found", id)
}

func (s *Store) RollbackSnapshot(id int64) (ConfigSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, snap := range s.snapshots {
		if snap.ID == id {
			if snap.Status != "published" {
				return ConfigSnapshot{}, fmt.Errorf("snapshot %d is not in published status", id)
			}
			now := time.Now().UTC()
			s.snapshots[i].Status = "rolled_back"
			s.snapshots[i].RolledBackAt = &now
			return s.snapshots[i], nil
		}
	}
	return ConfigSnapshot{}, fmt.Errorf("snapshot %d not found", id)
}

func (s *Store) ExportSnapshots() []ConfigSnapshot {
	return s.ListSnapshots()
}

func (s *Store) ImportSnapshots(snapshots []ConfigSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range snapshots {
		snapshots[i].ID = s.nextID
		s.nextID++
	}
	s.snapshots = append(s.snapshots, snapshots...)
}
