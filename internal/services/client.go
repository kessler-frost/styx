package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NomadClient wraps the Nomad HTTP API
type NomadClient struct {
	addr   string
	client *http.Client
}

// NewNomadClient creates a new Nomad API client
func NewNomadClient(addr string) *NomadClient {
	return &NomadClient{
		addr: addr,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DefaultClient returns a client pointing to localhost
func DefaultClient() *NomadClient {
	return NewNomadClient("http://127.0.0.1:4646")
}

// JobStatus represents the status of a Nomad job
type JobStatus struct {
	ID     string
	Status string // pending, running, dead
}

// RunJob submits a job to Nomad (HCL format)
func (c *NomadClient) RunJob(hcl string) error {
	// First, parse the HCL to JSON
	jsonJob, err := c.parseHCL(hcl)
	if err != nil {
		return fmt.Errorf("failed to parse job HCL: %w", err)
	}

	// Submit the job
	req, err := http.NewRequest("POST", c.addr+"/v1/jobs", bytes.NewReader(jsonJob))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to submit job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to submit job (status %d): unable to read response body", resp.StatusCode)
		}
		return fmt.Errorf("failed to submit job (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// parseHCL converts HCL job spec to JSON using Nomad's parse endpoint
func (c *NomadClient) parseHCL(hcl string) ([]byte, error) {
	payload := map[string]interface{}{
		"JobHCL":       hcl,
		"Canonicalize": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.addr+"/v1/jobs/parse", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HCL (status %d): unable to read response body", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to parse HCL (status %d): %s", resp.StatusCode, string(respBody))
	}

	// The response is the parsed job JSON
	parsedJob, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Wrap in the expected format for job submission
	var job interface{}
	if err := json.Unmarshal(parsedJob, &job); err != nil {
		return nil, err
	}

	wrapped := map[string]interface{}{
		"Job": job,
	}

	return json.Marshal(wrapped)
}

// StopJob stops a running job
func (c *NomadClient) StopJob(jobID string) error {
	req, err := http.NewRequest("DELETE", c.addr+"/v1/job/"+jobID, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stop job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to stop job (status %d): unable to read response body", resp.StatusCode)
		}
		return fmt.Errorf("failed to stop job (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetJobStatus returns the status of a job
func (c *NomadClient) GetJobStatus(jobID string) (*JobStatus, error) {
	resp, err := c.client.Get(c.addr + "/v1/job/" + jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Job doesn't exist
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to get job status (status %d): unable to read response body", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to get job status (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID     string `json:"ID"`
		Status string `json:"Status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &JobStatus{
		ID:     result.ID,
		Status: result.Status,
	}, nil
}

// ListJobs returns all jobs
func (c *NomadClient) ListJobs() ([]JobStatus, error) {
	resp, err := c.client.Get(c.addr + "/v1/jobs")
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to list jobs (status %d): unable to read response body", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to list jobs (status %d): %s", resp.StatusCode, string(body))
	}

	var jobs []struct {
		ID     string `json:"ID"`
		Status string `json:"Status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}

	result := make([]JobStatus, len(jobs))
	for i, j := range jobs {
		result[i] = JobStatus{
			ID:     j.ID,
			Status: j.Status,
		}
	}

	return result, nil
}

// IsHealthy checks if Nomad is responding
func (c *NomadClient) IsHealthy() bool {
	resp, err := c.client.Get(c.addr + "/v1/agent/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
