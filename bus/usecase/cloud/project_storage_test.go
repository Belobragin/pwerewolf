package cloud

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	stt "github.com/belobragin/werewolf/bus/pkg/storage"
)

type apiErr string

func (e apiErr) Error() string                 { return string(e) }
func (e apiErr) ErrorCode() string             { return string(e) }
func (e apiErr) ErrorMessage() string          { return string(e) }
func (e apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }

// fakeS3 is an in-memory bucket.
type fakeS3 struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func newFakeS3(objs map[string]string) *fakeS3 {
	f := &fakeS3{objs: map[string][]byte{}}
	for k, v := range objs {
		f.objs[k] = []byte(v)
	}
	return f
}

func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objs[aws.ToString(in.Key)]
	if !ok {
		return nil, apiErr("NoSuchKey")
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func (f *fakeS3) HeadObject(_ context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objs[aws.ToString(in.Key)]; !ok {
		return nil, apiErr("NotFound")
	}
	return &s3.HeadObjectOutput{}, nil
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := aws.ToString(in.Key)
	if _, ok := f.objs[key]; ok && aws.ToString(in.IfNoneMatch) == "*" {
		return nil, apiErr("PreconditionFailed")
	}
	b, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.objs[key] = b
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3) CopyObject(_ context.Context, in *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// CopySource is "<bucket>/<key>", url escaped
	src := strings.ReplaceAll(aws.ToString(in.CopySource), "%2F", "/")
	b, ok := f.objs[strings.TrimPrefix(src, testBucket+"/")]
	if !ok {
		return nil, apiErr("NoSuchKey")
	}
	f.objs[aws.ToString(in.Key)] = b
	return &s3.CopyObjectOutput{}, nil
}

func (f *fakeS3) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objs, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

const testBucket = "werewolf-test-bucket"

type testConf struct{}

func (testConf) ValidateStorageConfig() error  { return nil }
func (testConf) GetUploadSourceFolder() string { return "test-image" }
func (testConf) GetImageStorage() string       { return "storage/image" }
func (testConf) GetAdversarialStorage() string { return "storage/adversarial" }
func (testConf) GetFilterStorage() string      { return "storage/filter" }
func (testConf) GetDownloadsFolder() string    { return "storage/download" }
func (testConf) GetMaxImageSize() string       { return "4Mb" }

func newTestStorage(t *testing.T, objs map[string]string) (*StorageRealizationCloud, *fakeS3) {
	t.Helper()
	fake := newFakeS3(objs)
	st, err := newCloudStorage(testConf{}, fake, testBucket, "us-east-2")
	if err != nil {
		t.Fatal(err)
	}
	st.stagingDir = t.TempDir()
	return st, fake
}

func TestNewCloudStorageFields(t *testing.T) {
	st, _ := newTestStorage(t, nil)
	if st.GetS3BucketName() != testBucket {
		t.Fatalf("bucket name: %s", st.GetS3BucketName())
	}
	if want := "https://werewolf-test-bucket.s3.us-east-2.amazonaws.com"; st.GetS3BusketUrl() != want {
		t.Fatalf("bucket url: %s, want %s", st.GetS3BusketUrl(), want)
	}
	if st.GetImageCloudStorage() != "storage/image" || st.GetMaxImageSize() != "4Mb" {
		t.Fatalf("fields are not initialized from the config")
	}
}

func TestResolveSource(t *testing.T) {
	st, _ := newTestStorage(t, map[string]string{
		"test-image/a.jpg":    "image-a",
		"storage/image/b.jpg": "secret",
	})

	for _, p := range []string{"", "a.jpg", "/test-image/a.jpg", "test-image/../storage/image/b.jpg", "storage/image/b.jpg", "test-image/", "test-image"} {
		if _, err := st.ResolveSource(p); !errors.Is(err, pdata.ErrInvalidFilePath) {
			t.Errorf("%q: want ErrInvalidFilePath, got %v", p, err)
		}
	}
	if _, err := st.ResolveSource("test-image/none.jpg"); !errors.Is(err, pdata.ErrNoSourceFile) {
		t.Fatalf("missing source: %v", err)
	}

	src, err := st.ResolveSource("test-image/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(src) != "a.jpg" {
		t.Fatalf("staged file must keep the object name, got %s", src)
	}
	if b, _ := os.ReadFile(src); string(b) != "image-a" {
		t.Fatalf("staged content: %q", b)
	}
}

func TestStoreImage(t *testing.T) {
	st, fake := newTestStorage(t, map[string]string{"test-image/a.jpg": "image-a"})

	src, err := st.ResolveSource("test-image/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := st.ImageExists("a.jpg"); err != nil || exists {
		t.Fatalf("ImageExists before store: %v %v", exists, err)
	}
	if err = st.StoreImage(src, "a.jpg"); err != nil {
		t.Fatal(err)
	}
	if string(fake.objs["storage/image/a.jpg"]) != "image-a" {
		t.Fatalf("image is not stored under the image prefix")
	}
	if _, err = os.Stat(filepath.Dir(src)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged source must be removed after the store: %v", err)
	}
	if exists, err := st.ImageExists("a.jpg"); err != nil || !exists {
		t.Fatalf("ImageExists after store: %v %v", exists, err)
	}

	src, err = st.ResolveSource("test-image/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.StoreImage(src, "a.jpg"); !errors.Is(err, pdata.ErrExistImageFile) {
		t.Fatalf("repeated store: want ErrExistImageFile, got %v", err)
	}
	if string(fake.objs["storage/image/a.jpg"]) != "image-a" {
		t.Fatalf("existing image must stay untouched")
	}
}

func TestStoreImageKeepsForeignSource(t *testing.T) {
	st, _ := newTestStorage(t, nil)
	src := filepath.Join(t.TempDir(), "x.jpg")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.StoreImage(src, "x.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("a file outside of the staging folder must not be removed: %v", err)
	}
}

func TestRemove(t *testing.T) {
	st, fake := newTestStorage(t, map[string]string{
		"storage/image/a.jpg":       "i",
		"storage/adversarial/a.jpg": "a",
		"storage/filter/a.jpg":      "f",
	})

	if err := st.RemoveImage("a.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveObject(stt.KindAdversarial, "../../storage/adversarial/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveObject(stt.KindFilter, "none.jpg"); err != nil {
		t.Fatalf("a missing object is not an error: %v", err)
	}
	if err := st.RemoveObject("bogus", "a.jpg"); !errors.Is(err, pdata.ErrInvalidObjectKind) {
		t.Fatalf("invalid kind: %v", err)
	}
	if _, ok := fake.objs["storage/image/a.jpg"]; ok {
		t.Fatal("image is not removed")
	}
	if _, ok := fake.objs["storage/adversarial/a.jpg"]; ok {
		t.Fatal("adversarial is not removed")
	}
	if _, ok := fake.objs["storage/filter/a.jpg"]; !ok {
		t.Fatal("filter object must stay")
	}
}

func TestDownloadObject(t *testing.T) {
	st, fake := newTestStorage(t, map[string]string{"storage/filter/f.jpg": "v1"})

	if err := st.DownloadObject(stt.KindFilter, "f.jpg"); err != nil {
		t.Fatal(err)
	}
	if string(fake.objs["storage/download/f.jpg"]) != "v1" {
		t.Fatalf("object is not copied to the downloads folder")
	}
	fake.objs["storage/filter/f.jpg"] = []byte("v2")
	if err := st.DownloadObject(stt.KindFilter, "f.jpg"); err != nil {
		t.Fatal(err)
	}
	if string(fake.objs["storage/download/f.jpg"]) != "v2" {
		t.Fatalf("repeated download must replace the earlier copy")
	}
	if err := st.DownloadObject(stt.KindImage, "f.jpg"); !errors.Is(err, pdata.ErrNoStorageObject) {
		t.Fatalf("missing object: %v", err)
	}
	if err := st.DownloadObject("bogus", "f.jpg"); !errors.Is(err, pdata.ErrInvalidObjectKind) {
		t.Fatalf("invalid kind: %v", err)
	}
}

func TestSweepStaged(t *testing.T) {
	st, _ := newTestStorage(t, nil)
	old, fresh := filepath.Join(st.stagingDir, "src-old"), filepath.Join(st.stagingDir, "src-fresh")
	for _, d := range []string{old, fresh} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * stagedTTL)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	st.sweepStaged()

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale staged source must be removed: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh staged source must stay: %v", err)
	}
}

// the live test is read-only; it is skipped without aws access to the stack.
func TestNewCloudStorageLive(t *testing.T) {
	stack, resource := os.Getenv("AWS_RESOURCE_CFN_STACK_NAME"), os.Getenv("AWS_S3_BUCKET_RESOURCE")
	if stack == "" || resource == "" {
		t.Skip("AWS_RESOURCE_CFN_STACK_NAME and AWS_S3_BUCKET_RESOURCE are not set")
	}
	st, err := NewCloudStorage(liveConf{testConf{}, stack, resource})
	if err != nil {
		t.Skipf("aws stack is not available: %v", err)
	}
	cs := st.(*StorageRealizationCloud)
	if cs.GetS3BucketName() == "" || !strings.Contains(cs.GetS3BusketUrl(), cs.GetS3BucketName()) {
		t.Fatalf("bucket: %q url: %q", cs.GetS3BucketName(), cs.GetS3BusketUrl())
	}
}

type liveConf struct {
	testConf
	stack, resource string
}

func (c liveConf) GetStackName() string      { return c.stack }
func (c liveConf) GetBucketResource() string { return c.resource }
