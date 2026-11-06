// Package uploads stores user files on disk and tracks their metadata in the
// database. Avatars and event banners are public; submission attachments are
// restricted to the owner, event staff and judges.
package uploads

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"compytition/internal/apperr"
	"compytition/internal/config"
	"compytition/internal/db"
)

const (
	KindAvatar     = "avatar"
	KindBanner     = "banner"
	KindAttachment = "attachment"

	avatarMax     int64 = 4 << 20
	bannerMax     int64 = 8 << 20
	attachmentMax int64 = 25 << 20
)

type Service struct {
	store  *db.Store
	cfg    *config.Config
	log    *slog.Logger
	root   string
	office string
}

type FileView struct {
	ID         uuid.UUID `json:"id"`
	URL        string    `json:"url"`
	PreviewURL string    `json:"preview_url"`
	Name       string    `json:"name"`
	Mime       string    `json:"mime"`
	Size       int64     `json:"size"`
}

type File struct {
	Path    string
	Name    string
	Mime    string
	Kind    string
	ModTime time.Time
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	root := cfg.UploadDir
	for _, dir := range []string{KindAvatar, KindBanner, KindAttachment} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			log.Warn("could not prepare upload directory", "dir", dir, "error", err)
		}
	}
	office, _ := exec.LookPath("soffice")
	return &Service{store: store, cfg: cfg, log: log, root: root, office: office}
}

// MaxBytes is the request body ceiling for a kind. It includes room for the
// multipart envelope on top of the file itself.
func MaxBytes(kind string) int64 {
	switch kind {
	case KindAvatar:
		return avatarMax + (1 << 20)
	case KindBanner:
		return bannerMax + (1 << 20)
	case KindAttachment:
		return attachmentMax + (2 << 20)
	default:
		return 1 << 20
	}
}

func (s *Service) Save(ctx context.Context, userID uuid.UUID, kind string, eventID *uuid.UUID, header *multipart.FileHeader, src multipart.File) (*FileView, error) {
	if kind != KindAvatar && kind != KindBanner && kind != KindAttachment {
		return nil, apperr.Invalid("kind", "Unknown upload type.")
	}
	if kind == KindAttachment {
		if eventID == nil {
			return nil, apperr.Invalid("event_id", "Choose an event for this attachment.")
		}
		if err := s.requireParticipant(ctx, userID, *eventID); err != nil {
			return nil, err
		}
	} else {
		eventID = nil
	}

	name := safeName(header.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	head := make([]byte, 512)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, apperr.Invalid("file", "Could not read that file.")
	}
	head = head[:n]
	if len(head) == 0 {
		return nil, apperr.Invalid("file", "The file is empty.")
	}

	canonicalExt, mimeType, limit, err := validateKind(kind, ext, head)
	if err != nil {
		return nil, err
	}

	id := uuid.New()
	tmp, err := os.CreateTemp(s.root, "incoming-*"+canonicalExt)
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(head); err != nil {
		return nil, err
	}
	written, err := io.Copy(tmp, io.LimitReader(src, limit+1-int64(len(head))))
	if err != nil {
		return nil, err
	}
	total := int64(len(head)) + written
	if total > limit {
		return nil, apperr.Invalid("file", fmt.Sprintf("Keep this file under %d MB.", limit>>20))
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := validateFileContents(kind, mimeType, tmpName); err != nil {
		return nil, err
	}

	key := filepath.ToSlash(filepath.Join(kind, id.String()+canonicalExt))
	if err := os.Rename(tmpName, filepath.Join(s.root, filepath.FromSlash(key))); err != nil {
		return nil, err
	}

	previewKey := ""
	if kind == KindAttachment && (canonicalExt == ".ppt" || canonicalExt == ".pptx") {
		if converted, convErr := s.convertPreview(ctx, filepath.Join(s.root, filepath.FromSlash(key)), id.String()); convErr == nil {
			previewKey = converted
		} else {
			s.log.Warn("presentation preview failed", "file", name, "error", convErr)
		}
	}

	row, err := s.store.CreateUpload(ctx, db.CreateUploadParams{
		OwnerID: userID, Kind: kind, EventID: eventID, OriginalName: name,
		MimeType: mimeType, SizeBytes: total, StorageKey: key, PreviewKey: previewKey,
	})
	if err != nil {
		_ = os.Remove(filepath.Join(s.root, filepath.FromSlash(key)))
		if previewKey != "" {
			_ = os.Remove(filepath.Join(s.root, filepath.FromSlash(previewKey)))
		}
		return nil, err
	}
	view := fileView(row)
	if kind == KindAvatar {
		if err := s.store.SetProfileAvatar(ctx, db.SetProfileAvatarParams{UserID: userID, AvatarUrl: view.URL}); err != nil {
			return nil, err
		}
	}
	return &view, nil
}

func (s *Service) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	return s.store.SetProfileAvatar(ctx, db.SetProfileAvatarParams{UserID: userID})
}

