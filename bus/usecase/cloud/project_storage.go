package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/belobragin/bvv_utils/goutils/awsutil"
	"github.com/belobragin/werewolf/bus/internal/config"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	stt "github.com/belobragin/werewolf/bus/pkg/storage"
)

const (
	initTimeout = 30 * time.Second
	s3Timeout   = 2 * time.Minute
	// the stack exports the bucket name as <bucket resource>Name
	bucketNameOutputSuffix = "Name"
	stagingDirName         = "werewolf-s3-source"
	stagedTTL              = time.Hour
)

// s3API is the part of the s3 client used by the storage.
type s3API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	CopyObject(ctx context.Context, in *s3.CopyObjectInput, opts ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// StorageRealizationCloud keeps the objects in the s3 bucket of the stack. All the folders are
// key prefixes inside the bucket; uploadSourceFolder is relative to the bucket root.
type StorageRealizationCloud struct {
	s3                      s3API
	s3BucketName            string
	s3BusketUrl             string
	stagingDir              string
	uploadSourceFolder      string
	imageCloudStorage       string
	adversarialCloudStorage string
	filterCloudStorage      string
	downloadsCloudFolder    string
	maxImageSize            string
}

// NewCloudStorage resolves the bucket name from the outputs of the cfn stack.
func NewCloudStorage(conf config.CloudStorageConfigI) (stt.ProjectStorageI, error) {
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()

	cfg, err := awscfg.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	bucket, err := awsutil.GetStackOutput(ctx, cfg, conf.GetStackName(), conf.GetBucketResource()+bucketNameOutputSuffix)
	if err != nil {
		return nil, err
	}

	return newCloudStorage(conf, s3.NewFromConfig(cfg), bucket, cfg.Region)
}

func newCloudStorage(conf config.StorageConfigI, client s3API, bucket, region string) (*StorageRealizationCloud, error) {
	stagingDir := filepath.Join(os.TempDir(), stagingDirName)
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return nil, err
	}

	return &StorageRealizationCloud{
		s3:                      client,
		s3BucketName:            bucket,
		s3BusketUrl:             fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region),
		stagingDir:              stagingDir,
		uploadSourceFolder:      strings.Trim(conf.GetUploadSourceFolder(), "/"),
		imageCloudStorage:       strings.Trim(conf.GetImageStorage(), "/"),
		adversarialCloudStorage: strings.Trim(conf.GetAdversarialStorage(), "/"),
		filterCloudStorage:      strings.Trim(conf.GetFilterStorage(), "/"),
		downloadsCloudFolder:    strings.Trim(conf.GetDownloadsFolder(), "/"),
		maxImageSize:            conf.GetMaxImageSize(),
	}, nil
}

func (st *StorageRealizationCloud) GetMaxImageSize() string {
	return st.maxImageSize
}
func (st *StorageRealizationCloud) GetS3BucketName() string {
	return st.s3BucketName
}
func (st *StorageRealizationCloud) GetS3BusketUrl() string {
	return st.s3BusketUrl
}
func (st *StorageRealizationCloud) GetUploadSourceFolder() string {
	return st.uploadSourceFolder
}
func (st *StorageRealizationCloud) GetImageCloudStorage() string {
	return st.imageCloudStorage
}

func (st *StorageRealizationCloud) GetAdversarialCloudStorage() string {
	return st.adversarialCloudStorage
}

func (st *StorageRealizationCloud) GetFilterCloudStorage() string {
	return st.filterCloudStorage
}

func (st *StorageRealizationCloud) GetDownloadsCloudFolder() string {
	return st.downloadsCloudFolder
}

// ResolveSource accepts only objects inside the upload source folder of the bucket (relPath is the
// full key, e.g. test-image/a.jpg) and stages the object as a local file named like the object,
// because the image checks work with local files. StoreImage removes the staged file.
func (st *StorageRealizationCloud) ResolveSource(relPath string) (string, error) {
	if !fs.ValidPath(relPath) || !strings.HasPrefix(relPath, st.uploadSourceFolder+"/") {
		return "", pdata.ErrInvalidFilePath
	}
	st.sweepStaged()

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	obj, err := st.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(st.s3BucketName),
		Key:    aws.String(relPath),
	})
	if hasCode(err, "NoSuchKey", "NotFound") {
		return "", pdata.ErrNoSourceFile
	}
	if err != nil {
		return "", fmt.Errorf("get s3 object %s: %w", relPath, err)
	}
	defer obj.Body.Close()

	dir, err := os.MkdirTemp(st.stagingDir, "src-")
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, path.Base(relPath))
	if err = writeFile(dst, obj.Body); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dst, nil
}

