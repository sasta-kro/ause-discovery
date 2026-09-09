package artifacts

import (
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