func (s *Service) Get(ctx context.Context, userID *uuid.UUID, isAdmin bool, id uuid.UUID, preview bool) (*File, error) {
	row, err := s.store.GetUpload(ctx, id)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := s.authorize(ctx, userID, isAdmin, row); err != nil {
		return nil, err
	}
	key := row.StorageKey
	name := row.OriginalName
	mimeType := row.MimeType
	if preview {
		if row.PreviewKey == "" {
			if row.MimeType != "application/pdf" {
				return nil, apperr.ErrNotFound
			}
		} else {
			key = row.PreviewKey
			name = strings.TrimSuffix(name, filepath.Ext(name)) + ".pdf"
			mimeType = "application/pdf"
		}
	}
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if info.IsDir() {
		return nil, apperr.ErrNotFound
	}
	return &File{Path: path, Name: name, Mime: mimeType, Kind: row.Kind, ModTime: info.ModTime()}, nil
}

func (s *Service) authorize(ctx context.Context, userID *uuid.UUID, isAdmin bool, row db.Upload) error {
	if row.Kind != KindAttachment {
		return nil
	}
	if userID == nil {
		return apperr.ErrUnauthorized
	}
	if *userID == row.OwnerID || isAdmin {
		return nil
	}
	if row.EventID == nil {
		return apperr.ErrForbidden
	}
	allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: *userID, ID: *row.EventID})
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	judge, err := s.store.IsJudgeForEvent(ctx, db.IsJudgeForEventParams{EventID: *row.EventID, UserID: userID})
	if err != nil {
		return err
	}
	if judge {
		return nil
	}
	return apperr.ErrForbidden
}

func (s *Service) requireParticipant(ctx context.Context, userID, eventID uuid.UUID) error {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if ev.Status != "published" {
		return apperr.Invalid("event", "This event is not accepting submissions.")
	}
	if ev.SubmissionDeadline != nil && ev.SubmissionDeadline.Before(time.Now()) {
		return apperr.Invalid("event", "The submission deadline has passed.")
	}
	reg, err := s.store.GetRegistrationForEventUser(ctx, db.GetRegistrationForEventUserParams{EventID: eventID, UserID: userID})
	if err != nil || reg.Status != "registered" {
		return apperr.Invalid("registration", "Register for this event before attaching a file.")
	}
	return nil
}

func (s *Service) resolve(key string) (string, error) {
	if key == "" {
		return "", apperr.ErrNotFound
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", apperr.ErrNotFound
	}
	return path, nil
}

func (s *Service) convertPreview(ctx context.Context, src, id string) (string, error) {
	if s.office == "" {
		return "", errors.New("no office converter is installed")
	}
	profileDir, err := os.MkdirTemp(s.root, "lo-profile-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(profileDir)
	outDir, err := os.MkdirTemp(s.root, "lo-out-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(outDir)

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	profileURL := (&url.URL{Scheme: "file", Path: profileDir}).String()
	cmd := exec.CommandContext(ctx, s.office,
		"--headless", "--norestore", "--nologo", "--nodefault", "--nofirststartwizard",
		"-env:UserInstallation="+profileURL,
		"--convert-to", "pdf", "--outdir", outDir, src,
	)
	// soffice is a wrapper that starts soffice.bin. Killing only the wrapper
	// leaves the real converter holding the output pipe, so cancel the whole
	// process group and stop waiting shortly afterwards.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = append(os.Environ(), "HOME="+profileDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("soffice: %w: %s", err, strings.TrimSpace(string(output)))
	}
	expected := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))+".pdf")
	if _, err := os.Stat(expected); err != nil {
		return "", fmt.Errorf("soffice produced no pdf: %w", err)
	}
	key := filepath.ToSlash(filepath.Join(KindAttachment, id+".preview.pdf"))
	if err := os.Rename(expected, filepath.Join(s.root, filepath.FromSlash(key))); err != nil {
		return "", err
	}
	return key, nil
}

