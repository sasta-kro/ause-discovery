package artifactimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var manifestHeaders = []string{"project_id", "project_import_key", "artifact_type", "display_name", "file_path"}

func LoadManifest(path string) ([]Entry, error) {
	manifestPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || strings.TrimSpace(path) == "" {
		return nil, errors.New("manifest path is required")
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = len(manifestHeaders)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read manifest header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	for index, expected := range manifestHeaders {
		if strings.TrimSpace(header[index]) != expected {
			return nil, fmt.Errorf("manifest headers must be %s", strings.Join(manifestHeaders, ","))
		}
	}
	entries := []Entry{}
	for rowNumber := 2; ; rowNumber++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read manifest row %d: %w", rowNumber, err)
		}
		if allBlank(record) {
			continue
		}
		projectIDText := strings.TrimSpace(record[0])
		projectImportKey := strings.TrimSpace(record[1])
		if (projectIDText == "") == (projectImportKey == "") {
			return nil, fmt.Errorf("manifest row %d must set exactly one of project_id or project_import_key", rowNumber)
		}
		var projectID uuid.UUID
		if projectIDText != "" {
			projectID, err = uuid.Parse(projectIDText)
			if err != nil {
				return nil, fmt.Errorf("manifest row %d project_id must be a UUID", rowNumber)
			}
		}
		sourcePath, err := resolveSourcePath(filepath.Dir(manifestPath), strings.TrimSpace(record[4]))
		if err != nil {
			return nil, fmt.Errorf("manifest row %d file_path: %w", rowNumber, err)
		}
		entries = append(entries, Entry{
			ProjectID:        projectID,
			ProjectImportKey: projectImportKey,
			ArtifactType:     strings.TrimSpace(record[2]),
			DisplayName:      strings.TrimSpace(record[3]),
			SourcePath:       sourcePath,
		})
	}
	if len(entries) == 0 {
		return nil, errors.New("manifest contains no Project files")
	}
	return entries, nil
}

func resolveSourcePath(root, relativePath string) (string, error) {
	if relativePath == "" {
		return "", errors.New("path is required")
	}
	if filepath.IsAbs(relativePath) {
		return "", errors.New("path must be relative")
	}
	rootPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve source directory: %w", err)
	}
	filePath, err := filepath.EvalSymlinks(filepath.Join(rootPath, filepath.Clean(relativePath)))
	if err != nil {
		return "", fmt.Errorf("resolve source file: %w", err)
	}
	relative, err := filepath.Rel(rootPath, filePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the source directory")
	}
	information, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("inspect source file: %w", err)
	}
	if !information.Mode().IsRegular() {
		return "", errors.New("path must identify a regular file")
	}
	return filePath, nil
}

func allBlank(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
