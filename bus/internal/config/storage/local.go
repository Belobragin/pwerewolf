package storage

import (
	"errors"
)

type StorageLocalConfig struct {
	ProjectFolder           string `conf:"env:PROJECTFOLDER"`
	UploadSourceFolder      string `conf:"env:LOCAL_IMAGES_FOLDER"`
	ImageLocalStorage       string `conf:"env:IMAGE_LOCAL_STORAGE"`
	AdversarialLocalStorage string `conf:"env:ADVERSARIAL_LOCAL_STORAGE"`
	FilterLocalStorage      string `conf:"env:FILTER_LOCAL_STORAGE"`
	DownloadsLocalFolder    string `conf:"env:DOWNLOADS_LOCAL_FOLDER"`
	MaxImageSize            string `conf:"env:MAX_IMAGE_SIZE"`
}

func (s *StorageLocalConfig) ValidateStorageConfig() error {
	if len(s.ProjectFolder) == 0 || len(s.UploadSourceFolder) == 0 || len(s.MaxImageSize) == 0 ||
		len(s.ImageLocalStorage) == 0 || len(s.AdversarialLocalStorage) == 0 ||
		len(s.FilterLocalStorage) == 0 || len(s.DownloadsLocalFolder) == 0 {
		return errors.New("storage config data invalid: PROJECTFOLDER, LOCAL_IMAGES_FOLDER, IMAGE_LOCAL_STORAGE, ADVERSARIAL_LOCAL_STORAGE, FILTER_LOCAL_STORAGE, DOWNLOADS_LOCAL_FOLDER and MAX_IMAGE_SIZE are required")
	}
	return nil
}
func (s *StorageLocalConfig) GetProjectFolder() string {
	return s.ProjectFolder
}
func (s *StorageLocalConfig) GetUploadSourceFolder() string {
	return s.UploadSourceFolder
}
func (s *StorageLocalConfig) GetImageStorage() string {
	return s.ImageLocalStorage
}
func (s *StorageLocalConfig) GetAdversarialStorage() string {
	return s.AdversarialLocalStorage
}
func (s *StorageLocalConfig) GetFilterStorage() string {
	return s.FilterLocalStorage
}
func (s *StorageLocalConfig) GetDownloadsFolder() string {
	return s.DownloadsLocalFolder
}
func (s *StorageLocalConfig) GetMaxImageSize() string {
	return s.MaxImageSize
}
