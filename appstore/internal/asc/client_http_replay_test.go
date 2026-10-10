package asc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

func TestReplayableRequestSnapshotsRemainingBody(t *testing.T) {
	for _, kind := range []string{"bytes-reader", "bytes-buffer", "strings-reader", "unknown-reader", "empty-reader", "empty-buffer", "empty-string", "nil"} {
		t.Run(kind, func(t *testing.T) {
			original := []byte("prefix:payload")
			var body io.Reader
			want := "payload"
			switch kind {
			case "bytes-reader":
				body = bytes.NewReader(original)
			case "bytes-buffer":
				body = bytes.NewBuffer(original)
			case "strings-reader":
				body = strings.NewReader(string(original))
			case "unknown-reader":
				body = io.LimitReader(bytes.NewReader(original), int64(len(original)))
			case "empty-reader":
				body, want = bytes.NewReader(nil), ""
			case "empty-buffer":
				body, want = bytes.NewBuffer(nil), ""
			case "empty-string":
				body, want = strings.NewReader(""), ""
			case "nil":
				want = ""
			}
			if want != "" {
				if _, err := io.CopyN(io.Discard, body, 7); err != nil {
					t.Fatal(err)
				}
			}
			client := newTestClient(t, func(req *http.Request) {
				var got []byte
				if req.Body != nil {
					var err error
					got, err = io.ReadAll(req.Body)
					if err != nil {
						t.Fatal(err)
					}
				}
				if string(got) != want {
					t.Fatalf("body = %q, want %q", got, want)
				}
			}, &http.Response{StatusCode: 204, Body: http.NoBody}, &http.Response{StatusCode: 204, Body: http.NoBody})
			request, err := client.replayableRequest(http.MethodPost, "/v1/apps", body)
			if err != nil {
				t.Fatal(err)
			}
			if body != nil {
				remaining, err := io.ReadAll(body)
				if err != nil || len(remaining) != 0 {
					t.Fatalf("reader not consumed: %q, %v", remaining, err)
				}
			}
			for i := range original {
				original[i] = 'x'
			}
			for range 2 {
				if _, err := request(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

type replayFailureReader struct{ err error }

func (r replayFailureReader) Read([]byte) (int, error) { return 0, r.err }

func TestReplayableRequestPreservesReaderError(t *testing.T) {
	cause := errors.New("body failed")
	request, err := (&Client{}).replayableRequest(http.MethodPost, "/v1/apps", replayFailureReader{cause})
	if request != nil || !errors.Is(err, cause) || err.Error() != "failed to read request body: body failed" {
		t.Fatalf("request/error = %v/%v", request != nil, err)
	}
}

func BenchmarkReplayableRequestSnapshot(b *testing.B) {
	for _, size := range []int{512, 64 << 10, 1 << 20} {
		for _, kind := range []string{"bytes-reader", "bytes-buffer", "strings-reader"} {
			b.Run(fmt.Sprintf("%s/%d", kind, size), func(b *testing.B) {
				payload := bytes.Repeat([]byte{'a'}, size)
				text := string(payload)
				client := &Client{}
				b.ReportAllocs()
				b.SetBytes(int64(size))
				b.ResetTimer()
				for range b.N {
					var body io.Reader
					switch kind {
					case "bytes-reader":
						body = bytes.NewReader(payload)
					case "bytes-buffer":
						body = bytes.NewBuffer(payload)
					case "strings-reader":
						body = strings.NewReader(text)
					}
					request, err := client.replayableRequest(http.MethodPost, "/v1/apps", body)
					if err != nil {
						b.Fatal(err)
					}
					runtime.KeepAlive(request)
				}
			})
		}
	}
}