func fileView(row db.Upload) FileView {
	view := FileView{
		ID: row.ID, URL: "/api/v1/uploads/" + row.ID.String(),
		Name: row.OriginalName, Mime: row.MimeType, Size: row.SizeBytes,
	}
	if row.PreviewKey != "" {
		view.PreviewURL = view.URL + "?preview=1"
	} else if row.Kind == KindAttachment && row.MimeType == "application/pdf" {
		view.PreviewURL = view.URL + "?preview=1"
	}
	return view
}

func validateKind(kind, ext string, head []byte) (string, string, int64, error) {
	switch kind {
	case KindAvatar, KindBanner:
		return validateImage(kind, ext, head)
	case KindAttachment:
		return validateDocument(ext, head)
	default:
		return "", "", 0, apperr.Invalid("kind", "Unknown upload type.")
	}
}

func validateImage(kind, ext string, head []byte) (string, string, int64, error) {
	limit := avatarMax
	if kind == KindBanner {
		limit = bannerMax
	}
	detected := detectImage(head)
	if detected == "" {
		return "", "", 0, apperr.Invalid("file", "Use a JPEG, PNG, GIF or WebP image.")
	}
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowed[ext] {
		return "", "", 0, apperr.Invalid("file", "Use a JPEG, PNG, GIF or WebP image.")
	}
	if !compatibleImageExt(detected, ext) {
		return "", "", 0, apperr.Invalid("file", "The file extension does not match the image.")
	}
	canonical := detectedExt(detected)
	return canonical, detected, limit, nil
}

func validateDocument(ext string, head []byte) (string, string, int64, error) {
	switch ext {
	case ".pdf":
		if !strings.HasPrefix(string(head), "%PDF-") {
			return "", "", 0, apperr.Invalid("file", "That PDF is not valid.")
		}
		return ".pdf", "application/pdf", attachmentMax, nil
	case ".ppt":
		if len(head) < 8 || string(head[:8]) != "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1" {
			return "", "", 0, apperr.Invalid("file", "That PowerPoint file is not valid.")
		}
		return ".ppt", "application/vnd.ms-powerpoint", attachmentMax, nil
	case ".pptx":
		if len(head) < 4 || string(head[:4]) != "PK\x03\x04" {
			return "", "", 0, apperr.Invalid("file", "That PowerPoint file is not valid.")
		}
		return ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", attachmentMax, nil
	default:
		return "", "", 0, apperr.Invalid("file", "Attach a PDF, PPT or PPTX file.")
	}
}

func validateFileContents(kind, mimeType, path string) error {
	if mimeType == "image/webp" {
		return nil
	}
	switch kind {
	case KindAvatar, KindBanner:
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			return apperr.Invalid("file", "That image could not be read.")
		}
		if cfg.Width < 16 || cfg.Height < 16 || cfg.Width > 12_000 || cfg.Height > 12_000 {
			return apperr.Invalid("file", "Use an image between 16 and 12,000 pixels per side.")
		}
	case KindAttachment:
		if mimeType == "application/vnd.openxmlformats-officedocument.presentationml.presentation" {
			reader, err := zip.OpenReader(path)
			if err != nil {
				return apperr.Invalid("file", "That PowerPoint file could not be read.")
			}
			defer reader.Close()
			found := false
			for _, file := range reader.File {
				if file.Name == "ppt/presentation.xml" {
					found = true
					break
				}
			}
			if !found {
				return apperr.Invalid("file", "That PowerPoint file is missing its presentation data.")
			}
		}
	}
	return nil
}

func detectImage(head []byte) string {
	kind := strings.ToLower(http.DetectContentType(head))
	switch kind {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return kind
	default:
		return ""
	}
}

func detectedExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}

func compatibleImageExt(mimeType, ext string) bool {
	switch mimeType {
	case "image/jpeg":
		return ext == ".jpg" || ext == ".jpeg"
	case "image/png":
		return ext == ".png"
	case "image/gif":
		return ext == ".gif"
	case "image/webp":
		return ext == ".webp"
	default:
		return false
	}
}

func safeName(raw string) string {
	name := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	runes := []rune(name)
	if len(runes) > 180 {
		name = string(runes[:180])
	}
	return name
}
