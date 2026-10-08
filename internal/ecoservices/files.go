package ecoservices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

const MaxFileBytes = 10 << 20

type File struct {
	ID             string `json:"id"`
	OrganizationID string `json:"-"`
	ParentID       string `json:"parentId"`
	ParentKind     string `json:"parentKind"`
	Filename       string `json:"filename"`
	ContentType    string `json:"contentType"`
	SizeBytes      int64  `json:"sizeBytes,string"`
	SHA256         string `json:"-"`
	ObjectKey      string `json:"-"`
	State          string `json:"state"`
}
type FileIntent struct {
	Scope       Scope
	Key         string
	File        File
	Fingerprint string
}
type FileRepository interface {
	CreateFileIntent(context.Context, FileIntent) (File, error)
	ConfirmFile(context.Context, FileIntent) (File, error)
	ReadFile(context.Context, Scope, string) (File, error)
}
type ObjectInspection struct {
	Exists              bool
	ContentType, SHA256 string
	SizeBytes           int64
}
type PrivateObjectStore interface {
	Inspect(context.Context, File) (ObjectInspection, error)
	PutImmutable(context.Context, File, []byte) error
	ReadBounded(context.Context, File) ([]byte, error)
}
type FileService struct {
	repo    FileRepository
	objects PrivateObjectStore
}

func NewFileService(repo FileRepository, objects PrivateObjectStore) (*FileService, error) {
	if repo == nil || objects == nil {
		return nil, ErrUnavailable
	}
	return &FileService{repo: repo, objects: objects}, nil
}
func FileDigest(data []byte) string { v := sha256.Sum256(data); return hex.EncodeToString(v[:]) }
func (s *FileService) Upload(ctx context.Context, scope Scope, key, parentKind, parentID, filename string, data []byte) (File, error) {
	if scope.Platform || !validText(scope.OrganizationID, 128) || !validText(scope.ActorID, 256) || !ValidID(key) || parentID != "" && !ValidID(parentID) || (parentKind != "APPLICATION" && parentKind != "REQUEST") || !validText(filename, 256) || strings.ContainsAny(filename, "/\\\r\n") || len(data) == 0 || len(data) > MaxFileBytes {
		return File{}, ErrInvalid
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "application/pdf", "image/png", "image/jpeg", "text/plain; charset=utf-8":
	default:
		return File{}, ErrInvalid
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("ecoservices-file:"+scope.OrganizationID+":"+key)).String()
	in := FileIntent{Scope: scope, Key: key, File: File{ID: id, OrganizationID: scope.OrganizationID, ParentKind: parentKind, ParentID: parentID, Filename: filename, ContentType: contentType, SizeBytes: int64(len(data)), SHA256: FileDigest(data), ObjectKey: "ecoservices/files/" + id, State: "PENDING"}}
	in.Fingerprint = Fingerprint([]any{scope, key, parentKind, parentID, filename, contentType, in.File.SizeBytes, in.File.SHA256})
	file, err := s.repo.CreateFileIntent(ctx, in)
	if err != nil {
		return File{}, err
	}
	if file.State == "CONFIRMED" {
		return file, nil
	}
	inspection, err := s.objects.Inspect(ctx, in.File)
	if err != nil {
		return file, ErrUnavailable
	}
	if !inspection.Exists {
		if err := s.objects.PutImmutable(ctx, in.File, data); err != nil {
			return file, ErrUnavailable
		}
		inspection, err = s.objects.Inspect(ctx, in.File)
		if err != nil {
			return file, ErrUnavailable
		}
	}
	if !inspection.Exists || inspection.SHA256 != in.File.SHA256 || inspection.SizeBytes != in.File.SizeBytes || inspection.ContentType != in.File.ContentType {
		return file, ErrConflict
	}
	return s.repo.ConfirmFile(ctx, in)
}
func (s *FileService) Download(ctx context.Context, scope Scope, id string) (File, []byte, error) {
	return s.download(ctx, scope, id, "")
}
func (s *FileService) DownloadForKind(ctx context.Context, scope Scope, id, kind string) (File, []byte, error) {
	if kind != "APPLICATION" && kind != "REQUEST" && !(kind == "" && scope.Platform) {
		return File{}, nil, ErrForbidden
	}
	return s.download(ctx, scope, id, kind)
}
func (s *FileService) download(ctx context.Context, scope Scope, id, kind string) (File, []byte, error) {
	if !ValidID(id) || !validText(scope.ActorID, 256) || !scope.Platform && !validText(scope.OrganizationID, 128) {
		return File{}, nil, ErrInvalid
	}
	file, err := s.repo.ReadFile(ctx, scope, id)
	if err != nil {
		return File{}, nil, err
	}
	if file.State != "CONFIRMED" {
		return file, nil, ErrConflict
	}
	if kind != "" && file.ParentKind != kind {
		return File{}, nil, ErrForbidden
	}
	data, err := s.objects.ReadBounded(ctx, file)
	if err != nil {
		return file, nil, err
	}
	if int64(len(data)) != file.SizeBytes || FileDigest(data) != file.SHA256 {
		return file, nil, ErrConflict
	}
	return file, data, nil
}
