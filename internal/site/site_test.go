package site_test

import (
	"os"
	"path/filepath"
	"testing"

	"nota/internal/site"
	"nota/internal/vault"
)

func createTestVault(t *testing.T) *vault.Vault {
	// Create a temp vault rooted at /tmp/test-nota-vault
	tmpDir, err := os.MkdirTemp("", "nota-test-*")
	if err != nil {
		t.Fatal(err)
	}
	v, err := vault.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	// Create all rigid directories
	for _, d := range []string{
		"pages", "posts", "static", "keywords", "keywords/audits",
		"assets", "assets/images", "notes", "_archive", ".nota",
	} {
		if err := v.Mkdir(d); err != nil {
			t.Fatal(err)
		}
	}
	return v
}

func TestDefaultURLPath(t *testing.T) {
	tests := []struct {
		relPath string
		want    string
	}{
		{"pages/use-cases/certificates.md", "/use-cases/certificates"},
		{"pages/use-cases/index.md", "/use-cases"},
		{"static/tools/free-og-preview/index.md", "/tools/free-og-preview"},
		{"posts/credentials/index.md", "/credentials"},
		{"posts/credentials/guide.md", "/credentials/guide"},
		{"pages/legal/privacy.md", "/legal/privacy"},
	}

	for _, tt := range tests {
		got, err := site.DefaultURLPath(tt.relPath)
		if err != nil {
			t.Errorf("DefaultURLPath(%q) error: %v", tt.relPath, err)
			continue
		}
		if got != tt.want {
			t.Errorf("DefaultURLPath(%q) = %q, want %q", tt.relPath, got, tt.want)
		}
	}
}

func TestEnsureLayout(t *testing.T) {
	v := createTestVault(t)
	err := site.EnsureLayout(v, "/tmp/test-nota")
	if err != nil {
		t.Fatal(err)
	}
	// Check that all dirs exist
	for _, d := range []string{
		"pages", "posts", "static", "keywords", "keywords/audits",
		"assets", "assets/images", "notes", "_archive", ".nota",
	} {
		_, err := v.Stat(d)
		if err != nil {
			t.Errorf("dir %s does not exist after EnsureLayout", d)
		}
	}
}

func TestEnsureLayoutCreatesSeedFiles(t *testing.T) {
	v := createTestVault(t)
	err := site.EnsureLayout(v, "/tmp/test-nota-seed")
	if err != nil {
		t.Fatal(err)
	}
	// Check seed files exist
	for _, f := range []string{
		"keywords/index.yaml",
		".nota/inventory.yaml",
	} {
		_, err := v.Stat(f)
		if err != nil {
			t.Errorf("seed file %s does not exist after EnsureLayout", f)
		}
	}
}

func TestValidatePostFolderMissingIndex(t *testing.T) {
	v := createTestVault(t)
	_, err := site.ValidatePostFolder(v, "test-topic")
	if err == nil {
		t.Error("expected error for missing pillar index.md")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

func TestValidatePostFolderWithIndex(t *testing.T) {
	v := createTestVault(t)
	// Create index.md
	indexPath := filepath.ToSlash(filepath.Join("posts", "test-topic", "index.md"))
	if err := v.Write(indexPath, []byte("test")); err != nil {
		t.Fatal(err)
	}
	indexPathStr, err := site.ValidatePostFolder(v, "test-topic")
	if err != nil {
		t.Fatal(err)
	}
	if indexPathStr != indexPath {
		t.Errorf("ValidatePostFolder got %q, want %q", indexPathStr, indexPath)
	}
}

func TestEnsurePillar(t *testing.T) {
	v := createTestVault(t)
	// Should succeed when pillar doesn't exist
	path, err := site.EnsurePillar(v, "new-topic", "My New Topic")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.ToSlash(filepath.Join("posts", "new-topic", "index.md"))
	if path != expected {
		t.Errorf("EnsurePillar got %q, want %q", path, expected)
	}

	// Second call should succeed (already exists)
	path2, err := site.EnsurePillar(v, "new-topic", "Renamed Topic")
	if err != nil {
		t.Fatal(err)
	}
	if path2 != expected {
		t.Errorf("EnsurePillar second call got %q, want %q", path2, expected)
	}
}

func TestCreateClusterPost(t *testing.T) {
	v := createTestVault(t)
	// First ensure pillar
	_, err := site.EnsurePillar(v, "my-topic", "Pillar Title")
	if err != nil {
		t.Fatal(err)
	}

	// Create cluster post
	path, err := site.CreateClusterPost(v, "my-topic", "cluster-post", "Cluster Post Title")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.ToSlash(filepath.Join("posts", "my-topic", "cluster-post.md"))
	if path != expected {
		t.Errorf("CreateClusterPost got %q, want %q", path, expected)
	}

	// Second cluster post should succeed
	path2, err := site.CreateClusterPost(v, "my-topic", "another-post", "Another Post")
	if err != nil {
		t.Fatal(err)
	}
	expected2 := filepath.ToSlash(filepath.Join("posts", "my-topic", "another-post.md"))
	if path2 != expected2 {
		t.Errorf("CreateClusterPost second got %q, want %q", path2, expected2)
	}

	// Duplicate should fail
	_, err = site.CreateClusterPost(v, "my-topic", "cluster-post", "Duplicate")
	if err == nil {
		t.Error("expected error for duplicate cluster post")
	}
}

func TestCreatePage(t *testing.T) {
	v := createTestVault(t)
	path, err := site.CreatePage(v, "pages", "use-cases/certificates.md", "Certificates", site.SourceStatic)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.ToSlash(filepath.Join("pages", "use-cases", "certificates.md"))
	if path != expected {
		t.Errorf("CreatePage got %q, want %q", path, expected)
	}

	// Duplicate should fail
	_, err = site.CreatePage(v, "pages", "use-cases/certificates.md", "Duplicate", site.SourceStatic)
	if err == nil {
		t.Error("expected error for duplicate page")
	}
}

func TestSaveLoadSite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "site-test-*")
	if err != nil {
		t.Fatal(err)
	}
	// Create site config dir and file
	siteRoot := filepath.Join(tmpDir, "my-site")
	if err := os.MkdirAll(siteRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &site.Site{
		ID:             "my-site",
		Name:           "My Site",
		PublicSiteURL:  "https://example.com",
		Kind:           site.KindFrameworkOnly,
		Framework:      site.FrameworkAstro,
	}

	err = site.SaveSite(siteRoot, s)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := site.LoadSite(siteRoot)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "my-site" {
		t.Errorf("Loaded site ID = %q, want %q", loaded.ID, "my-site")
	}
	if loaded.Name != "My Site" {
		t.Errorf("Loaded site Name = %q, want %q", loaded.Name, "My Site")
	}
	if loaded.Kind != site.KindFrameworkOnly {
		t.Errorf("Loaded site Kind = %q, want %q", loaded.Kind, "KindFrameworkOnly")
	}
	if loaded.Framework != site.FrameworkAstro {
		t.Errorf("Loaded site Framework = %q, want %q", loaded.Framework, "FrameworkAstro")
	}
}
