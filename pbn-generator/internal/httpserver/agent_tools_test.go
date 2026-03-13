package httpserver

import (
	"testing"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

func TestValidateAgentFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{"simple html", "index.html", false},
		{"nested", "about/index.html", false},
		{"css file", "assets/style.css", false},
		{"js file", "js/app.js", false},
		{"json file", "data/config.json", false},
		{"svg file", "assets/logo.svg", false},
		{"txt file", "robots.txt", false},
		{"xml file", "sitemap.xml", false},

		// Path traversal attacks
		{"traversal", "../etc/passwd", true},
		{"traversal nested", "subdir/../../etc/passwd", true},
		{"traversal start", "../", true},

		// Absolute paths
		{"absolute unix", "/etc/passwd", true},
		{"absolute drive", "C:/Windows/system32", true},

		// Empty path
		{"empty", "", true},

		// Forbidden filenames
		{"dot file", ".htaccess", true},

		// Disallowed extensions
		{"php file", "hack.php", true},
		{"exe file", "run.exe", true},
		{"sh file", "script.sh", true},
		{"py file", "script.py", true},
		{"binary no ext", "binary", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAgentFilePath(%q) err=%v, wantErr=%v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAgentImagePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid webp", "assets/hero.webp", false},
		{"valid png", "assets/logo.png", false},
		{"valid jpg", "assets/photo.jpg", false},
		{"valid nested", "assets/icons/arrow.svg", false},

		// Must be inside assets/
		{"no assets prefix", "images/hero.webp", true},
		{"root level", "hero.webp", true},

		// Path traversal
		{"traversal", "assets/../../etc/passwd", true},

		// Unsupported extension
		{"no extension", "assets/logo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentImagePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAgentImagePath(%q) err=%v, wantErr=%v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsProtectedAgentFile(t *testing.T) {
	tests := []struct {
		path      string
		protected bool
	}{
		{"index.html", true},
		{"about.html", false},
		{"assets/style.css", false},
		{".htaccess", true},
		{".env", true},
		{"index.html", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isProtectedAgentFile(tt.path)
			if got != tt.protected {
				t.Errorf("isProtectedAgentFile(%q) = %v, want %v", tt.path, got, tt.protected)
			}
		})
	}
}

func TestAgentFileChangedAction(t *testing.T) {
	t.Run("created when file does not exist", func(t *testing.T) {
		if got := agentFileChangedAction(nil); got != "created" {
			t.Fatalf("agentFileChangedAction(nil) = %q, want created", got)
		}
	})

	t.Run("updated when file exists", func(t *testing.T) {
		existing := &sqlstore.SiteFile{ID: "f1", Path: "assets/hero.webp"}
		if got := agentFileChangedAction(existing); got != "updated" {
			t.Fatalf("agentFileChangedAction(existing) = %q, want updated", got)
		}
	})
}
