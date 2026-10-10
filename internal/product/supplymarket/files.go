package supplymarket

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"task-processor/internal/product/collection"
	"time"
)

const MaxQualificationBytes = 20 << 20

type PrivateFile struct {
	ID          string           `json:"id"`
	Owner       collection.Scope `json:"-"`
	ObjectKey   string           `json:"-"`
	ContentType string           `json:"contentType"`
	SHA256      string           `json:"sha256"`
	Size        int64            `json:"size"`
	CreatedAt   time.Time        `json:"createdAt"`
}
type PrivateObject struct {
	Exists              bool
	SHA256, ContentType string
	Size                int64
	Data                []byte
}

func PrivateObjectKey(scope collection.Scope, id, hash string) string {
	return "supply-market/qualifications/" + collection.Digest([]string{scope.OrganizationID, scope.ActorID, scope.MemberID}) + "/" + id + "/" + hash
}
func (f PrivateFile) Validate() error {
	decoded, err := hex.DecodeString(f.SHA256)
	if err != nil || len(decoded) != 32 || strings.ToLower(f.SHA256) != f.SHA256 || f.Owner.Validate() != nil || !collection.ValidID(f.ID) || f.Size < 1 || f.Size > MaxQualificationBytes || f.ContentType != "image/png" && f.ContentType != "image/jpeg" && f.ContentType != "application/pdf" || f.ObjectKey != PrivateObjectKey(f.Owner, f.ID, f.SHA256) {
		return ErrInvalid
	}
	return nil
}

type PrivateFileStorage interface {
	PutImmutable(context.Context, PrivateFile, []byte) error
	Inspect(context.Context, PrivateFile) (PrivateObject, error)
	ReadBounded(context.Context, PrivateFile) ([]byte, error)
}
type FileRepository interface {
	SaveUpload(context.Context, PrivateFile, Guard) (PrivateFile, error)
	ReadUpload(context.Context, collection.Scope, string) (PrivateFile, error)
	ReadAttachedFile(context.Context, collection.Scope, string, string, string) (PrivateFile, error)
}
type FileService struct {
	auth       Authorizer
	storage    PrivateFileStorage
	repository FileRepository
}

func NewFileService(auth Authorizer, storage PrivateFileStorage, repository FileRepository) (*FileService, error) {
	if auth == nil || storage == nil || repository == nil {
		return nil, ErrUnavailable
	}
	return &FileService{auth, storage, repository}, nil
}
func inspectQualification(contentType string, data []byte) error {
	if len(data) < 1 || len(data) > MaxQualificationBytes {
		return ErrInvalid
	}
	if contentType == "application/pdf" {
		if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(data), []byte("%%EOF")) {
			return ErrInvalid
		}
		return nil
	}
	if contentType != "image/png" && contentType != "image/jpeg" {
		return ErrInvalid
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 10000 || config.Height > 10000 || int64(config.Width)*int64(config.Height) > 40_000_000 || contentType == "image/png" && format != "png" || contentType == "image/jpeg" && format != "jpeg" {
		return ErrInvalid
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return ErrInvalid
	}
	return nil
}
func (s *FileService) Upload(ctx context.Context, key, contentType string, data []byte) (PrivateFile, error) {
	if ctx == nil || s == nil {
		return PrivateFile{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !collection.ValidID(key) || inspectQualification(contentType, data) != nil {
		return PrivateFile{}, ErrInvalid
	}
	scope, err := s.auth.Authorize(ctx, PermissionApply)
	if err != nil || scope.Validate() != nil {
		return PrivateFile{}, ErrForbidden
	}
	sum := sha256.Sum256(data)
	file := PrivateFile{ID: collection.StableID(scope.OrganizationID, scope.ActorID, "qualification", key), Owner: scope, ContentType: contentType, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	file.ObjectKey = PrivateObjectKey(scope, file.ID, file.SHA256)
	existing, readErr := s.repository.ReadUpload(ctx, scope, file.ID)
	if readErr == nil {
		if existing.Validate() != nil || existing.SHA256 != file.SHA256 || existing.Size != file.Size || existing.ContentType != file.ContentType {
			return PrivateFile{}, ErrConflict
		}
		file = existing
	} else if !errors.Is(readErr, ErrNotFound) {
		return PrivateFile{}, readErr
	} else {
		_ = s.storage.PutImmutable(ctx, file, data)
	}
	object, err := s.storage.Inspect(ctx, file)
	if err != nil || !object.Exists || object.SHA256 != file.SHA256 || object.Size != file.Size || object.ContentType != file.ContentType {
		return PrivateFile{}, ErrUnknown
	}
	guard := func(ctx context.Context) error {
		current, err := s.auth.Authorize(ctx, PermissionApply)
		if err != nil || current != scope {
			return ErrForbidden
		}
		return ctx.Err()
	}
	return s.repository.SaveUpload(ctx, file, guard)
}
func (s *FileService) Download(ctx context.Context, recordID, fileID string, platform bool) (PrivateFile, []byte, error) {
	if ctx == nil || s == nil {
		return PrivateFile{}, nil, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !collection.ValidID(recordID) || !collection.ValidID(fileID) {
		return PrivateFile{}, nil, ErrInvalid
	}
	var scope collection.Scope
	var actor string
	var err error
	if platform {
		actor, err = s.auth.AuthorizePlatform(ctx)
	} else {
		scope, err = s.auth.Authorize(ctx, PermissionRead)
	}
	if err != nil {
		return PrivateFile{}, nil, ErrForbidden
	}
	file, err := s.repository.ReadAttachedFile(ctx, scope, actor, recordID, fileID)
	if err != nil {
		return PrivateFile{}, nil, err
	}
	if file.Validate() != nil {
		return PrivateFile{}, nil, ErrUnavailable
	}
	data, err := s.storage.ReadBounded(ctx, file)
	if err != nil {
		return PrivateFile{}, nil, ErrUnavailable
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != file.Size || hex.EncodeToString(sum[:]) != file.SHA256 {
		return PrivateFile{}, nil, ErrConflict
	}
	if platform {
		current, e := s.auth.AuthorizePlatform(ctx)
		if e != nil || current != actor {
			return PrivateFile{}, nil, ErrForbidden
		}
	} else {
		current, e := s.auth.Authorize(ctx, PermissionRead)
		if e != nil || current != scope {
			return PrivateFile{}, nil, ErrForbidden
		}
	}
	return file, data, ctx.Err()
}
