package csvwriter

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
)

type CSVWriter interface {
	Run()
}

func New(csvWriterInfo <-chan CSVWriterInfo, bucketName string, csvFile io.Writer) CSVWriter {
	return &writerImpl{csvWriterInfo, bucketName, csvFile}
}

type CSVWriterInfo struct {
	Key   string
	Label string
}

type writerImpl struct {
	csvWriterInfo <-chan CSVWriterInfo
	bucketName    string
	csvFile       io.Writer
}

func (w *writerImpl) Run() {
	writer := csv.NewWriter(w.csvFile)
	defer writer.Flush()
	for info := range w.csvWriterInfo {
		path := fmt.Sprintf("gs://%s/%s", w.bucketName, info.Key)
		data := []string{path, info.Label}
		if err := writer.Write(data); err != nil {
			log.Fatalf("failed to write CSV file; %v", err.Error())
		}
	}
}
