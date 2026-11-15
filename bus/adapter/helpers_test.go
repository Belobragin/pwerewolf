package adapter

import (
	"context"
	stdimage "image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	cfgstorage "github.com/belobragin/werewolf/bus/internal/config/storage"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/belobragin/werewolf/bus/usecase/local"
	"github.com/google/uuid"
)

type stubDb struct {
	database.ProjectDbaseI
	uploadErr     error
	uploaded      *picture_model.UploadReply
	downloadReply *picture_model.DownloadObjectReply
	downloadErr   error
	imageName     string
	imageNameErr  error
}

func (s *stubDb) DbUploadImage(_ context.Context, r *picture_model.UploadReply) error {
	s.uploaded = r
	return s.uploadErr
}
func (s *stubDb) DbDownloadImage(context.Context, uuid.UUID) (*picture_model.DownloadObjectReply, error) {
	return s.downloadReply, s.downloadErr
}
func (s *stubDb) DbGetImageName(context.Context, uuid.UUID) (string, error) {
	return s.imageName, s.imageNameErr
}

// stubBroker records what is published instead of using NATS.
type stubBroker struct {
	abroker.AllBrokerI
	published [][]byte
	err       error
}

func (b *stubBroker) GetPublishSubject() string { return "test.subject" }
func (b *stubBroker) PublishEvent(data []byte) error {
	if b.err != nil {
		return b.err
	}
	b.published = append(b.published, data)
	return nil
}

// newTestStorage makes the local storage in a temp project folder: test-image/ is the upload source,
// store/{image,adversarial,filter} are the storage folders, download/ is the downloads folder.
func newTestStorage(t *testing.T, maxImageSize string) (storage.ProjectStorageI, string) {
	t.Helper()
	proj := t.TempDir()
	for _, d := range []string{"test-image", "store/image", "store/adversarial", "store/filter"} {
		if err := os.MkdirAll(filepath.Join(proj, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := local.NewLocalStorage(&cfgstorage.StorageLocalConfig{
		ProjectFolder:           proj,
		UploadSourceFolder:      "test-image",
		MaxImageSize:            maxImageSize,
		ImageLocalStorage:       filepath.Join(proj, "store/image"),
		AdversarialLocalStorage: filepath.Join(proj, "store/adversarial"),
		FilterLocalStorage:      filepath.Join(proj, "store/filter"),
		DownloadsLocalFolder:    filepath.Join(proj, "download"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, proj
}

// writePNG writes a width x height png; noisy pixels make the file big enough for the size limit tests.
func writePNG(t *testing.T, path string, width, height int, noisy bool) {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, width, height))
	rnd := rand.New(rand.NewSource(1))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			if noisy {
				img.Set(x, y, color.RGBA{uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), 255})
			} else {
				img.Set(x, y, color.RGBA{10, 20, 30, 255})
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func newTxDb(t *testing.T) *local.DbRealizationLocal {
	t.Helper()
	d, err := local.BuildGormDbS()
	if err != nil {
		t.Skipf("database is not available: %v", err)
	}
	tx := d.Db.DB.Begin()
	if tx.Error != nil {
		t.Skipf("database is not available: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	d.Db.DB = tx
	return d
}
