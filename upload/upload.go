package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/hsmtkk/qiita-apple-tomato-classify/upload/csvwriter"
	"github.com/hsmtkk/qiita-apple-tomato-classify/upload/uploader"
	"github.com/spf13/cobra"
)

const (
	trainDir  = "../archive/train"
	uploaders = 4
)

func process(imgDir, bucketName, label string, csvFile io.Writer) error {
	uploaderChan := make(chan uploader.UploaderInfo)
	csvWriterChan := make(chan csvwriter.CSVWriterInfo)

	var uploaderGroup sync.WaitGroup
	var csvWriterGroup sync.WaitGroup

	for i := 0; i < uploaders; i++ {
		uploaderGroup.Add(1)
		go func() {
			defer uploaderGroup.Done()
			uploader := uploader.New(uploaderChan, csvWriterChan, bucketName, label)
			uploader.Run()
		}()
	}

	csvWriterGroup.Add(1)
	go func() {
		defer csvWriterGroup.Done()
		writer := csvwriter.New(csvWriterChan, bucketName, csvFile)
		writer.Run()
	}()

	entries, err := os.ReadDir(imgDir)
	if err != nil {
		return fmt.Errorf("failed to read directory; %v", err.Error())
	}
	for _, entry := range entries {
		uploaderChan <- uploader.UploaderInfo{FilePath: filepath.Join(imgDir, entry.Name())}
	}

	close(uploaderChan)
	uploaderGroup.Wait()
	close(csvWriterChan)
	csvWriterGroup.Wait()

	return nil
}

func run(cmd *cobra.Command, args []string) {
	applesBucket := args[0]
	tomatoesBucket := args[1]
	csvPath := args[2]

	csvFile, err := os.Create(csvPath)
	if err != nil {
		log.Fatalf("failed to create CSV file; %v", err.Error())
	}
	defer csvFile.Close()

	if err := process(filepath.Join(trainDir, "apples"), applesBucket, "apple", csvFile); err != nil {
		log.Fatal(err)
	}
	if err := process(filepath.Join(trainDir, "tomatoes"), tomatoesBucket, "tomato", csvFile); err != nil {
		log.Fatal(err)
	}
}

func main() {
	cmd := &cobra.Command{
		Use:  "upload applesBucket tomatoesBucket csvPath",
		Args: cobra.ExactArgs(1),
		Run:  run,
	}
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
