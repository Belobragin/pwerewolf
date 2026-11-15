package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/google/uuid"
)

func TestDownloadImage(t *testing.T) {
	st, proj := newTestStorage(t, "4Mb")
	for _, kind := range []string{"image", "adversarial", "filter"} {
		if err := os.WriteFile(filepath.Join(proj, "store", kind, kind+".jpg"), []byte("data-"+kind), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	db := &stubDb{}
	l := &ApiClientRealization{Db: db, Storage: st}
	download := func() (any, error) { return l.DownloadImage(context.Background(), uuid.New()) }

	replies := map[string]*picture_model.DownloadObjectReply{
		"image":       {IfImage: true, ObjectName: "image.jpg"},
		"adversarial": {IfAdversarial: true, ObjectName: "adversarial.jpg"},
		"filter":      {IfFilter: true, ObjectName: "filter.jpg"},
	}
	for kind, reply := range replies {
		db.downloadReply = reply
		out, err := download()
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if out.(picture_model.DownloadObjectReply).Code != 200 {
			t.Errorf("%s: reply %+v", kind, out)
		}
		got, err := os.ReadFile(filepath.Join(proj, "download", reply.ObjectName))
		if err != nil || string(got) != "data-"+kind {
			t.Errorf("%s: downloaded %q, %v", kind, got, err)
		}
	}

	t.Run("a repeated download replaces the file and leaves no temp file", func(t *testing.T) {
		if _, err := download(); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(filepath.Join(proj, "download"))
		if err != nil || len(entries) != 3 {
			t.Errorf("download folder: %d entries, %v", len(entries), err)
		}
	})
	t.Run("file missing in the storage", func(t *testing.T) {
		db.downloadReply = &picture_model.DownloadObjectReply{IfImage: true, ObjectName: "gone.jpg"}
		if _, err := download(); !errors.Is(err, pdata.ErrNoStorageObject) || pdata.GetCodeFromErr(err) != 406 {
			t.Errorf("got %v", err)
		}
	})
	t.Run("a stored name can not leave the storage folder", func(t *testing.T) {
		db.downloadReply = &picture_model.DownloadObjectReply{IfImage: true, ObjectName: "../../etc/passwd"}
		if _, err := download(); !errors.Is(err, pdata.ErrNoStorageObject) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("no database record", func(t *testing.T) {
		db.downloadReply, db.downloadErr = nil, pdata.ErrNoObjectId
		if _, err := download(); !errors.Is(err, pdata.ErrNoObjectId) {
			t.Errorf("got %v", err)
		}
	})
}
