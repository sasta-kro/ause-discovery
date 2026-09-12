package artifacts

import (
	"errors"
	"testing"
)

func TestValidateUploadMetadataNormalizesSafeValues(t *testing.T) {
	metadata, err := validateUploadMetadata("slides", "  Final presentation  ", `C:\fakepath\Final Deck.PPTX`)
	if err != nil {
		t.Fatalf("validateUploadMetadata returned an error: %v", err)
	}
	if metadata.DisplayName != "Final presentation" {
		t.Fatalf("display name was %q", metadata.DisplayName)
	}
	if metadata.OriginalFilename != "Final Deck.PPTX" {
		t.Fatalf("original filename was %q", metadata.OriginalFilename)
	}
	if metadata.Extension != "pptx" {
		t.Fatalf("extension was %q", metadata.Extension)
	}
}

func TestValidateUploadMetadataEnforcesTypeAllowlist(t *testing.T) {
	tests := []struct {
		artifactType string
		filename     string
	}{
		{artifactType: "report", filename: "report.doc"},
		{artifactType: "report", filename: "report.docm"},
		{artifactType: "report", filename: "report.zip"},
		{artifactType: "report", filename: "report"},
		{artifactType: "source_code", filename: "source.exe"},
		{artifactType: "poster", filename: "poster.svg"},
		{artifactType: "dataset", filename: "sheet.xlsm"},
		{artifactType: "unknown", filename: "file.pdf"},
	}
	for _, test := range tests {
		if _, err := validateUploadMetadata(test.artifactType, "Artifact", test.filename); !errors.Is(err, ErrArtifactTypeMismatch) {
			t.Errorf("type %q filename %q returned %v, expected type mismatch", test.artifactType, test.filename, err)
		}
	}
}

func TestValidateUploadMetadataAcceptsDOCXReports(t *testing.T) {
	for _, filename := range []string{"report.docx", "Final Report.DOCX", "final-report.Docx"} {
		metadata, err := validateUploadMetadata("report", "Final report", filename)
		if err != nil {
			t.Fatalf("report filename %q returned %v, expected acceptance", filename, err)
		}
		if metadata.Extension != "docx" {
			t.Fatalf("filename %q normalized to extension %q, expected docx", filename, metadata.Extension)
		}
	}
}

func TestValidateDetectedContentRejectsUnsafeOrMismatchedContent(t *testing.T) {
	if err := validateDetectedContent("pdf", "application/pdf", []byte("%PDF-1.7")); err != nil {
		t.Fatalf("PDF content returned an error: %v", err)
	}
	if err := validateDetectedContent("pdf", "text/plain; charset=utf-8", []byte("not a PDF")); !errors.Is(err, ErrContentTypeMismatch) {
		t.Fatalf("mismatched PDF returned %v, expected content type mismatch", err)
	}
	if err := validateDetectedContent("json", "text/html; charset=utf-8", []byte("<html><script>")); !errors.Is(err, ErrUnsafeContentType) {
		t.Fatalf("HTML content returned %v, expected unsafe content type", err)
	}
	if err := validateDetectedContent("zip", "application/zip", []byte("PK\x03\x04")); err != nil {
		t.Fatalf("ZIP content returned an error: %v", err)
	}
	if err := validateDetectedContent("docx", "application/zip", []byte("PK\x03\x04")); err != nil {
		t.Fatalf("DOCX container content returned an error: %v", err)
	}
	if err := validateDetectedContent("docx", "application/pdf", []byte("%PDF-1.7")); !errors.Is(err, ErrContentTypeMismatch) {
		t.Fatalf("PDF content named docx returned %v, expected content type mismatch", err)
	}
	if err := validateDetectedContent("docx", "text/html; charset=utf-8", []byte("<html><script>")); !errors.Is(err, ErrUnsafeContentType) {
		t.Fatalf("HTML content named docx returned %v, expected unsafe content type", err)
	}
	if err := validateDetectedContent("docx", "application/octet-stream", []byte{0x7f, 'E', 'L', 'F'}); !errors.Is(err, ErrUnsafeContentType) {
		t.Fatalf("executable content named docx returned %v, expected unsafe content type", err)
	}
	if err := validateDetectedContent("json", "application/octet-stream", []byte{0x7f, 'E', 'L', 'F'}); !errors.Is(err, ErrUnsafeContentType) {
		t.Fatalf("executable content returned %v, expected unsafe content type", err)
	}
}