func (st *StorageRealizationCloud) ImageExists(name string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	_, err := st.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(st.s3BucketName),
		Key:    aws.String(objectKey(st.imageCloudStorage, name)),
	})
	if err == nil {
		return true, nil
	}
	if hasCode(err, "NotFound", "NoSuchKey") {
		return false, nil
	}
	return false, fmt.Errorf("head s3 object %s: %w", name, err)
}

// StoreImage uploads src with a conditional put, so a concurrent upload of the same name fails with ErrExistImageFile.
func (st *StorageRealizationCloud) StoreImage(src, name string) error {
	defer st.dropStaged(src)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	_, err = st.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(st.s3BucketName),
		Key:         aws.String(objectKey(st.imageCloudStorage, name)),
		Body:        in,
		IfNoneMatch: aws.String("*"),
	})
	if hasCode(err, "PreconditionFailed", "ConditionalRequestConflict") {
		return pdata.ErrExistImageFile
	}
	if err != nil {
		return fmt.Errorf("put s3 object %s: %w", name, err)
	}
	return nil
}

// RemoveImage: s3 reports a missing key as success.
func (st *StorageRealizationCloud) RemoveImage(name string) error {
	return st.deleteKey(objectKey(st.imageCloudStorage, name))
}

func (st *StorageRealizationCloud) RemoveObject(kind stt.ObjectKind, name string) error {
	dir, err := st.kindDir(kind)
	if err != nil {
		return err
	}
	return st.deleteKey(objectKey(dir, name))
}

func (st *StorageRealizationCloud) deleteKey(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	_, err := st.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(st.s3BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete s3 object %s: %w", key, err)
	}
	return nil
}

func (st *StorageRealizationCloud) kindDir(kind stt.ObjectKind) (string, error) {
	switch kind {
	case stt.KindImage:
		return st.imageCloudStorage, nil
	case stt.KindAdversarial:
		return st.adversarialCloudStorage, nil
	case stt.KindFilter:
		return st.filterCloudStorage, nil
	}
	return "", pdata.ErrInvalidObjectKind
}

// DownloadObject copies the object inside the bucket to the downloads folder; the copy replaces the earlier one.
func (st *StorageRealizationCloud) DownloadObject(kind stt.ObjectKind, name string) error {
	dir, err := st.kindDir(kind)
	if err != nil {
		return err
	}
	srcKey := objectKey(dir, name)

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	_, err = st.s3.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(st.s3BucketName),
		Key:        aws.String(objectKey(st.downloadsCloudFolder, name)),
		CopySource: aws.String(url.PathEscape(st.s3BucketName + "/" + srcKey)),
	})
	if hasCode(err, "NoSuchKey", "NotFound") {
		return pdata.ErrNoStorageObject
	}
	if err != nil {
		return fmt.Errorf("copy s3 object %s: %w", srcKey, err)
	}
	return nil
}

// objectKey keeps only the base name, so a name can never leave the folder.
func objectKey(folder, name string) string {
	return path.Join(folder, path.Base(name))
}

// hasCode reports whether err is an s3 api error with one of the codes.
func hasCode(err error, codes ...string) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, c := range codes {
		if apiErr.ErrorCode() == c {
			return true
		}
	}
	return false
}

func writeFile(dst string, r io.Reader) error {
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, r); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// dropStaged removes the staged source of src; any other path is left alone.
func (st *StorageRealizationCloud) dropStaged(src string) {
	dir := filepath.Dir(src)
	if filepath.Dir(dir) == st.stagingDir {
		_ = os.RemoveAll(dir)
	}
}

// sweepStaged removes sources that were staged, but never stored (rejected uploads).
func (st *StorageRealizationCloud) sweepStaged() {
	entries, err := os.ReadDir(st.stagingDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > stagedTTL {
			_ = os.RemoveAll(filepath.Join(st.stagingDir, e.Name()))
		}
	}
}
