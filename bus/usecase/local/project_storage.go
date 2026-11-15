package local

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/belobragin/werewolf/bus/internal/config"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	stt "github.com/belobragin/werewolf/bus/pkg/storage"
)

type StorageRealizationLocal struct {
	projectFolder           string
	uploadSourceFolder      string
	imageLocalStorage       string
	adversarialLocalStorage string
	filterLocalStorage      string
	downloadsLocalFolder    string
	maxImageSize            string
}

func (st *StorageRealizationLocal) GetMaxImageSize() string {
	return st.maxImageSize
}

func NewLocalStorage(conf config.LocalStorageConfigI) (stt.ProjectStorageI, error) {
	var (
		s = new(StorageRealizationLocal)
	)
	s.projectFolder = conf.GetProjectFolder()
	s.uploadSourceFolder = conf.GetUploadSourceFolder()
	s.imageLocalStorage = conf.GetImageStorage()
	s.adversarialLocalStorage = conf.GetAdversarialStorage()
	s.filterLocalStorage = conf.GetFilterStorage()
	s.downloadsLocalFolder = conf.GetDownloadsFolder()
	s.maxImageSize = conf.GetMaxImageSize()

	return s, nil
}

// ResolveSource accepts only files inside $PROJECTFOLDER/$LOCAL_IMAGES_FOLDER.
func (st *StorageRealizationLocal) ResolveSource(relPath string) (string, error) {
	if relPath == "" || !filepath.IsLocal(relPath) {
		return "", pdata.ErrInvalidFilePath
	}
	root, err := filepath.EvalSymlinks(filepath.Join(st.projectFolder, st.uploadSourceFolder))
	if err != nil {
		return "", err
	}
	// resolving symlinks makes sure a link inside the images folder cannot point outside of it.
	src, err := filepath.EvalSymlinks(filepath.Join(st.projectFolder, relPath))
	if errors.Is(err, fs.ErrNotExist) {
		return "", pdata.ErrNoSourceFile
	}
	if err != nil {
		return "", err
	}
	if !isInside(root, src) {
		return "", pdata.ErrInvalidFilePath
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", pdata.ErrInvalidFilePath
	}
	return src, nil
}

func (st *StorageRealizationLocal) ImageExists(name string) (bool, error) {
	_, err := os.Lstat(st.imagePath(name))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// StoreImage copies src with exclusive create, so a concurrent upload of the same name fails with ErrExistImageFile.
func (st *StorageRealizationLocal) StoreImage(src, name string) error {
	dst := st.imagePath(name)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return pdata.ErrExistImageFile
	}
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err = out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

func (st *StorageRealizationLocal) RemoveImage(name string) error {
	return os.Remove(st.imagePath(name))
}

func (st *StorageRealizationLocal) RemoveObject(kind stt.ObjectKind, name string) error {
	dir, err := st.kindDir(kind)
	if err != nil {
		return err
	}
	if err = os.Remove(filepath.Join(dir, filepath.Base(name))); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (st *StorageRealizationLocal) kindDir(kind stt.ObjectKind) (string, error) {
	switch kind {
	case stt.KindImage:
		return st.imageLocalStorage, nil
	case stt.KindAdversarial:
		return st.adversarialLocalStorage, nil
	case stt.KindFilter:
		return st.filterLocalStorage, nil
	}
	return "", pdata.ErrInvalidObjectKind
}

// DownloadObject writes through a temp file and renames it, so a reader never sees a partial download.
func (st *StorageRealizationLocal) DownloadObject(kind stt.ObjectKind, name string) error {
	dir, err := st.kindDir(kind)
	if err != nil {
		return err
	}
	name = filepath.Base(name)

	in, err := os.Open(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return pdata.ErrNoStorageObject
	}
	if err != nil {
		return err
	}
	defer in.Close()

	if err = os.MkdirAll(st.downloadsLocalFolder, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(st.downloadsLocalFolder, ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(st.downloadsLocalFolder, name))
}

// imagePath keeps only the base name, so a name can never leave the image storage folder.
func (st *StorageRealizationLocal) imagePath(name string) string {
	return filepath.Join(st.imageLocalStorage, filepath.Base(name))
}

// isInside reports whether path is root itself or lies under it; both must be clean absolute paths.
func isInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
