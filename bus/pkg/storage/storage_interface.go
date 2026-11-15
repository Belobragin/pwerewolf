package storage

// ObjectKind is the storage folder of an object.
type ObjectKind string

const (
	KindImage       ObjectKind = "image"
	KindAdversarial ObjectKind = "adversarial"
	KindFilter      ObjectKind = "filter"
)

// Image names are plain file names, without any folder.
type ProjectStorageI interface {
	// ResolveSource validates the upload source path, relative to the project folder,
	// and returns the path of a readable local file.
	// Errors: pdata.ErrInvalidFilePath (400), pdata.ErrNoSourceFile (404)
	ResolveSource(relPath string) (string, error)
	// ImageExists reports whether the image is already in the storage.
	ImageExists(name string) (bool, error)
	// StoreImage puts the local file src to the storage under the name.
	// It fails with pdata.ErrExistImageFile if the name is taken, and leaves no partial image on failure.
	StoreImage(src, name string) error
	// RemoveImage deletes the image from the storage.
	RemoveImage(name string) error
	// DownloadObject copies the object from its storage folder to the downloads folder, keeping the file name.
	// A repeated download replaces the earlier copy. Fails with pdata.ErrNoStorageObject (406) if the object file is missing.
	DownloadObject(kind ObjectKind, name string) error
	// RemoveObject deletes the object file from its storage folder. A file that is already missing is not an error.
	RemoveObject(kind ObjectKind, name string) error
	// GetMaxImageSize returns the upload size limit like "4Mb".
	GetMaxImageSize() string
}
