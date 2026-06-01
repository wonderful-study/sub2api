package service

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrNoUpdateAvailable = infraerrors.Conflict("ALREADY_UP_TO_DATE", "no update available; current version is latest")
)

const (
	updateCacheKey    = "update_check_cache"
	updateCacheTTL    = 1200 // 20 minutes
	defaultGitHubRepo = "Wei-Shaw/sub2api"

	UpdateModeBinary              = "binary"
	UpdateModeGitHubActionsDocker = "github_actions_docker"
	UpdateModeDisabled            = "disabled"

	// Security: allowed download domains for updates
	allowedDownloadHost = "github.com"
	allowedAssetHost    = "objects.githubusercontent.com"

	// Security: max download size (500MB)
	maxDownloadSize = 500 * 1024 * 1024
)

// UpdateCache defines cache operations for update service
type UpdateCache interface {
	GetUpdateInfo(ctx context.Context) (string, error)
	SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error
}

// GitHubReleaseClient 获取 GitHub release 信息的接口
type GitHubReleaseClient interface {
	FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error)
	FetchReleaseByTag(ctx context.Context, repo, tag string) (*GitHubRelease, error)
	DownloadFile(ctx context.Context, url, dest string, maxSize int64) error
	FetchChecksumFile(ctx context.Context, url string) ([]byte, error)
	DispatchWorkflow(ctx context.Context, repo, workflowID, ref, token string, inputs map[string]string) (string, error)
}

type UpdateOptions struct {
	NoticeRepo        string
	ArtifactRepo      string
	Mode              string
	DeployWorkflow    string
	DeployRef         string
	DeployGitHubToken string
}

// UpdateService handles software updates
type UpdateService struct {
	cache          UpdateCache
	githubClient   GitHubReleaseClient
	currentVersion string
	buildType      string // "source" for manual builds, "release" for CI builds
	noticeRepo     string
	artifactRepo   string
	mode           string
	deployWorkflow string
	deployRef      string
	deployToken    string
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, version, buildType string, opts UpdateOptions) *UpdateService {
	noticeRepo := strings.TrimSpace(opts.NoticeRepo)
	if noticeRepo == "" {
		noticeRepo = defaultGitHubRepo
	}
	artifactRepo := strings.TrimSpace(opts.ArtifactRepo)
	if artifactRepo == "" {
		artifactRepo = noticeRepo
	}
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = UpdateModeBinary
	}
	deployWorkflow := strings.TrimSpace(opts.DeployWorkflow)
	if deployWorkflow == "" {
		deployWorkflow = "deploy-production.yml"
	}
	deployRef := strings.TrimSpace(opts.DeployRef)
	if deployRef == "" {
		deployRef = "main"
	}

	return &UpdateService{
		cache:          cache,
		githubClient:   githubClient,
		currentVersion: version,
		buildType:      buildType,
		noticeRepo:     noticeRepo,
		artifactRepo:   artifactRepo,
		mode:           mode,
		deployWorkflow: deployWorkflow,
		deployRef:      deployRef,
		deployToken:    strings.TrimSpace(opts.DeployGitHubToken),
	}
}

// UpdateInfo contains update information
type UpdateInfo struct {
	CurrentVersion        string       `json:"current_version"`
	LatestVersion         string       `json:"latest_version"`
	LatestTag             string       `json:"latest_tag,omitempty"`
	UpstreamLatestVersion string       `json:"upstream_latest_version"`
	HasUpdate             bool         `json:"has_update"`
	CustomReleaseReady    bool         `json:"custom_release_ready"`
	DeployConfigured      bool         `json:"deploy_configured"`
	NoticeRepo            string       `json:"notice_repo"`
	ArtifactRepo          string       `json:"artifact_repo"`
	UpdateMode            string       `json:"update_mode"`
	SyncPRURL             string       `json:"sync_pr_url,omitempty"`
	DeployRunURL          string       `json:"deploy_run_url,omitempty"`
	ReleaseInfo           *ReleaseInfo `json:"release_info,omitempty"`
	Cached                bool         `json:"cached"`
	Warning               string       `json:"warning,omitempty"`
	BuildType             string       `json:"build_type"` // "source" or "release"
}

