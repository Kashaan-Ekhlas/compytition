package uploads

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"compytition/internal/db"
)

func TestValidateDocumentAcceptsSupportedFormats(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		head     string
		wantExt  string
		wantMime string
	}{
		{name: "pdf", ext: ".pdf", head: "%PDF-1.7", wantExt: ".pdf", wantMime: "application/pdf"},
		{name: "ppt", ext: ".ppt", head: "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1", wantExt: ".ppt", wantMime: "application/vnd.ms-powerpoint"},
		{name: "pptx", ext: ".pptx", head: "PK\x03\x04", wantExt: ".pptx", wantMime: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, mime, limit, err := validateDocument(tt.ext, []byte(tt.head))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ext != tt.wantExt || mime != tt.wantMime {
				t.Fatalf("got %q %q, want %q %q", ext, mime, tt.wantExt, tt.wantMime)
			}
			if limit != attachmentMax {
				t.Fatalf("limit = %d, want %d", limit, attachmentMax)
			}
		})
	}
}

func TestValidateDocumentRejectsMismatchedContent(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		head string
	}{
		{name: "pdf without magic", ext: ".pdf", head: "not a pdf"},
		{name: "ppt without ole header", ext: ".ppt", head: "not a ppt"},
		{name: "pptx without zip header", ext: ".pptx", head: "not a pptx"},
		{name: "unsupported extension", ext: ".docx", head: "PK\x03\x04"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, _, err := validateDocument(tt.ext, []byte(tt.head)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestValidateImageMatchesDetectedType(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	ext, mime, limit, err := validateImage(KindAvatar, ".png", png)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ext != ".png" || mime != "image/png" || limit != avatarMax {
		t.Fatalf("unexpected result: %q %q %d", ext, mime, limit)
	}

	if _, _, _, err := validateImage(KindBanner, ".jpg", png); err == nil {
		t.Fatal("expected an extension mismatch error")
	}
	if _, _, _, err := validateImage(KindBanner, ".png", []byte("%PDF-1.7")); err == nil {
		t.Fatal("expected a non-image error")
	}
}

func TestValidateFileContentsRejectsBrokenImages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.png")
	if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\nnot enough data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateFileContents(KindAvatar, "image/png", path); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestValidateFileContentsChecksPresentationData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	if _, err := writer.Create("docProps/core.xml"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	err = validateFileContents(
		KindAttachment,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		path,
	)
	if err == nil {
		t.Fatal("expected a missing presentation error")
	}
}

func TestFileViewAddsPreviewForPDFAndConvertedPresentations(t *testing.T) {
	pdf := fileView(db.Upload{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Kind: KindAttachment, MimeType: "application/pdf"})
	if pdf.PreviewURL != pdf.URL+"?preview=1" {
		t.Fatalf("pdf preview = %q, want %q", pdf.PreviewURL, pdf.URL+"?preview=1")
	}
	ppt := fileView(db.Upload{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Kind: KindAttachment, MimeType: "application/vnd.ms-powerpoint", PreviewKey: "attachment/preview.pdf"})
	if ppt.PreviewURL != ppt.URL+"?preview=1" {
		t.Fatalf("ppt preview = %q, want %q", ppt.PreviewURL, ppt.URL+"?preview=1")
	}
	image := fileView(db.Upload{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Kind: KindAvatar, MimeType: "image/png"})
	if image.PreviewURL != "" {
		t.Fatalf("avatar preview = %q, want empty", image.PreviewURL)
	}
}

func TestSafeNameStripsPathsAndControls(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd":      "passwd",
		`C:\Users\me\deck.pptx`: "deck.pptx",
		"  slides\x00.pdf  ":    "slides.pdf",
		"":                      "file",
		"..":                    "file",
	}
	for input, want := range cases {
		if got := safeName(input); got != want {
			t.Errorf("safeName(%q) = %q, want %q", input, got, want)
		}
	}
	if got := safeName(strings.Repeat("a", 300) + ".pdf"); len([]rune(got)) != 180 {
		t.Errorf("long name length = %d, want 180", len([]rune(got)))
	}
}
