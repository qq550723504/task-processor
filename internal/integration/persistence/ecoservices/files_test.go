package ecoservices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
	"testing"
)

type objectFixture struct {
	files        map[string][]byte
	metadata     map[string]e.File
	lostResponse bool
	puts         int
	reads        int
}

func (o *objectFixture) Inspect(_ context.Context, f e.File) (e.ObjectInspection, error) {
	metadata, ok := o.metadata[f.ObjectKey]
	return e.ObjectInspection{Exists: ok, SHA256: metadata.SHA256, ContentType: metadata.ContentType, SizeBytes: metadata.SizeBytes}, nil
}
func (o *objectFixture) PutImmutable(_ context.Context, f e.File, data []byte) error {
	o.puts++
	if _, ok := o.files[f.ObjectKey]; ok {
		return e.ErrConflict
	}
	o.files[f.ObjectKey] = append([]byte(nil), data...)
	o.metadata[f.ObjectKey] = f
	if o.lostResponse {
		o.lostResponse = false
		return e.ErrUnavailable
	}
	return nil
}
func (o *objectFixture) ReadBounded(_ context.Context, f e.File) ([]byte, error) {
	o.reads++
	return append([]byte(nil), o.files[f.ObjectKey]...), nil
}
func TestPrivateApplicationFilesCannotUseRequestDownloadAction(t *testing.T) {
	r, _ := fixture(t)
	o := &objectFixture{files: map[string][]byte{}, metadata: map[string]e.File{}}
	files, err := e.NewFileService(r, o)
	if err != nil {
		t.Fatal(err)
	}
	scope := e.Scope{OrganizationID: "org", ActorID: "actor"}
	ctx := context.Background()
	app, err := files.Upload(ctx, scope, uuid.NewString(), "APPLICATION", "", "identity.txt", []byte("private merchant identity"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := files.DownloadForKind(ctx, scope, app.ID, "REQUEST"); !errors.Is(err, e.ErrForbidden) || o.reads != 0 {
		t.Fatal("generic request action fetched merchant PII object", err)
	}
	if _, _, err := files.DownloadForKind(ctx, scope, app.ID, "APPLICATION"); err != nil || o.reads != 1 {
		t.Fatal("join action cannot read its original proof", err)
	}
	req, err := files.Upload(ctx, scope, uuid.NewString(), "REQUEST", "", "request.txt", []byte("private request"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := files.DownloadForKind(ctx, scope, req.ID, "APPLICATION"); !errors.Is(err, e.ErrForbidden) || o.reads != 1 {
		t.Fatal("join route bypassed original request action", err)
	}
}
func TestPrivateUploadLostResponseAndSameKeyChangedContent(t *testing.T) {
	repo, _ := fixture(t)
	objects := &objectFixture{files: map[string][]byte{}, metadata: map[string]e.File{}, lostResponse: true}
	service, err := e.NewFileService(repo, objects)
	if err != nil {
		t.Fatal(err)
	}
	scope := e.Scope{OrganizationID: "buyer", ActorID: "b"}
	key := uuid.NewString()
	ctx := context.Background()
	pending, err := service.Upload(ctx, scope, key, "APPLICATION", "", "qualification.txt", []byte("hello"))
	if !errors.Is(err, e.ErrUnavailable) || pending.State != "PENDING" {
		t.Fatalf("lost response fabricated confirmed attachment: %+v %v", pending, err)
	}
	confirmed, err := service.Upload(ctx, scope, key, "APPLICATION", "", "qualification.txt", []byte("hello"))
	if err != nil || confirmed.ID != pending.ID || confirmed.State != "CONFIRMED" || objects.puts != 1 {
		t.Fatalf("original immutable upload not recovered: %+v puts=%d %v", confirmed, objects.puts, err)
	}
	if _, err := service.Upload(ctx, scope, key, "APPLICATION", "", "qualification.txt", []byte("other")); !errors.Is(err, e.ErrConflict) {
		t.Fatalf("same-key different file body accepted: %v", err)
	}
	if _, _, err := service.Download(ctx, e.Scope{OrganizationID: "stranger", ActorID: "x"}, confirmed.ID); !errors.Is(err, e.ErrNotFound) {
		t.Fatalf("foreign private file readable: %v", err)
	}
	objects.files[confirmed.ObjectKey] = []byte("other")
	if _, _, err := service.Download(ctx, scope, confirmed.ID); !errors.Is(err, e.ErrConflict) {
		t.Fatalf("corrupt object readable: %v", err)
	}
}
