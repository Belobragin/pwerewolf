package config

// StorageConfigI is common for the local and the cloud storage.
type StorageConfigI interface {
	ValidateStorageConfig() error
	GetUploadSourceFolder() string
	GetImageStorage() string
	GetAdversarialStorage() string
	GetFilterStorage() string
	GetDownloadsFolder() string
	GetMaxImageSize() string
}

type LocalStorageConfigI interface {
	StorageConfigI
	GetProjectFolder() string
}

// CloudStorageConfigI: the storage folders are key prefixes inside the s3 bucket,
// the bucket is a resource of the cfn stack.
type CloudStorageConfigI interface {
	StorageConfigI
	GetStackName() string
	GetBucketResource() string
}
