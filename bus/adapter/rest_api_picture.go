package adapter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/belobragin/bvv_utils/goutils/image"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/google/uuid"
)

func (l *ApiClientRealization) UploadImage(ctx context.Context, p picture_model.UploadImageI) (any, error) {
	st := l.GetStorage()
	src, err := st.ResolveSource(p.GetImagePath())
	if err != nil {
		return nil, err
	}

	imageName := filepath.Base(src)
	exists, err := st.ImageExists(imageName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, pdata.ErrExistImageFile
	}

	imageSize, err := image.GetFileSize(src, st.GetMaxImageSize())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pdata.ErrImageTooLarge, err)
	}
	mimeType, err := detectImageMime(src)
	if err != nil {
		return nil, err
	}
	width, height, err := image.GetFileDim(src, st.GetMaxImageSize())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pdata.ErrInvalidImage, err)
	}

	output := picture_model.UploadReply{
		ImageID:          uuid.New(),
		ImageName:        imageName,
		ImageDescription: p.GetFileDescription(),
		MimeType:         mimeType,
		ImageSize:        imageSize,
		ImageHeight:      height,
		ImageWidth:       width,
		CreatedAt:        time.Now().Format(time.RFC3339Nano),
	}

	if err = st.StoreImage(src, imageName); err != nil {
		return nil, err
	}
	if err = l.GetDb().DbUploadImage(ctx, &output); err != nil {
		_ = st.RemoveImage(imageName)
		return nil, err
	}

	return any(output), nil
}

func detectImageMime(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := f.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	switch mt := http.DetectContentType(head[:n]); mt {
	case "image/jpeg", "image/png", "image/gif":
		return mt, nil
	default:
		return "", fmt.Errorf("%w: detected %s", pdata.ErrInvalidImage, mt)
	}
}

func (l *ApiClientRealization) DownloadImage(ctx context.Context, objectID uuid.UUID) (any, error) {
	output, err := l.GetDb().DbDownloadImage(ctx, objectID)
	if err != nil {
		return nil, err
	}

	kind := storage.KindImage
	switch {
	case output.IfAdversarial:
		kind = storage.KindAdversarial
	case output.IfFilter:
		kind = storage.KindFilter
	}
	if err = l.GetStorage().DownloadObject(kind, output.ObjectName); err != nil {
		return nil, err
	}

	output.Code = http.StatusOK

	return any(*output), nil
}

func (l *ApiClientRealization) DeleteImage(ctx context.Context, objectID uuid.UUID) (any, error) {
	db, st := l.GetDb(), l.GetStorage()

	objects, err := db.DbListCascadeObjects(ctx, objectID)
	if err != nil {
		return nil, err
	}

	// files go first: a record without its file is visible and a retry repairs it, an untracked file is not.
	for _, o := range objects {
		if err = st.RemoveObject(o.Kind, o.Name); err != nil {
			return nil, fmt.Errorf("%w: %v", pdata.ErrDeleteFile, err)
		}
	}
	if err = db.DbDeleteImage(ctx, objectID); err != nil && !errors.Is(err, pdata.ErrNoObjectId) {
		return nil, fmt.Errorf("%w: %v", pdata.ErrDeleteRecord, err)
	}

	return any(picture_model.DeleteObjectReply{
		AllObject: objects,
		DeletedAt: time.Now().Format(time.RFC3339Nano),
		Code:      http.StatusOK,
	}), nil
}
