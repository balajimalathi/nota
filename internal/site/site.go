// Package site manages per-site configuration, rigid folder layout, and page/post creation.
package site

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nota/internal/vault"
)

// SiteKind represents the site type.
type SiteKind string

const (
	KindFrameworkOnly SiteKind = "framework_only"
	KindWordPressCMS  SiteKind = "wordpress_cms"
)

// Framework is the optional framework for framework_only sites.
type Framework string

const (
	FrameworkAstro Framework = "astro"
	FrameworkNext  Framework = "next"
	FrameworkOther Framework = "other"
)

// Site is the per-site configuration stored in .nota/site.json.
type Site struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	PublicSiteURL string    `json:"publicSiteUrl"`
	Kind          SiteKind  `json:"kind"`
	Framework     Framework `json:"framework,omitempty"`
	WPAPIURL      string    `json:"wpApiUrl,omitempty"`
}

// Rigid folder layout constants.
const (
	DirPages        = "pages"
	DirPosts        = "posts"
	DirStatic       = "static"
	DirKeywords     = "keywords"
	DirAudits       = "keywords/audits"
	DirAssets       = "assets"
	DirAssetsImages = "assets/images"
	DirNotes        = "notes"
	DirArchive      = "_archive"
	DirNota         = ".nota"
)

// AllDirs is the complete list of rigid directories to create.
var AllDirs = []string{
	DirPages,
	DirPosts,
	DirStatic,
	DirKeywords,
	DirAudits,
	DirAssets,
	DirAssetsImages,
	DirNotes,
	DirArchive,
	DirNota,
}

// SeedFiles maps seed file paths to their initial content.
var SeedFiles = map[string]string{
	"keywords/index.yaml":    "# Keyword inventory\n",
	".nota/inventory.yaml":   "# Site inventory\n",
}

// SiteFileName is the site config filename.
const SiteFileName = "site.json"

// SiteDir is the .nota directory name.
const SiteDir = ".nota"

// SiteConfigPath returns the path to site.json for a given site root.
func SiteConfigPath(siteRoot string) string {
	return filepath.Join(siteRoot, SiteDir, SiteFileName)
}

