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

func TestDefaultRestyClientHasTimeout(t *testing.T) {
	client := defaultRestyClient()
	assert.Equal(t, client.GetClient().Timeout, defaultTimeout)
}

func TestRestyClientRetriesServerErrors(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < retryCount {
			http.Error(w, "try again", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := configureResty(resty.New().SetRetryWaitTime(time.Millisecond)).R().Get(server.URL)
	assert.NilError(t, err)
	assert.Equal(t, resp.String(), "ok")
	assert.Equal(t, attempts.Load(), int32(retryCount))
}
