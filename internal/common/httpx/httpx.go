package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/renameio/v2"
	"github.com/hashicorp/go-retryablehttp"
)

var defaultRestyClient = sync.OnceValue(func() *resty.Client {
	return configureResty(resty.New().SetTimeout(defaultTimeout))
})

type Client struct {
	HTTP      *http.Client
	UserAgent string
}

type TextResponse struct {
	Status int
	Body   string
}

const defaultTimeout = 30 * time.Second

func (c *Client) GetBearerText(url, token string) (TextResponse, error) {
	resp, err := c.request().SetAuthToken(token).Get(url)
	return textResponse(resp, err)
}

func (c *Client) PostJSONBearerText(url, token string, body any) (TextResponse, error) {
	resp, err := c.request().
		SetAuthToken(token).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(url)
	return textResponse(resp, err)
}

func (c *Client) Reader(url string) (io.ReadCloser, error) {
	resp, err := c.rawGet(url)
	if err != nil {
		return nil, err
	}
	return resp.RawBody(), nil
}

func (c *Client) Bytes(url string) ([]byte, error) {
	resp, err := c.request().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status())
	}
	return resp.Body(), nil
}

func (c *Client) Text(url string) (string, error) {
	body, err := c.Bytes(url)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) JSON(url string, out any) error {
	resp, err := c.request().Get(url)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("GET %s: %s", url, resp.Status())
	}
	return json.Unmarshal(resp.Body(), out)
}

func (c *Client) DownloadFile(url, path string) (err error) {
	resp, err := c.rawGet(url)
	if err != nil {
		return err
	}
	body := resp.RawBody()
	defer func() { err = errors.Join(err, body.Close()) }()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := renameio.NewPendingFile(path, renameio.WithPermissions(0o644))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Cleanup()) }()
	_, copyErr := io.Copy(file, body)
	if copyErr != nil {
		return copyErr
	}
	return file.CloseAtomicallyReplace()
}

func (c *Client) ResolveURL(url string) (string, error) {
	resp, err := c.request().Head(url)
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("HEAD %s: %s", url, resp.Status())
	}
	return resp.RawResponse.Request.URL.String(), nil
}

func (c *Client) rawGet(url string) (*resty.Response, error) {
	resp, err := c.request().SetDoNotParseResponse(true).Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, errors.Join(
			fmt.Errorf("GET %s: %s", url, resp.Status()),
			resp.RawBody().Close(),
		)
	}
	return resp, nil
}

func textResponse(resp *resty.Response, err error) (TextResponse, error) {
	if err != nil {
		return TextResponse{}, err
	}
	return TextResponse{Status: resp.StatusCode(), Body: resp.String()}, nil
}

func RetryableClient(timeout time.Duration) *http.Client {
	retryClient := retryablehttp.NewClient()
	retryClient.Logger = nil
	retryClient.RetryMax = 3
	client := retryClient.StandardClient()
	client.Timeout = timeout
	return client
}

func (c *Client) request() *resty.Request {
	req := c.resty().R()
	if c.UserAgent != "" {
		req.SetHeader("User-Agent", c.UserAgent)
	}
	return req
}

func (c *Client) resty() *resty.Client {
	if c.HTTP == nil {
		return defaultRestyClient()
	}
	return configureResty(resty.NewWithClient(c.HTTP))
}

func configureResty(client *resty.Client) *resty.Client {
	return client.
		SetRetryCount(3).
		AddRetryCondition(func(resp *resty.Response, err error) bool {
			return err != nil || resp.StatusCode() >= http.StatusInternalServerError
		})
}
