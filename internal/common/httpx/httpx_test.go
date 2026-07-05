package httpx

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"gotest.tools/v3/assert"
)

func TestDefaultRestyClientHasTimeoutAndRetries(t *testing.T) {
	client := defaultRestyClient()
	assert.Equal(t, client.GetClient().Timeout, defaultTimeout)
	assert.Equal(t, client.RetryCount, 3)
}

func TestRetryableClientAllowsExplicitNoTimeout(t *testing.T) {
	assert.Equal(t, RetryableClient(0).Timeout, time.Duration(0))
}

func TestRestyClientRetriesServerErrors(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			http.Error(w, "try again", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := configureResty(resty.New().SetRetryWaitTime(time.Millisecond)).R().Get(server.URL)
	assert.NilError(t, err)
	assert.Equal(t, resp.String(), "ok")
	assert.Equal(t, attempts.Load(), int32(3))
}
