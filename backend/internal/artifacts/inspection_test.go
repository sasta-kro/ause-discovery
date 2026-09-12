package artifacts

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
)

func TestInspectUploadValidatesContentWithoutStoringIt(t *testing.T) {
	content := []byte("%PDF-1.7\nreport")
	inspection, err := InspectUpload(context.Background(), UploadInput{
		ArtifactType:     "report",
		DisplayName:      "Final report",
		OriginalFilename: "report.pdf",
		ExpectedSize:     int64(len(content)),
		Content:          bytes.NewReader(content),
	}, 1024)
	if err != nil {
		t.Fatalf("InspectUpload returned an error: %v", err)
	}
	if inspection.ByteCount != int64(len(content)) || inspection.SHA256 != sha256.Sum256(content) {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}

	unsafe := []byte("<html>unsafe</html>")
	if _, err := InspectUpload(context.Background(), UploadInput{
		ArtifactType: "report", DisplayName: "Report", OriginalFilename: "report.pdf", ExpectedSize: int64(len(unsafe)), Content: bytes.NewReader(unsafe),
	}, 1024); err == nil {
		t.Fatal("expected unsafe content to fail inspection")
	}
}

// docxFixture builds a minimal DOCX-compatible OOXML package with the
// standard library: a ZIP container carrying the Office content types and a
// word document part.
func docxFixture(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`,
	}
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create fixture entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatalf("write fixture entry %s: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close fixture archive: %v", err)
	}
	return buffer.Bytes()
}

func TestInspectUploadAcceptsDOCXReports(t *testing.T) {
	content := docxFixture(t)
	inspection, err := InspectUpload(context.Background(), UploadInput{
		ArtifactType:     "report",
		DisplayName:      "Final report",
		OriginalFilename: "final-report.docx",
		ExpectedSize:     int64(len(content)),
		Content:          bytes.NewReader(content),
	}, 1<<20)
	if err != nil {
		t.Fatalf("DOCX report inspection returned an error: %v", err)
	}
	if inspection.ByteCount != int64(len(content)) || inspection.SHA256 != sha256.Sum256(content) {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}

	// A plain text body named .docx fails content detection.
	mismatch := []byte("definitely not an office document")
	if _, err := InspectUpload(context.Background(), UploadInput{
		ArtifactType: "report", DisplayName: "Report", OriginalFilename: "report.docx", ExpectedSize: int64(len(mismatch)), Content: bytes.NewReader(mismatch),
	}, 1<<20); err == nil {
		t.Fatal("mismatched docx content passed inspection")
	}

	// Active web content stays unsafe even inside a docx-named upload.
	unsafe := append([]byte{}, docxFixture(t)...)
	unsafe[0] = '<'
	if _, err := InspectUpload(context.Background(), UploadInput{
		ArtifactType: "report", DisplayName: "Report", OriginalFilename: "report.docx", ExpectedSize: int64(len(unsafe)), Content: bytes.NewReader(unsafe),
	}, 1<<20); err == nil {
		t.Fatal("unsafe docx-named content passed inspection")
	}
}
