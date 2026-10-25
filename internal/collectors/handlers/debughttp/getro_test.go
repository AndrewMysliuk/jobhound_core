package debughttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/getro"
)

func TestPostGetro(t *testing.T) {
	t.Parallel()

	var posts [][]byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		posts = append(posts, body)
		_, _ = io.WriteString(w, `{"results":{"jobs":[]}}`)
	}))
	t.Cleanup(upstream.Close)

	board := &getro.Board{
		HTTPClient: getroDebugClient(upstream),
		Tenant: getro.Tenant{
			SourceID:     "getro_accel",
			CollectionID: 8672,
		},
		MaxPages: 3,
	}
	h := NewHTTPHandler(
		stubCollector{name: "europe_remotely"},
		stubCollector{name: "working_nomads"},
		stubCollector{name: "himalayas"},
		stubCollector{name: "builtin"},
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		zerolog.Nop(),
		nil,
		map[string]*getro.Board{board.Name(): board},
	)

	t.Run("missing source_id is 400", func(t *testing.T) {
		rec := postGetro(h, `{}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("blank source_id is 400", func(t *testing.T) {
		rec := postGetro(h, `{"source_id":"  "}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown source_id is 404", func(t *testing.T) {
		rec := postGetro(h, `{"source_id":"getro_missing"}`)
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Empty(t, posts)
	})

	t.Run("omitted q uses vue", func(t *testing.T) {
		rec := postGetro(h, `{"source_id":"getro_accel","max_pages":1}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 3, board.MaxPages)

		var out runCollectorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		require.True(t, out.OK)
		require.Equal(t, "getro_accel", out.Collector)
		require.Equal(t, 0, out.Count)

		require.Len(t, posts, 1)
		var body struct {
			Query string `json:"query"`
		}
		require.NoError(t, json.Unmarshal(posts[0], &body))
		require.Equal(t, "vue", body.Query)
	})
}

func postGetro(h *HTTPHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/debug/collectors/getro", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getroDebugClient(srv *httptest.Server) *http.Client {
	base := srv.Client().Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{Transport: getroRewriteHost{baseURL: srv.URL, next: base}}
}

type getroRewriteHost struct {
	baseURL string
	next    http.RoundTripper
}

func (r getroRewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = u.Scheme
	clone.URL.Host = u.Host
	clone.Host = u.Host
	return r.next.RoundTrip(clone)
}
