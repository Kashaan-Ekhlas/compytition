package uploads

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"compytition/internal/apperr"
	"compytition/internal/config"
	"compytition/internal/db"
)

type fakeStore struct {
	upload       db.Upload
	uploadErr    error
	created      *db.CreateUploadParams
	createdRow   db.Upload
	createErr    error
	deleted      []uuid.UUID
	deleteErr    error
	profile      db.Profile
	profileErr   error
	avatarParams []db.SetProfileAvatarParams
	avatarErr    error
	usedBytes    int64
	sumErr       error
	event        db.GetEventByIDRow
	eventErr     error
	registration db.Registration
	regErr       error
	canManage    bool
	canManageErr error
	isJudge      bool
	judgeErr     error
	teammate     bool
	teammateErr  error
	bannerCount  int32
	bannerErr    error
}

func (f *fakeStore) CreateUpload(_ context.Context, arg db.CreateUploadParams) (db.Upload, error) {
	f.created = &arg
	return f.createdRow, f.createErr
}

func (f *fakeStore) GetUpload(_ context.Context, _ uuid.UUID) (db.Upload, error) {
	return f.upload, f.uploadErr
}

func (f *fakeStore) DeleteUpload(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

func (f *fakeStore) SetProfileAvatar(_ context.Context, arg db.SetProfileAvatarParams) error {
	f.avatarParams = append(f.avatarParams, arg)
	return f.avatarErr
}

func (f *fakeStore) GetProfile(_ context.Context, _ uuid.UUID) (db.Profile, error) {
	if f.profileErr != nil {
		return db.Profile{}, f.profileErr
	}
	if f.profile.UserID == uuid.Nil {
		return db.Profile{}, pgx.ErrNoRows
	}
	return f.profile, nil
}

func (f *fakeStore) SumUploadBytesByOwner(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.usedBytes, f.sumErr
}

func (f *fakeStore) GetEventByID(_ context.Context, _ uuid.UUID) (db.GetEventByIDRow, error) {
	return f.event, f.eventErr
}

func (f *fakeStore) GetRegistrationForEventUser(_ context.Context, _ db.GetRegistrationForEventUserParams) (db.Registration, error) {
	return f.registration, f.regErr
}

func (f *fakeStore) CanManageEvent(_ context.Context, _ db.CanManageEventParams) (bool, error) {
	return f.canManage, f.canManageErr
}

func (f *fakeStore) IsJudgeForEvent(_ context.Context, _ db.IsJudgeForEventParams) (bool, error) {
	return f.isJudge, f.judgeErr
}

func (f *fakeStore) HasSubmissionAttachmentAccess(_ context.Context, _ db.HasSubmissionAttachmentAccessParams) (bool, error) {
	return f.teammate, f.teammateErr
}

func (f *fakeStore) CountEventsUsingBanner(_ context.Context, _ string) (int32, error) {
	return f.bannerCount, f.bannerErr
}

func newTestService(t *testing.T, store Store) *Service {
	t.Helper()
	cfg := &config.Config{UploadDir: t.TempDir(), UploadQuotaBytes: 250 << 20}
	return NewService(store, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func writeTestFile(t *testing.T, root, key string, data []byte) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

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

func TestValidateFileContentsChecksWebPDimensions(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRkwAAABXRUJQVlA4IEAAAABwAwCdASogACAAPm00lkekIyIhKAgAgA2JZQDKLoB+AABBlBAA/vCbQ//ILlhdcjX/8gP+QH/ID/+PfYwoMAAA")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "avatar.webp")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateFileContents(KindAvatar, "image/webp", path); err != nil {
		t.Fatalf("valid webp rejected: %v", err)
	}

	truncated := filepath.Join(t.TempDir(), "broken.webp")
	if err := os.WriteFile(truncated, data[:20], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateFileContents(KindAvatar, "image/webp", truncated); err == nil {
		t.Fatal("expected a malformed webp error")
	}
}

func TestFileViewPreviewsOnlyPDF(t *testing.T) {
	pdf := fileView(db.Upload{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Kind: KindAttachment, MimeType: "application/pdf"})
	if pdf.PreviewURL != pdf.URL+"?preview=1" {
		t.Fatalf("pdf preview = %q, want %q", pdf.PreviewURL, pdf.URL+"?preview=1")
	}
	ppt := fileView(db.Upload{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Kind: KindAttachment, MimeType: "application/vnd.ms-powerpoint"})
	if ppt.PreviewURL != "" {
		t.Fatalf("ppt preview = %q, want empty", ppt.PreviewURL)
	}
	image := fileView(db.Upload{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Kind: KindAvatar, MimeType: "image/png"})
	if image.PreviewURL != "" {
		t.Fatalf("avatar preview = %q, want empty", image.PreviewURL)
	}
}

func TestUploadPathHelpers(t *testing.T) {
	id := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if got := PathFor(id); got != "/api/v1/uploads/"+id.String() {
		t.Fatalf("PathFor = %q", got)
	}
	if parsed, ok := IDFromPath(PathFor(id)); !ok || parsed != id {
		t.Fatalf("IDFromPath = %v, %v", parsed, ok)
	}
	for _, raw := range []string{"", "/api/v1/uploads/", PathFor(id) + "/extra", PathFor(id) + "?preview=1", "https://example.com/x"} {
		if _, ok := IDFromPath(raw); ok {
			t.Fatalf("IDFromPath(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestAuthorizeAttachment(t *testing.T) {
	userID := uuid.New()
	ownerID := uuid.New()
	eventID := uuid.New()
	row := db.Upload{ID: uuid.New(), OwnerID: ownerID, Kind: KindAttachment, EventID: &eventID}

	tests := []struct {
		name  string
		user  *uuid.UUID
		admin bool
		setup func(*fakeStore)
		want  error
	}{
		{name: "anonymous", user: nil, want: apperr.ErrUnauthorized},
		{name: "owner", user: &ownerID, want: nil},
		{name: "admin", user: &userID, admin: true, want: nil},
		{name: "manager", user: &userID, setup: func(f *fakeStore) { f.canManage = true }, want: nil},
		{name: "judge", user: &userID, setup: func(f *fakeStore) { f.isJudge = true }, want: nil},
		{name: "teammate", user: &userID, setup: func(f *fakeStore) { f.teammate = true }, want: nil},
		{name: "stranger", user: &userID, want: apperr.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{}
			if tt.setup != nil {
				tt.setup(store)
			}
			svc := newTestService(t, store)
			err := svc.authorize(context.Background(), tt.user, tt.admin, row)
			if !errors.Is(err, tt.want) {
				t.Fatalf("authorize error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRequireParticipantChecksLifecycle(t *testing.T) {
	userID := uuid.New()
	eventID := uuid.New()
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	tests := []struct {
		name  string
		store *fakeStore
		want  error
	}{
		{name: "open", store: &fakeStore{event: db.GetEventByIDRow{Status: "published", SubmissionDeadline: &future}, registration: db.Registration{Status: "registered"}}},
		{name: "draft", store: &fakeStore{event: db.GetEventByIDRow{Status: "draft"}}, want: apperr.ErrValidation},
		{name: "deadline passed", store: &fakeStore{event: db.GetEventByIDRow{Status: "published", SubmissionDeadline: &past}}, want: apperr.ErrValidation},
		{name: "not registered", store: &fakeStore{event: db.GetEventByIDRow{Status: "published"}, registration: db.Registration{Status: "cancelled"}}, want: apperr.ErrValidation},
		{name: "missing event", store: &fakeStore{eventErr: pgx.ErrNoRows}, want: apperr.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t, tt.store)
			err := svc.requireParticipant(context.Background(), userID, eventID)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCheckQuota(t *testing.T) {
	store := &fakeStore{usedBytes: 8 << 20}
	svc := newTestService(t, store)
	svc.quota = 10 << 20
	if err := svc.checkQuota(context.Background(), uuid.New(), 1<<20); err != nil {
		t.Fatalf("under quota: %v", err)
	}
	if err := svc.checkQuota(context.Background(), uuid.New(), 3<<20); err == nil {
		t.Fatal("expected an over-quota error")
	}
	store.sumErr = errors.New("db down")
	if err := svc.checkQuota(context.Background(), uuid.New(), 1); err == nil {
		t.Fatal("expected the store error to surface")
	}
}

func TestSaveRejectsOverQuotaBeforeCreatingRow(t *testing.T) {
	store := &fakeStore{usedBytes: 4 << 20}
	svc := newTestService(t, store)
	svc.quota = 4 << 20
	header := &multipart.FileHeader{Filename: "avatar.png"}
	f := tempMultipartFile(t, []byte("\x89PNG\r\n\x1a\n"))
	if _, err := svc.Save(context.Background(), uuid.New(), KindAvatar, nil, header, f); err == nil {
		t.Fatal("expected an over-quota error")
	}
	if store.created != nil {
		t.Fatal("upload row was created despite the quota failure")
	}
}

func TestGetAllowsPDFPreviewOnly(t *testing.T) {
	id := uuid.New()
	owner := uuid.New()
	store := &fakeStore{}
	svc := newTestService(t, store)
	writeTestFile(t, svc.root, "attachment/deck.pdf", []byte("%PDF-1.7"))

	store.upload = db.Upload{ID: id, OwnerID: owner, Kind: KindAttachment, MimeType: "application/pdf", StorageKey: "attachment/deck.pdf", OriginalName: "deck.pdf"}
	if _, err := svc.Get(context.Background(), &owner, false, id, true); err != nil {
		t.Fatalf("pdf preview: %v", err)
	}

	store.upload = db.Upload{ID: id, OwnerID: owner, Kind: KindAttachment, MimeType: "application/vnd.ms-powerpoint", StorageKey: "attachment/deck.pdf", OriginalName: "deck.ppt"}
	if _, err := svc.Get(context.Background(), &owner, false, id, true); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("ppt preview error = %v, want not found", err)
	}
	if _, err := svc.Get(context.Background(), &owner, false, id, false); err != nil {
		t.Fatalf("ppt download: %v", err)
	}
}

func TestDeleteAvatarRemovesFileAndRow(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	key := "avatar/" + id.String() + ".png"
	store := &fakeStore{
		profile: db.Profile{UserID: userID, AvatarUrl: PathFor(id)},
		upload:  db.Upload{ID: id, OwnerID: userID, Kind: KindAvatar, StorageKey: key},
	}
	svc := newTestService(t, store)
	path := writeTestFile(t, svc.root, key, []byte("image"))
	if err := svc.DeleteAvatar(context.Background(), userID); err != nil {
		t.Fatalf("delete avatar: %v", err)
	}
	if len(store.avatarParams) == 0 || store.avatarParams[len(store.avatarParams)-1].AvatarUrl != "" {
		t.Fatal("profile pointer was not cleared")
	}
	if len(store.deleted) != 1 || store.deleted[0] != id {
		t.Fatalf("deleted = %v, want [%s]", store.deleted, id)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestDeleteOwnedUploadKeepsReferencedBanner(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	key := "banner/" + id.String() + ".png"
	url := PathFor(id)

	kept := &fakeStore{upload: db.Upload{ID: id, OwnerID: userID, Kind: KindBanner, StorageKey: key}, bannerCount: 1}
	svc := newTestService(t, kept)
	writeTestFile(t, svc.root, key, []byte("image"))
	if err := DeleteOwnedUpload(context.Background(), kept, svc.root, userID, url, KindBanner); err != nil {
		t.Fatalf("keep banner: %v", err)
	}
	if len(kept.deleted) != 0 {
		t.Fatal("referenced banner was deleted")
	}

	removed := &fakeStore{upload: db.Upload{ID: id, OwnerID: userID, Kind: KindBanner, StorageKey: key}}
	svc = newTestService(t, removed)
	path := writeTestFile(t, svc.root, key, []byte("image"))
	if err := DeleteOwnedUpload(context.Background(), removed, svc.root, userID, url, KindBanner); err != nil {
		t.Fatalf("remove banner: %v", err)
	}
	if len(removed.deleted) != 1 {
		t.Fatalf("deleted = %v, want one row", removed.deleted)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("banner file still exists: %v", err)
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

func tempMultipartFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "upload-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}
