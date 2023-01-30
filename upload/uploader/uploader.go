package uploader

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/hsmtkk/qiita-apple-tomato-classify/upload/csvwriter"
)

type Uploader interface {
	Run()
}

func New(uploaderInfo <-chan UploaderInfo, csvWriterInfo chan<- csvwriter.CSVWriterInfo, bucketName, label string) Uploader {
	return &uploaderImpl{uploaderInfo, csvWriterInfo, bucketName, label}
}

type UploaderInfo struct {
	FilePath string
}

type uploaderImpl struct {
	uploaderInfo  <-chan UploaderInfo
	csvWriterInfo chan<- csvwriter.CSVWriterInfo
	bucketName    string
	label         string
}

func (u *uploaderImpl) Run() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create Cloud Storage client; %v", err.Error())
	}
	defer client.Close()

	bucket := client.Bucket(u.bucketName)
	for info := range u.uploaderInfo {
		fileName := filepath.Base(info.FilePath)
		key := fmt.Sprintf("%s/%s", u.label, fileName)
		obj := bucket.Object(key)
		if err := u.upload(ctx, obj, info.FilePath); err != nil {
			log.Fatal(err)
		}
		u.csvWriterInfo <- csvwriter.CSVWriterInfo{Key: key, Label: u.label}
	}
}

func (u *uploaderImpl) upload(ctx context.Context, dstObj *storage.ObjectHandle, srcPath string) error {
	writer := dstObj.NewWriter(ctx)
	defer writer.Close()
	reader, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open file; %w", err)
	}
	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("failed to copy contents; %w", err)
	}
	return nil
}
