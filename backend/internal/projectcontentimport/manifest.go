// Package projectcontentimport imports one strict versioned JSON Project
// Content manifest describing each Project's optional logo and Project
// Files, dispatching the two kinds of content to their separate domain
// services through the shared bounded Project pool.
package projectcontentimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ause-discovery.local/backend/internal/projectpool"
	"github.com/google/uuid"
)

const ManifestVersion = 1

type Manifest struct {
	Version  int               `json:"version"`
	Projects []ManifestProject `json:"projects"`
}

type ManifestProject struct {
	ProjectID        string         `json:"project_id"`
	ProjectImportKey string         `json:"project_import_key"`
	Logo             *ManifestLogo  `json:"logo"`
	Files            []ManifestFile `json:"files"`
}

type ManifestLogo struct {
	FilePath string `json:"file_path"`
}

type ManifestFile struct {
	ArtifactType     string `json:"artifact_type"`
	DisplayName      string `json:"display_name"`
	OriginalFilename string `json:"original_filename"`
	FilePath         string `json:"file_path"`
}

// LoadManifest reads and strictly validates one Project Content manifest.
// Unknown fields, unsupported versions, duplicate Projects, blank strings,
// and trailing JSON data are rejected. File paths are resolved during
// planning, not here.
func LoadManifest(path string) (Manifest, error) {
	manifestPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || strings.TrimSpace(path) == "" {
		return Manifest{}, errors.New("manifest path is required")
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("open manifest: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if decoder.More() {
		return Manifest{}, errors.New("manifest contains trailing JSON data")
	}
	if err := validateManifest(&manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// BundleRoot returns the directory containing the manifest file; every
// file_path resolves relative to it.
func BundleRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("manifest path is required")
	}
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	return filepath.Dir(absolute), nil
}

func validateManifest(manifest *Manifest) error {
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("unsupported manifest version %d, expected %d", manifest.Version, ManifestVersion)
	}
	if len(manifest.Projects) == 0 {
		return errors.New("manifest must contain at least one Project")
	}
	seen := map[string]bool{}
	for index := range manifest.Projects {
		project := &manifest.Projects[index]
		label := fmt.Sprintf("project %d", index+1)
		projectID := strings.TrimSpace(project.ProjectID)
		importKey := strings.TrimSpace(project.ProjectImportKey)
		if (projectID == "") == (importKey == "") {
			return fmt.Errorf("%s must set exactly one of project_id or project_import_key", label)
		}
		identity := "key:" + importKey
		if projectID != "" {
			parsed, err := uuid.Parse(projectID)
			if err != nil {
				return fmt.Errorf("%s project_id must be a UUID", label)
			}
			identity = parsed.String()
		}
		if seen[identity] {
			return fmt.Errorf("%s appears more than once in the manifest", identity)
		}
		seen[identity] = true
		project.ProjectID = projectID
		project.ProjectImportKey = importKey
		hasContent := false
		if project.Logo != nil {
			logoPath := strings.TrimSpace(project.Logo.FilePath)
			if logoPath == "" {
				return fmt.Errorf("%s logo requires a file_path", label)
			}
			if strings.Contains(project.Logo.FilePath, "\x00") {
				return fmt.Errorf("%s logo file_path is invalid", label)
			}
			project.Logo.FilePath = logoPath
			hasContent = true
		}
		for fileIndex := range project.Files {
			entry := &project.Files[fileIndex]
			fileLabel := fmt.Sprintf("%s file %d", label, fileIndex+1)
			entry.ArtifactType = strings.TrimSpace(entry.ArtifactType)
			entry.DisplayName = strings.TrimSpace(entry.DisplayName)
			entry.FilePath = strings.TrimSpace(entry.FilePath)
			if entry.ArtifactType == "" || entry.DisplayName == "" || entry.FilePath == "" {
				return fmt.Errorf("%s requires artifact_type, display_name, and file_path", fileLabel)
			}
			original := strings.TrimSpace(entry.OriginalFilename)
			if original == "" {
				original = filepath.Base(entry.FilePath)
			}
			entry.OriginalFilename = original
			hasContent = true
		}
		if !hasContent {
			return fmt.Errorf("%s must contain a logo, files, or both", label)
		}
	}
	return nil
}

// Worker bounds are shared with the other bulk importers.
const (
	DefaultWorkers = projectpool.DefaultWorkers
	MaxWorkers     = projectpool.MaxWorkers
)
