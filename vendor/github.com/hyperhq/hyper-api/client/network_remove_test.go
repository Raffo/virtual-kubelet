package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"context"
)

func TestNetworkRemoveError(t *testing.T) {
	client := &Client{
		transport: newMockClient(nil, errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.NetworkRemove(context.Background(), "network_id")
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestNetworkRemove(t *testing.T) {
	expectedURL := "/networks/network_id"

	client := &Client{
		transport: newMockClient(nil, func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != "DELETE" {
				return nil, fmt.Errorf("expected DELETE method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	err := client.NetworkRemove(context.Background(), "network_id")
	if err != nil {
		t.Fatal(err)
	}
}