type SystemUpdateResult struct {
	Message      string `json:"message"`
	NeedRestart  bool   `json:"need_restart"`
	DeployRunURL string `json:"deploy_run_url,omitempty"`
}

// ReleaseInfo contains GitHub release details
type ReleaseInfo struct {
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Assets      []Asset `json:"assets,omitempty"`
}

// Asset represents a release asset
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

// GitHubRelease represents GitHub API response
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for available updates
func (s *UpdateService) CheckUpdate(ctx context.Context, force bool) (*UpdateInfo, error) {
	// Try cache first
	if !force {
		if cached, err := s.getFromCache(ctx); err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fetch from GitHub
	info, err := s.fetchLatestRelease(ctx)
	if err != nil {
		// Return cached on error
		if cached, cacheErr := s.getFromCache(ctx); cacheErr == nil && cached != nil {
			cached.Warning = "Using cached data: " + err.Error()
			return cached, nil
		}
		return &UpdateInfo{
			CurrentVersion:        s.currentVersion,
			LatestVersion:         s.currentVersion,
			UpstreamLatestVersion: s.currentVersion,
			HasUpdate:             false,
			CustomReleaseReady:    false,
			DeployConfigured:      s.deployConfigured(),
			NoticeRepo:            s.noticeRepo,
			ArtifactRepo:          s.artifactRepo,
			UpdateMode:            s.mode,
			SyncPRURL:             s.syncPRURL(),
			Warning:               err.Error(),
			BuildType:             s.buildType,
		}, nil
	}

	// Cache result
	s.saveToCache(ctx, info)
	return info, nil
}

// PerformUpdate downloads and applies the update
// Uses atomic file replacement pattern for safe in-place updates
func (s *UpdateService) PerformUpdate(ctx context.Context) (*SystemUpdateResult, error) {
	info, err := s.CheckUpdate(ctx, true)
	if err != nil {
		return nil, err
	}

	if !info.HasUpdate {
		return nil, ErrNoUpdateAvailable
	}

	switch s.mode {
	case UpdateModeGitHubActionsDocker:
		return s.performGitHubActionsDockerUpdate(ctx, info)
	case UpdateModeDisabled:
		return nil, fmt.Errorf("online update is disabled")
	}

	// Find matching archive and checksum for current platform
	release, err := s.githubClient.FetchReleaseByTag(ctx, s.artifactRepo, info.LatestTag)
	if err != nil {
		return nil, fmt.Errorf("custom release %s is not ready in %s: %w", info.LatestTag, s.artifactRepo, err)
	}
	releaseInfo := convertGitHubRelease(release)
	archiveName := s.getArchiveName()
	var downloadURL string
	var checksumURL string

	for _, asset := range releaseInfo.Assets {
		if strings.Contains(asset.Name, archiveName) && !strings.HasSuffix(asset.Name, ".txt") {
			downloadURL = asset.DownloadURL
		}
		if asset.Name == "checksums.txt" {
			checksumURL = asset.DownloadURL
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no compatible release found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// SECURITY: Validate download URL is from trusted domain
	if err := validateDownloadURL(downloadURL); err != nil {
		return nil, fmt.Errorf("invalid download URL: %w", err)
	}
	if checksumURL != "" {
		if err := validateDownloadURL(checksumURL); err != nil {
			return nil, fmt.Errorf("invalid checksum URL: %w", err)
		}
	}

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	exeDir := filepath.Dir(exePath)

	// Create temp directory in the SAME directory as executable
	// This ensures os.Rename is atomic (same filesystem)
	tempDir, err := os.MkdirTemp(exeDir, ".sub2api-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download archive
	archivePath := filepath.Join(tempDir, filepath.Base(downloadURL))
	if err := s.downloadFile(ctx, downloadURL, archivePath); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum if available
	if checksumURL != "" {
		if err := s.verifyChecksum(ctx, archivePath, checksumURL); err != nil {
			return nil, fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract binary from archive
	newBinaryPath := filepath.Join(tempDir, "sub2api")
	if err := s.extractBinary(archivePath, newBinaryPath); err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Set executable permission before replacement
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return nil, fmt.Errorf("chmod failed: %w", err)
	}

	// Atomic replacement using rename pattern:
	// 1. Rename current -> backup (atomic on Unix)
	// 2. Rename new -> current (atomic on Unix, same filesystem)
	// If step 2 fails, restore backup
	backupPath := exePath + ".backup"

	// Remove old backup if exists
	_ = os.Remove(backupPath)

	// Step 1: Move current binary to backup
	if err := os.Rename(exePath, backupPath); err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}

	// Step 2: Move new binary to target location (atomic, same filesystem)
	if err := os.Rename(newBinaryPath, exePath); err != nil {
		// Restore backup on failure
		if restoreErr := os.Rename(backupPath, exePath); restoreErr != nil {
			return nil, fmt.Errorf("replace failed and restore failed: %w (restore error: %v)", err, restoreErr)
		}
		return nil, fmt.Errorf("replace failed (restored backup): %w", err)
	}

	// Success - backup file is kept for rollback capability
	// It will be cleaned up on next successful update
	return &SystemUpdateResult{
		Message:     "Update completed. Please restart the service.",
		NeedRestart: true,
	}, nil
}

// Rollback restores the previous version
func (s *UpdateService) Rollback() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	backupFile := exePath + ".backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	// Replace current with backup
	if err := os.Rename(backupFile, exePath); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	return nil
}

func (s *UpdateService) fetchLatestRelease(ctx context.Context) (*UpdateInfo, error) {
	release, err := s.githubClient.FetchLatestRelease(ctx, s.noticeRepo)
	if err != nil {
		return nil, err
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	releaseInfo := convertGitHubRelease(release)
	customReleaseReady := true
	if s.artifactRepo != s.noticeRepo {
		_, err = s.githubClient.FetchReleaseByTag(ctx, s.artifactRepo, release.TagName)
		customReleaseReady = err == nil
	}

	return &UpdateInfo{
		CurrentVersion:        s.currentVersion,
		LatestVersion:         latestVersion,
		LatestTag:             release.TagName,
		UpstreamLatestVersion: latestVersion,
		HasUpdate:             compareVersions(s.currentVersion, latestVersion) < 0,
		CustomReleaseReady:    customReleaseReady,
		DeployConfigured:      s.deployConfigured(),
		NoticeRepo:            s.noticeRepo,
		ArtifactRepo:          s.artifactRepo,
		UpdateMode:            s.mode,
		SyncPRURL:             s.syncPRURL(),
		ReleaseInfo:           releaseInfo,
		Cached:                false,
		BuildType:             s.buildType,
	}, nil
}

func (s *UpdateService) performGitHubActionsDockerUpdate(ctx context.Context, info *UpdateInfo) (*SystemUpdateResult, error) {
	if !info.CustomReleaseReady {
		return nil, fmt.Errorf("custom release %s is not ready in %s; merge upstream and publish your fork release first", info.LatestTag, s.artifactRepo)
	}
	if strings.TrimSpace(s.deployToken) == "" {
		return nil, fmt.Errorf("UPDATE_DEPLOY_GITHUB_TOKEN is required for Docker online updates")
	}

	workflowURL, err := s.githubClient.DispatchWorkflow(ctx, s.artifactRepo, s.deployWorkflow, s.deployRef, s.deployToken, map[string]string{
		"image_tag": info.LatestVersion,
		"version":   info.LatestTag,
	})
	if err != nil {
		return nil, err
	}

	info.DeployRunURL = workflowURL
	return &SystemUpdateResult{
		Message:      "Deployment workflow dispatched.",
		NeedRestart:  false,
		DeployRunURL: workflowURL,
	}, nil
}

func convertGitHubRelease(release *GitHubRelease) *ReleaseInfo {
	if release == nil {
		return nil
	}
	assets := make([]Asset, len(release.Assets))
	for i, a := range release.Assets {
		assets[i] = Asset{
			Name:        a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
		}
	}
	return &ReleaseInfo{
		Name:        release.Name,
		Body:        release.Body,
		PublishedAt: release.PublishedAt,
		HTMLURL:     release.HTMLURL,
		Assets:      assets,
	}
}

func (s *UpdateService) deployConfigured() bool {
	if s.mode != UpdateModeGitHubActionsDocker {
		return true
	}
	return strings.TrimSpace(s.deployToken) != "" && strings.TrimSpace(s.deployWorkflow) != ""
}

func (s *UpdateService) syncPRURL() string {
	if strings.TrimSpace(s.artifactRepo) == "" {
		return ""
	}
	return "https://github.com/" + strings.Trim(strings.TrimSpace(s.artifactRepo), "/") + "/pulls"
}

func (s *UpdateService) downloadFile(ctx context.Context, downloadURL, dest string) error {
	return s.githubClient.DownloadFile(ctx, downloadURL, dest, maxDownloadSize)
}

func (s *UpdateService) getArchiveName() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf("%s_%s", osName, arch)
}

// validateDownloadURL checks if the URL is from an allowed domain
// SECURITY: This prevents SSRF and ensures downloads only come from trusted GitHub domains
func validateDownloadURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Must be HTTPS
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}

	// Check against allowed hosts
	host := parsedURL.Host
	// GitHub release URLs can be from github.com or objects.githubusercontent.com
	if host != allowedDownloadHost &&
		!strings.HasSuffix(host, "."+allowedDownloadHost) &&
		host != allowedAssetHost &&
		!strings.HasSuffix(host, "."+allowedAssetHost) {
		return fmt.Errorf("download from untrusted host: %s", host)
	}

	return nil
}

func (s *UpdateService) verifyChecksum(ctx context.Context, filePath, checksumURL string) error {
	// Download checksums file
	checksumData, err := s.githubClient.FetchChecksumFile(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}

	// Calculate file hash
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	// Find expected hash in checksums file
	fileName := filepath.Base(filePath)
	scanner := bufio.NewScanner(strings.NewReader(string(checksumData)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == fileName {
			if parts[0] == actualHash {
				return nil
			}
			return fmt.Errorf("checksum mismatch: expected %s, got %s", parts[0], actualHash)
		}
	}

	return fmt.Errorf("checksum not found for %s", fileName)
}

func (s *UpdateService) extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f

	// Handle gzip compression
	if strings.HasSuffix(archivePath, ".gz") || strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gzr.Close() }()
		reader = gzr
	}

	// Handle tar archive
	if strings.Contains(archivePath, ".tar") {
		tr := tar.NewReader(reader)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			// SECURITY: Prevent Zip Slip / Path Traversal attack
			// Only allow files with safe base names, no directory traversal
			baseName := filepath.Base(hdr.Name)

			// Check for path traversal attempts
			if strings.Contains(hdr.Name, "..") {
				return fmt.Errorf("path traversal attempt detected: %s", hdr.Name)
			}

			// Validate the entry is a regular file
			if hdr.Typeflag != tar.TypeReg {
				continue // Skip directories and special files
			}

			// Only extract the specific binary we need
			if baseName == "sub2api" || baseName == "sub2api.exe" {
				// Additional security: limit file size (max 500MB)
				const maxBinarySize = 500 * 1024 * 1024
				if hdr.Size > maxBinarySize {
					return fmt.Errorf("binary too large: %d bytes (max %d)", hdr.Size, maxBinarySize)
				}

				out, err := os.Create(destPath)
				if err != nil {
					return err
				}

				// Use LimitReader to prevent decompression bombs
				limited := io.LimitReader(tr, maxBinarySize)
				if _, err := io.Copy(out, limited); err != nil {
					_ = out.Close()
					return err
				}
				if err := out.Close(); err != nil {
					return err
				}
				return nil
			}
		}
		return fmt.Errorf("binary not found in archive")
	}

	// Direct copy for non-tar files (with size limit)
	const maxBinarySize = 500 * 1024 * 1024
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	limited := io.LimitReader(reader, maxBinarySize)
	if _, err := io.Copy(out, limited); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func (s *UpdateService) getFromCache(ctx context.Context) (*UpdateInfo, error) {
	data, err := s.cache.GetUpdateInfo(ctx)
	if err != nil {
		return nil, err
	}

	var cached struct {
		Latest             string       `json:"latest"`
		LatestTag          string       `json:"latest_tag"`
		CustomReleaseReady bool         `json:"custom_release_ready"`
		NoticeRepo         string       `json:"notice_repo"`
		ArtifactRepo       string       `json:"artifact_repo"`
		UpdateMode         string       `json:"update_mode"`
		ReleaseInfo        *ReleaseInfo `json:"release_info"`
		Timestamp          int64        `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return nil, err
	}

	if time.Now().Unix()-cached.Timestamp > updateCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	if strings.TrimSpace(cached.LatestTag) == "" && strings.TrimSpace(cached.Latest) != "" {
		cached.LatestTag = "v" + strings.TrimPrefix(cached.Latest, "v")
	}
	noticeRepo := nonEmpty(cached.NoticeRepo, s.noticeRepo)
	artifactRepo := nonEmpty(cached.ArtifactRepo, s.artifactRepo)
	customReleaseReady := cached.CustomReleaseReady
	if artifactRepo == noticeRepo {
		customReleaseReady = true
	}

	return &UpdateInfo{
		CurrentVersion:        s.currentVersion,
		LatestVersion:         cached.Latest,
		LatestTag:             cached.LatestTag,
		UpstreamLatestVersion: cached.Latest,
		HasUpdate:             compareVersions(s.currentVersion, cached.Latest) < 0,
		CustomReleaseReady:    customReleaseReady,
		DeployConfigured:      s.deployConfigured(),
		NoticeRepo:            noticeRepo,
		ArtifactRepo:          artifactRepo,
		UpdateMode:            nonEmpty(cached.UpdateMode, s.mode),
		SyncPRURL:             s.syncPRURL(),
		ReleaseInfo:           cached.ReleaseInfo,
		Cached:                true,
		BuildType:             s.buildType,
	}, nil
}

func (s *UpdateService) saveToCache(ctx context.Context, info *UpdateInfo) {
	cacheData := struct {
		Latest             string       `json:"latest"`
		LatestTag          string       `json:"latest_tag"`
		CustomReleaseReady bool         `json:"custom_release_ready"`
		NoticeRepo         string       `json:"notice_repo"`
		ArtifactRepo       string       `json:"artifact_repo"`
		UpdateMode         string       `json:"update_mode"`
		ReleaseInfo        *ReleaseInfo `json:"release_info"`
		Timestamp          int64        `json:"timestamp"`
	}{
		Latest:             info.LatestVersion,
		LatestTag:          info.LatestTag,
		CustomReleaseReady: info.CustomReleaseReady,
		NoticeRepo:         info.NoticeRepo,
		ArtifactRepo:       info.ArtifactRepo,
		UpdateMode:         info.UpdateMode,
		ReleaseInfo:        info.ReleaseInfo,
		Timestamp:          time.Now().Unix(),
	}

	data, _ := json.Marshal(cacheData)
	_ = s.cache.SetUpdateInfo(ctx, string(data), time.Duration(updateCacheTTL)*time.Second)
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// compareVersions compares two semantic versions
func compareVersions(current, latest string) int {
	currentParts := parseComparableVersion(current)
	latestParts := parseComparableVersion(latest)

	for i := 0; i < 3; i++ {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func parseComparableVersion(v string) [3]int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if suffixIndex := strings.IndexAny(v, "-+"); suffixIndex >= 0 {
		v = v[:suffixIndex]
	}
	parts := strings.Split(v, ".")
	result := [3]int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		if parsed, err := strconv.Atoi(parts[i]); err == nil {
			result[i] = parsed
		}
	}
	return result
}
