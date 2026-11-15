package storage

import "errors"

// StorageCloudConfig: the stack name comes from the sqs config of the same service.
// All folders are key prefixes inside the bucket, UploadSourceFolder is relative to the bucket root.
type StorageCloudConfig struct {
	BucketResource          string `conf:"env:AWS_S3_BUCKET_RESOURCE"`
	UploadSourceFolder      string `conf:"env:IMAGES_FOLDER"`
	ImageCloudStorage       string `conf:"env:IMAGE_CLOUD_STORAGE"`
	AdversarialCloudStorage string `conf:"env:ADVERSARIAL_CLOUD_STORAGE"`
	FilterCloudStorage      string `conf:"env:FILTER_CLOUD_STORAGE"`
	DownloadsCloudFolder    string `conf:"env:DOWNLOADS_CLOUD_FOLDER"`
	MaxImageSize            string `conf:"env:MAX_IMAGE_SIZE"`
}

func (s *StorageCloudConfig) ValidateStorageConfig() error {
	if len(s.BucketResource) == 0 || len(s.UploadSourceFolder) == 0 || len(s.MaxImageSize) == 0 ||
		len(s.ImageCloudStorage) == 0 || len(s.AdversarialCloudStorage) == 0 ||
		len(s.FilterCloudStorage) == 0 || len(s.DownloadsCloudFolder) == 0 {
		return errors.New("storage config data invalid: AWS_S3_BUCKET_RESOURCE, IMAGES_FOLDER, IMAGE_CLOUD_STORAGE, ADVERSARIAL_CLOUD_STORAGE, FILTER_CLOUD_STORAGE, DOWNLOADS_CLOUD_FOLDER and MAX_IMAGE_SIZE are required")
	}
	return nil
}

func (s *StorageCloudConfig) GetBucketResource() string {
	return s.BucketResource
}

func (s *StorageCloudConfig) GetUploadSourceFolder() string {
	return s.UploadSourceFolder
}

func (s *StorageCloudConfig) GetImageStorage() string {
	return s.ImageCloudStorage
}

func (s *StorageCloudConfig) GetAdversarialStorage() string {
	return s.AdversarialCloudStorage
}

func (s *StorageCloudConfig) GetFilterStorage() string {
	return s.FilterCloudStorage
}

func (s *StorageCloudConfig) GetDownloadsFolder() string {
	return s.DownloadsCloudFolder
}

func (s *StorageCloudConfig) GetMaxImageSize() string {
	return s.MaxImageSize
}