// LoadSite reads and parses site.json from the site root.
func LoadSite(siteRoot string) (*Site, error) {
	path := SiteConfigPath(siteRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Site
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveSite writes site.json.
func SaveSite(siteRoot string, s *Site) error {
	notaDir := filepath.Join(siteRoot, SiteDir)
	if err := os.MkdirAll(notaDir, 0o755); err != nil {
		return err
	}
	path := SiteConfigPath(siteRoot)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// EnsureLayout creates all rigid directories and seed files under siteRoot.
func EnsureLayout(v *vault.Vault, siteRoot string) error {
	for _, dir := range AllDirs {
		if err := v.Mkdir(dir); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	for relPath, content := range SeedFiles {
		if err := v.Write(relPath, []byte(content)); err != nil {
			return fmt.Errorf("seed %s: %w", relPath, err)
		}
	}
	return nil
}

// DefaultURLPath computes the default public URL path from a vault-relative path.
func DefaultURLPath(relPath string) (string, error) {
	relPath = filepath.ToSlash(filepath.Clean(relPath))

	var prefix string
	switch {
	case strings.HasPrefix(relPath, DirPages+"/"):
		prefix = DirPages + "/"
	case strings.HasPrefix(relPath, DirStatic+"/"):
		prefix = DirStatic + "/"
	case strings.HasPrefix(relPath, DirPosts+"/"):
		prefix = DirPosts + "/"
	default:
		return "", errors.New("path not under pages/, posts/, or static/")
	}

	rest := strings.TrimPrefix(relPath, prefix)
	rest = strings.TrimSuffix(rest, ".md")

	if strings.HasSuffix(relPath, "/index.md") || rest == "index" || strings.HasSuffix(rest, "/index") {
		rest = strings.TrimSuffix(rest, "/index")
		rest = strings.TrimSuffix(rest, "index")
		if rest == "" {
			return "/", nil
		}
		return "/" + strings.Trim(rest, "/"), nil
	}

	return "/" + strings.Trim(rest, "/"), nil
}

// SourceType represents the content source.
type SourceType string

const (
	SourceWordPress SourceType = "wordpress"
	SourceStatic    SourceType = "static"
)

// Role represents the content role.
type Role string

const (
	RolePage    Role = "page"
	RolePillar  Role = "pillar"
	RoleCluster Role = "cluster"
)

// PageFrontmatter is the frontmatter for pages/ and static/.
type PageFrontmatter struct {
	Title             string       `yaml:"title"`
	URLPath           string       `yaml:"url_path"`
	Source            SourceType  `yaml:"source"`
	Role              Role        `yaml:"role"`
	WPID              int          `yaml:"wp_id,omitempty"`
	PrimaryKeyword   string       `yaml:"primary_keyword,omitempty"`
	SecondaryKeywords []string     `yaml:"secondary_keywords,omitempty"`
	Status            string       `yaml:"status,omitempty"`
}

// PostFrontmatter is the frontmatter for posts/ (pillar or cluster).
type PostFrontmatter struct {
	Title             string       `yaml:"title"`
	URLPath           string       `yaml:"url_path"`
	Source            SourceType  `yaml:"source"`
	Role              Role        `yaml:"role"`
	Pillar           string       `yaml:"pillar,omitempty"`
	WPID             int          `yaml:"wp_id,omitempty"`
	PrimaryKeyword   string       `yaml:"primary_keyword,omitempty"`
	SecondaryKeywords []string     `yaml:"secondary_keywords,omitempty"`
	Status            string       `yaml:"status,omitempty"`
}

// ValidatePostFolder checks that a topic folder under posts/ has index.md (pillar).
func ValidatePostFolder(v *vault.Vault, topicSlug string) (string, error) {
	indexPath := filepath.ToSlash(filepath.Join(DirPosts, topicSlug, "index.md"))

	f, err := v.Stat(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("topic folder %q missing pillar index.md: create it or run 'New topic folder'", topicSlug)
		}
		return "", err
	}
	if f.Dir {
		return "", fmt.Errorf("expected file at %s, found directory", indexPath)
	}
	return indexPath, nil
}

// EnsurePillar creates the pillar index.md if it doesn't exist.
func EnsurePillar(v *vault.Vault, topicSlug string, title string) (string, error) {
	indexPath := filepath.ToSlash(filepath.Join(DirPosts, topicSlug, "index.md"))

	_, err := v.Stat(indexPath)
	if err == nil {
		return indexPath, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	if err := v.Mkdir(filepath.Join(DirPosts, topicSlug)); err != nil {
		return "", err
	}

	urlPath, _ := DefaultURLPath(indexPath)
	fm := PostFrontmatter{
		Title:   title,
		URLPath: urlPath,
		Source:  SourceStatic,
		Role:    RolePillar,
		Status:  "draft",
	}
	content := BuildPostFrontmatter(fm, "")
	if err := v.Write(indexPath, []byte(content)); err != nil {
		return "", err
	}
	return indexPath, nil
}

// CreateClusterPost creates a cluster post under a topic folder.
func CreateClusterPost(v *vault.Vault, topicSlug, slug, title string) (string, error) {
	if _, err := ValidatePostFolder(v, topicSlug); err != nil {
		return "", err
	}

	clusterPath := filepath.ToSlash(filepath.Join(DirPosts, topicSlug, slug+".md"))

	_, err := v.Stat(clusterPath)
	if err == nil {
		return "", fmt.Errorf("cluster post %q already exists", slug)
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	urlPath, _ := DefaultURLPath(clusterPath)
	fm := PostFrontmatter{
		Title:   title,
		URLPath: urlPath,
		Source:  SourceStatic,
		Role:    RoleCluster,
		Pillar:  topicSlug,
		Status:  "draft",
	}
	content := BuildPostFrontmatter(fm, "")
	if err := v.Write(clusterPath, []byte(content)); err != nil {
		return "", err
	}
	return clusterPath, nil
}

// CreatePage creates a page under pages/ or static/.
func CreatePage(v *vault.Vault, baseDir, relPath, title string, source SourceType) (string, error) {
	if baseDir != DirPages && baseDir != DirStatic {
		return "", errors.New("baseDir must be pages or static")
	}

	fullPath := filepath.ToSlash(filepath.Join(baseDir, relPath))

	if _, err := v.Stat(fullPath); err == nil {
		return "", fmt.Errorf("page %q already exists", fullPath)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if baseDir == DirPages {
		if err := v.Mkdir(filepath.Join(DirPages, filepath.Dir(fullPath))); err != nil {
			return "", err
		}
	}

	urlPath, _ := DefaultURLPath(fullPath)
	fm := PageFrontmatter{
		Title:   title,
		URLPath: urlPath,
		Source:  source,
		Role:    RolePage,
		Status:  "draft",
	}
	content := BuildPageFrontmatter(fm, "")
	if err := v.Write(fullPath, []byte(content)); err != nil {
		return "", err
	}
	return fullPath, nil
}

// BuildPageFrontmatter creates page content with frontmatter.
func BuildPageFrontmatter(fm PageFrontmatter, body string) string {
	secondary := "[]"
	if len(fm.SecondaryKeywords) > 0 {
		secondary = fmt.Sprintf("[%s]", strings.Join(fm.SecondaryKeywords, ", "))
	}
	wpID := ""
	if fm.WPID > 0 {
		wpID = fmt.Sprintf("wp_id: %d\n", fm.WPID)
	}
	primary := ""
	if fm.PrimaryKeyword != "" {
		primary = fmt.Sprintf("primary_keyword: %s\n", fm.PrimaryKeyword)
	}
	return fmt.Sprintf("---\ntitle: %q\nurl_path: %q\nsource: %s\nrole: %s\n%s%s%sstatus: %q\n---\n\n%s", fm.Title, fm.URLPath, fm.Source, fm.Role, primary, wpID, secondary, fm.Status, body)
}

// BuildPostFrontmatter creates post content with frontmatter.
func BuildPostFrontmatter(fm PostFrontmatter, body string) string {
	primary := ""
	if fm.PrimaryKeyword != "" {
		primary = fmt.Sprintf("primary_keyword: %s\n", fm.PrimaryKeyword)
	}
	secondary := "[]"
	if len(fm.SecondaryKeywords) > 0 {
		secondary = fmt.Sprintf("[%s]", strings.Join(fm.SecondaryKeywords, ", "))
	}
	wpID := ""
	if fm.WPID > 0 {
		wpID = fmt.Sprintf("wp_id: %d\n", fm.WPID)
	}
	pillar := ""
	if fm.Pillar != "" {
		pillar = fmt.Sprintf("pillar: %s\n", fm.Pillar)
	}
	return fmt.Sprintf("---\ntitle: %q\nurl_path: %q\nsource: %s\nrole: %s\n%s%s%s%sstatus: %q\n---\n\n%s", fm.Title, fm.URLPath, fm.Source, fm.Role, primary, wpID, secondary, pillar, fm.Status, body)
}