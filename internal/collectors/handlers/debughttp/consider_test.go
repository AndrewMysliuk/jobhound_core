package debughttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/consider"
)

func TestPostConsider(t *testing.T) {
	t.Parallel()

	var posts [][]byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jobs":
			_, _ = io.WriteString(w, `{"csrfToken":"csrf-1"}`)
		case "/api-boards/search-jobs":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			posts = append(posts, body)
			_, _ = io.WriteString(w, `{"jobs":[]}`)
		default:
			http.Error(w, "unexpected path", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(upstream.Close)

	board := &consider.Board{
		HTTPClient: upstream.Client(),
		Tenant: consider.Tenant{
			SourceID: "consider_andreessen_horowitz",
			Host:     upstream.URL,
			BoardID:  "andreessen-horowitz",
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
		map[string]*consider.Board{board.Name(): board},
		nil,
	)

	t.Run("missing source_id is 400", func(t *testing.T) {
		rec := postConsider(h, `{}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("blank source_id is 400", func(t *testing.T) {
		rec := postConsider(h, `{"source_id":"  "}`)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown source_id is 404", func(t *testing.T) {
		rec := postConsider(h, `{"source_id":"consider_missing"}`)
		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Empty(t, posts)
	})

	t.Run("omitted queries use the default skill ids", func(t *testing.T) {
		rec := postConsider(h, `{"source_id":"consider_andreessen_horowitz","max_pages":1}`)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 3, board.MaxPages)

		var out runCollectorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		require.True(t, out.OK)
		require.Equal(t, "consider_andreessen_horowitz", out.Collector)
		require.Equal(t, 0, out.Count)

		require.Len(t, posts, 1)
		var body struct {
			Query struct {
				Skills []string `json:"skills"`
			} `json:"query"`
		}
		require.NoError(t, json.Unmarshal(posts[0], &body))
		require.Equal(t, []string{
			"resume:Vue.js",
			"resume:TypeScript",
			"resume:React.js",
			"resume:Go",
			"resume:Node.js",
		}, body.Query.Skills)
	})
}

func postConsider(h *HTTPHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/debug/collectors/consider", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
