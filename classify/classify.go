package classify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func init() {
	functions.CloudEvent("classify", classify)
}

type StorageObjectData struct {
	Bucket string `json:"bucket,omitempty"`
	Name   string `json:"name,omitempty"`
}

func classify(ctx context.Context, e cloudevents.Event) error {
	log.Print("dump event")
	log.Printf("%v", e)

	var storageData StorageObjectData
	if err := e.DataAs(&storageData); err != nil {
		return fmt.Errorf("failed to decode event; %w", err)
	}

	clt, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create new storage client; %w", err)
	}
	defer clt.Close()

	client := newStorageClient(clt)

	content, err := client.fetch(ctx, storageData.Bucket, storageData.Name)
	if err != nil {
		return err
	}

	projectID := os.Getenv("PROJECT_ID")
	endpointID := os.Getenv("ENDPOINT_ID")

	result, err := queryVertexAI(projectID, endpointID, content)
	if err != nil {
		return err
	}

	var bucket string
	switch result {
	case "apple":
		bucket = os.Getenv("DST_APPLE_BUCKET")
	case "tomato":
		bucket = os.Getenv("DST_TOMATO_BUCKET")
	default:
		return fmt.Errorf("unknown result; %s", result)
	}

	if err := client.upload(ctx, bucket, storageData.Name, content); err != nil {
		return err
	}
	if err := client.delete(ctx, storageData.Bucket, storageData.Name); err != nil {
		return err
	}

	return nil
}

type storageClient struct {
	client *storage.Client
}

func newStorageClient(client *storage.Client) *storageClient {
	return &storageClient{client}
}

func (s *storageClient) fetch(ctx context.Context, bucket, key string) ([]byte, error) {
	reader, err := s.client.Bucket(bucket).Object(key).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read object; %w", err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read content; %w", err)
	}
	return content, nil
}

func (s *storageClient) upload(ctx context.Context, bucket, key string, content []byte) error {
	writer := s.client.Bucket(bucket).Object(key).NewWriter(ctx)
	defer writer.Close()
	if _, err := writer.Write(content); err != nil {
		return fmt.Errorf("failed to write object; %w", err)
	}
	return nil
}

func (s *storageClient) delete(ctx context.Context, bucket, key string) error {
	if err := s.client.Bucket(bucket).Object(key).Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object; %w", err)
	}
	return nil
}

type vertexRequest struct {
	Instances  []vertexRequestInstance `json:"instances"`
	Parameters vertexRequestParameters `json:"parameters"`
}

type vertexRequestInstance struct {
	Content string `json:"content"`
}

type vertexRequestParameters struct {
	ConfidenceThreshold float64 `json:"confidenceThreshold"`
	MaxPredictions      int     `json:"maxPredictions"`
}

type vertexResponse struct {
	Predictions []vertexResponsePrediction `json:"predictions"`
}

type vertexResponsePrediction struct {
	DisplayNames []string `json:"displayNames"`
}

func queryVertexAI(projectID, endpointID string, content []byte) (string, error) {
	url := fmt.Sprintf("https://us-central1-aiplatform.googleapis.com/v1/projects/%s/locations/us-central1/endpoints/%s:predict", projectID, endpointID)

	token, err := getAccessToken()
	if err != nil {
		return "", err
	}

	encodedContent := base64.StdEncoding.EncodeToString(content)
	body, err := json.Marshal(vertexRequest{
		Instances: []vertexRequestInstance{
			{Content: encodedContent},
		},
		Parameters: vertexRequestParameters{
			ConfidenceThreshold: 0.5,
			MaxPredictions:      5,
		}})
	if err != nil {
		return "", fmt.Errorf("failed to encode JSON; %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to make new HTTP request; %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("got error HTTP status code; %d; %s", resp.StatusCode, resp.Status)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read HTTP response; %w", err)
	}
	var vertexResp vertexResponse
	if err := json.Unmarshal(respBytes, &vertexResp); err != nil {
		return "", fmt.Errorf("failed to decode JSON; %w", err)
	}

	if len(vertexResp.Predictions) > 0 && len(vertexResp.Predictions[0].DisplayNames) > 0 {
		return vertexResp.Predictions[0].DisplayNames[0], nil
	} else {
		return "", fmt.Errorf("failed to classify image")
	}
}

type metadataResponse struct {
	AccessToken string `json:"access_token"`
}

// https://cloud.google.com/functions/docs/securing/function-identity#access_tokens
func getAccessToken() (string, error) {
	url := "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token?scopes=%s"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request; %w", err)
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request; %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("got error HTTP status code; %d; %s", resp.StatusCode, resp.Status)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read HTTP response; %w", err)
	}
	var metaResponse metadataResponse
	if err := json.Unmarshal(respBytes, &metaResponse); err != nil {
		return "", fmt.Errorf("failed to decode response JSON; %w", err)
	}
	return metaResponse.AccessToken, nil
}
