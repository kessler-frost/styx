package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client provides access to Styx/Nomad/Vault APIs.
type Client struct {
	httpClient *http.Client
	nomadAddr  string
	vaultAddr  string
}

// NewClient creates a new API client with default addresses.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 2 * time.Second},
		nomadAddr:  "http://127.0.0.1:4646",
		vaultAddr:  "http://127.0.0.1:8200",
	}
}

// WithNomadAddr sets a custom Nomad address.
func (c *Client) WithNomadAddr(addr string) *Client {
	c.nomadAddr = addr
	return c
}

// WithVaultAddr sets a custom Vault address.
func (c *Client) WithVaultAddr(addr string) *Client {
	c.vaultAddr = addr
	return c
}

// get performs a GET request and decodes JSON response.
func (c *Client) get(url string, v interface{}) error {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// getStatus performs a GET request and returns the status code.
func (c *Client) getStatus(url string) (int, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
