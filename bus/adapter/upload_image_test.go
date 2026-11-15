package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
)

func TestUploadImage(t *testing.T) {
	st, proj := newTestStorage(t, "4Mb")
	writePNG(t, filepath.Join(proj, "test-image/pic.png"), 4, 3, false)
	if err := os.WriteFile(filepath.Join(proj, "test-image/note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePNG(t, filepath.Join(proj, "secret.png"), 4, 3, false)
	if err := os.Symlink(filepath.Join(proj, "secret.png"), filepath.Join(proj, "test-image/link.png")); err != nil {
		t.Fatal(err)
	}

	db := &stubDb{}
	l := &ApiClientRealization{Db: db, Storage: st}
	upload := func(path, description string) (any, error) {
		return l.UploadImage(context.Background(), &picture_model.UploadImage{FilePath: path, FileDescription: description})
	}

	out, err := upload("test-image/pic.png", "a test picture")
	if err != nil {
		t.Fatal(err)
	}
	reply := out.(picture_model.UploadReply)
	if reply.ImageName != "pic.png" || reply.MimeType != "image/png" || reply.ImageWidth != 4 || reply.ImageHeight != 3 ||
		reply.ImageDescription != "a test picture" {
		t.Errorf("reply: %+v", reply)
	}
	if db.uploaded == nil || db.uploaded.ImageID != reply.ImageID {
		t.Error("the reply is not saved to the database")
	}
	stored := filepath.Join(proj, "store/image/pic.png")
	if _, err := os.Stat(stored); err != nil {
		t.Errorf("image is not stored: %v", err)
	}

	cases := map[string]error{
		"test-image/pic.png":       pdata.ErrExistImageFile,
		"test-image/nope.png":      pdata.ErrNoSourceFile,
		"nodir/x.png":              pdata.ErrNoSourceFile,
		"/etc/passwd":              pdata.ErrInvalidFilePath,
		"../etc/passwd":            pdata.ErrInvalidFilePath,
		"test-image/../secret.png": pdata.ErrInvalidFilePath,
		"secret.png":               pdata.ErrInvalidFilePath,
		"test-image/link.png":      pdata.ErrInvalidFilePath,
		"test-image":               pdata.ErrInvalidFilePath,
		"":                         pdata.ErrInvalidFilePath,
		"test-image/note.txt":      pdata.ErrInvalidImage,
	}
	for path, want := range cases {
		if _, err := upload(path, ""); !errors.Is(err, want) {
			t.Errorf("%q: got %v, want %v", path, err, want)
		}
	}

	t.Run("database failure removes the stored file", func(t *testing.T) {
		if err := os.Remove(stored); err != nil {
			t.Fatal(err)
		}
		db.uploadErr = pdata.ErrExistObjectName
		defer func() { db.uploadErr = nil }()
		if _, err := upload("test-image/pic.png", ""); !errors.Is(err, pdata.ErrExistObjectName) {
			t.Errorf("got %v", err)
		}
		if _, err := os.Stat(stored); err == nil {
			t.Error("the file stays in the storage")
		}
	})
}

func TestUploadImageTooLarge(t *testing.T) {
	st, proj := newTestStorage(t, "1Kb")
	writePNG(t, filepath.Join(proj, "test-image/big.png"), 100, 100, true)

	l := &ApiClientRealization{Db: &stubDb{}, Storage: st}
	_, err := l.UploadImage(context.Background(), &picture_model.UploadImage{FilePath: "test-image/big.png"})
	if !errors.Is(err, pdata.ErrImageTooLarge) {
		t.Errorf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, "store/image/big.png")); err == nil {
		t.Error("a too large image is stored")
	}
}
