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
	return append([]byte(nil), o.files[f.ObjectKey]...), nil
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
