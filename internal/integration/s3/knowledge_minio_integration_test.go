//go:build integration

package s3

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	dockerclient "github.com/moby/moby/client"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"task-processor/internal/knowledge"
)

// Reuse the shipped storage bootstrap and IAM policy, with synthetic test-only
// credentials. This verifies the actual runtime user, never the root user.
func TestKnowledgeMinIORuntimeMissingObjectAndImmutableRoundTrip(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	build := func(target string) tc.FromDockerfile {
		return tc.FromDockerfile{Context: filepath.Join(root, "deployments/docker/account-compose"), Dockerfile: "Dockerfile.knowledge-storage", BuildOptionsModifier: func(options *dockerclient.ImageBuildOptions) { options.Target = target }}
	}
	net, err := network.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = net.Remove(context.Background()) })
	server, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: tc.ContainerRequest{
		FromDockerfile: build("server"),
		Env:            map[string]string{"MINIO_ROOT_USER": "knowledge_local_root", "MINIO_ROOT_PASSWORD": "isolated-minio-root-password"},
		Cmd:            []string{"server", "/data", "--address", ":9000"}, ExposedPorts: []string{"9000/tcp"}, Networks: []string{net.Name}, NetworkAliases: map[string][]string{net.Name: {"knowledge-objects"}},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithStartupTimeout(60 * time.Second),
	}, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Terminate(context.Background()) })
	files := []tc.ContainerFile{
		{HostFilePath: filepath.Join(root, "deployments/docker/account-compose/knowledge-storage-init.sh"), ContainerFilePath: "/init/knowledge-storage-init.sh", FileMode: 0600},
		{HostFilePath: filepath.Join(root, "deployments/docker/account-compose/knowledge-storage-policy.json"), ContainerFilePath: "/init/policy.json", FileMode: 0600},
		{Reader: strings.NewReader("isolated-minio-root-password"), ContainerFilePath: "/private/root-password", FileMode: 0600},
		{Reader: strings.NewReader("knowledge-runtime-test"), ContainerFilePath: "/private/access-key", FileMode: 0600},
		{Reader: strings.NewReader("isolated-minio-runtime-password"), ContainerFilePath: "/private/secret-key", FileMode: 0600},
	}
	setup, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: tc.ContainerRequest{
		FromDockerfile: build("client"),
		Entrypoint:     []string{"/bin/sh", "/init/knowledge-storage-init.sh"}, Files: files, Networks: []string{net.Name}, WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
	}, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = setup.Terminate(context.Background()) })
	state, err := setup.State(ctx)
	if err != nil || state.ExitCode != 0 {
		t.Fatalf("storage init failed: %v", err)
	}
	host, _ := server.Host(ctx)
	port, _ := server.MappedPort(ctx, "9000/tcp")
	client, err := NewKnowledgeClient(ClientConfig{Region: "us-east-1", Endpoint: "http://" + host + ":" + port.Port(), AccessKeyID: "knowledge-runtime-test", SecretAccessKey: "isolated-minio-runtime-password", UsePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	uploader := mustNewUploader(t, client, UploaderOptions{Bucket: "knowledge", ArtifactCapabilities: ArtifactStorageCapabilities{Mode: ArtifactStorageModeAWS}})
	store, err := NewKnowledgeStore(uploader)
	if err != nil {
		t.Fatal(err)
	}
	object := knowledge.Object{Key: "knowledge/test-revision/" + knowledge.Digest([]byte("hello")), SHA256: knowledge.Digest([]byte("hello")), ContentType: "text/plain", SizeBytes: 5, Data: []byte("hello")}
	missing, err := store.Inspect(ctx, object)
	if err != nil || missing.Exists {
		t.Fatalf("runtime must distinguish missing object before first upload: %+v %v", missing, err)
	}
	if err = store.PutImmutable(ctx, object); err != nil {
		t.Fatal(err)
	}
	if err = store.PutImmutable(ctx, object); err == nil {
		t.Fatal("immutable object overwritten")
	}
	got, err := store.Inspect(ctx, object)
	if err != nil || !got.Exists || got.SHA256 != object.SHA256 || got.SizeBytes != 5 {
		t.Fatalf("inspect saved object: %+v %v", got, err)
	}
	data, err := store.ReadBounded(ctx, object, 5)
	if err != nil || string(data) != "hello" {
		t.Fatalf("read saved object: %q %v", data, err)
	}
	outside := object
	outside.Key = "outside/test"
	if err = store.PutImmutable(ctx, outside); err == nil {
		t.Fatal("runtime wrote outside Knowledge prefix")
	}
	if _, err = client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String("knowledge"), Key: aws.String(object.Key)}); err == nil {
		t.Fatal("runtime deleted a Knowledge object")
	}
}
