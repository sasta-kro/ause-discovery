// Package projectcontentimport imports one strict versioned JSON Project
// Content manifest describing each Project's optional logo, Project Files,
// and repository links, dispatching each kind of content to its separate
// domain service through the shared bounded Project pool.
package projectcontentimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	Links            *optionalLinks `json:"links"`
}

// ManifestLink is one authoritative repository link declaration. Every
// required field must be explicitly present: a JSON object, never null,
// carrying url, primary, availability, and checked_at. An omitted primary
// is rejected rather than read as false.
type ManifestLink struct {
	URL          string `json:"url"`
	IsPrimary    bool   `json:"primary"`
	Availability string `json:"availability"`
	CheckedAt    string `json:"checked_at"`
}

func (link *ManifestLink) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("repository link must be an object, not null")
	}
	var present struct {
		URL          *string `json:"url"`
		IsPrimary    *bool   `json:"primary"`
		Availability *string `json:"availability"`
		CheckedAt    *string `json:"checked_at"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&present); err != nil {
		return err
	}
	if present.URL == nil || present.IsPrimary == nil || present.Availability == nil || present.CheckedAt == nil {
		return errors.New("repository link requires url, primary, availability, and checked_at")
	}
	link.URL, link.IsPrimary, link.Availability, link.CheckedAt = *present.URL, *present.IsPrimary, *present.Availability, *present.CheckedAt
	return nil
}

// optionalLinks keeps field presence distinct from emptiness: omitted links
// leave the Project's repository links unchanged, while an explicitly
// present array, including the empty array, is the complete desired set. A
// JSON null is rejected.
type optionalLinks struct {
	Links []ManifestLink
}

func (optional *optionalLinks) UnmarshalJSON(data []byte) error {
	if strings.TrimSpace(string(data)) == "null" {
		return errors.New("links must be an array, not null")
	}
	optional.Links = nil
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(&optional.Links)
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
	// The document must end at EOF: a second decode must report io.EOF, so a
	// second JSON document, a stray closing brace or bracket, trailing
	// non-JSON text, and whitespace-only trailing content are distinguished
	// exactly (whitespace remains valid).
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
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
		if project.Links != nil {
			for linkIndex := range project.Links.Links {
				link := &project.Links.Links[linkIndex]
				linkLabel := fmt.Sprintf("%s repository link %d", label, linkIndex+1)
				link.URL = strings.TrimSpace(link.URL)
				if link.URL == "" {
					return fmt.Errorf("%s requires a url", linkLabel)
				}
				if _, err := time.Parse(time.RFC3339, link.CheckedAt); err != nil {
					return fmt.Errorf("%s checked_at must be an RFC 3339 timestamp", linkLabel)
				}
			}
			hasContent = true
		}
		if !hasContent {
			return fmt.Errorf("%s must contain a logo, files, links, or a combination", label)
		}
	}
	return nil
}

// Worker bounds are shared with the other bulk importers.
const (
	DefaultWorkers = projectpool.DefaultWorkers
	MaxWorkers     = projectpool.MaxWorkers
)
