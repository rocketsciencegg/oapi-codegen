package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSwaggerWithOverlayBaseDirBaseline(t *testing.T) {
	if os.Getenv("OAPI_TEST_SCOPED_BASELINE") != "1" {
		t.Skip("subprocess baseline for original relative loading")
	}
	opts := LoadSwaggerWithOverlayOpts{Strict: true}
	if os.Getenv("OAPI_TEST_SCOPED_OVERLAY") == "1" {
		opts.Path = "overlay.yaml"
	}
	spec, err := LoadSwaggerWithOverlay("spec.yaml", opts)
	require.NoError(t, err)
	spec.InternalizeRefs(context.Background(), nil)
	data, err := spec.MarshalJSON()
	require.NoError(t, err)
	_, err = os.Stdout.Write(data)
	require.NoError(t, err)
	os.Exit(0)
}

func TestLoadSwaggerWithOverlayBaseDir(t *testing.T) {
	baseDir, err := filepath.Abs("testdata/scoped")
	require.NoError(t, err)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	executable, err := os.Executable()
	require.NoError(t, err)

	for _, withOverlay := range []bool{false, true} {
		t.Run(fmt.Sprint(withOverlay), func(t *testing.T) {
			cmd := exec.Command(executable, "-test.run=^TestLoadSwaggerWithOverlayBaseDirBaseline$")
			cmd.Dir = baseDir
			cmd.Env = append(os.Environ(), "OAPI_TEST_SCOPED_BASELINE=1")
			if withOverlay {
				cmd.Env = append(cmd.Env, "OAPI_TEST_SCOPED_OVERLAY=1")
			}
			want, err := cmd.Output()
			require.NoError(t, err)
			for i := range 8 {
				t.Run(fmt.Sprint(i), func(t *testing.T) {
					t.Parallel()
					opts := LoadSwaggerWithOverlayOpts{BaseDir: baseDir, Strict: true}
					if i%2 == 0 {
						opts.BaseDir = "testdata/scoped"
					}
					if withOverlay {
						opts.Path = "overlay.yaml"
					}
					spec, err := LoadSwaggerWithOverlay("spec.yaml", opts)
					require.NoError(t, err)
					require.Equal(t, "string", spec.Components.Schemas["Record"].Value.Properties["value"].Value.Type.Slice()[0])
					if withOverlay {
						require.Equal(t, "scoped overlay applied", spec.Info.Description)
						require.Equal(t, "integer", spec.Components.Schemas["FromOverlay"].Value.Type.Slice()[0])
					}
					spec.InternalizeRefs(context.Background(), nil)
					got, err := spec.MarshalJSON()
					require.NoError(t, err)
					require.Equal(t, string(want), string(got))
					require.NotContains(t, string(got), filepath.ToSlash(baseDir))
					current, err := os.Getwd()
					require.NoError(t, err)
					require.Equal(t, cwd, current)
				})
			}
		})
	}
}

func TestLoadSwaggerWithOverlayBaseDirAbsolutePaths(t *testing.T) {
	specPath, err := filepath.Abs("testdata/scoped/spec.yaml")
	require.NoError(t, err)
	overlayPath, err := filepath.Abs("testdata/scoped/overlay.yaml")
	require.NoError(t, err)
	spec, err := LoadSwaggerWithOverlay(specPath, LoadSwaggerWithOverlayOpts{
		BaseDir: "does-not-exist", Path: overlayPath, Strict: true,
	})
	require.NoError(t, err)
	require.Equal(t, "scoped overlay applied", spec.Info.Description)
	require.Equal(t, "string", spec.Components.Schemas["Record"].Value.Properties["value"].Value.Type.Slice()[0])
}

func TestLocalRefReaderBaseDir(t *testing.T) {
	reader := localRefReader("testdata/scoped")
	location := &url.URL{Path: "shared.yaml"}
	data, err := reader(openapi3.NewLoader(), location)
	require.NoError(t, err)
	require.Contains(t, string(data), "title: shared")
	require.Equal(t, "shared.yaml", location.Path, "the reader must not mutate logical document URLs")
	location.Scheme = "file"
	_, err = reader(openapi3.NewLoader(), location)
	require.NoError(t, err)
	_, err = reader(openapi3.NewLoader(), &url.URL{Scheme: "https", Host: "example.com", Path: "shared.yaml"})
	require.ErrorIs(t, err, openapi3.ErrURINotSupported)
}

func TestLoadSwaggerWithOverlayBaseDirRemotePolicy(t *testing.T) {
	const top = "https://specs.example.com/spec.yaml"
	const shared = "https://specs.example.com/shared.yaml"
	for _, allowed := range []bool{false, true} {
		t.Run(fmt.Sprint(allowed), func(t *testing.T) {
			transport := &fakeTransport{docs: map[string]string{
				top: remoteDoc, shared: strings.ReplaceAll(remoteDoc, "Remote:", "Extra:"),
			}}
			opts := LoadSwaggerWithOverlayOpts{
				BaseDir: "testdata/scoped", Path: "overlay.yaml", Strict: true,
				AllowRemoteRefsFrom: []string{},
			}
			if allowed {
				opts.AllowRemoteRefsFrom = []string{shared}
			}
			spec, err := loadSwaggerWithOverlay(top, opts, &http.Client{Transport: transport})
			if !allowed {
				require.ErrorContains(t, err, `refusing to resolve remote reference "`+shared+`"`)
				require.Equal(t, []string{top}, transport.requested)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "scoped overlay applied", spec.Info.Description)
			require.Equal(t, "string", spec.Components.Schemas["FromOverlay"].Value.Type.Slice()[0])
			require.Equal(t, []string{top, shared}, transport.requested)
		})
	}
}

const remoteDoc = `openapi: "3.0.0"
info:
  title: remote
  version: "1.0.0"
paths: {}
components:
  schemas:
    Remote:
      type: string
`

// fakeTransport serves remoteDoc for every request, or the document in docs
// for a URL listed there, and records the document URLs it was asked for (a
// fragment never goes on the wire), so the tests need no listening socket.
type fakeTransport struct {
	docs      map[string]string
	requested []string
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := remoteRefKey(req.URL)
	f.requested = append(f.requested, key)
	doc, ok := f.docs[key]
	if !ok {
		doc = remoteDoc
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(doc)),
		Header:     http.Header{"Content-Type": []string{"application/yaml"}},
		Request:    req,
	}, nil
}

func writeSpec(t *testing.T, dir, name, ref string) string {
	t.Helper()
	spec := `openapi: "3.0.0"
info:
  title: local
  version: "1.0.0"
paths: {}
components:
  schemas:
    Local:
      type: object
      properties:
        remote:
          $ref: "` + ref + `"
`
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(spec), 0o600))
	return p
}

func TestLoadSwagger_RemoteRefs(t *testing.T) {
	dir := t.TempDir()
	const doc = "http://schemas.example.com:8080/remote.yaml"
	spec := writeSpec(t, dir, "spec.yaml", doc+"#/components/schemas/Remote")

	cases := []struct {
		name    string
		allowed []string
		refused bool
	}{
		{name: "followed by default on the 2.8 release line", allowed: nil},
		{name: "refused with empty list", allowed: []string{}, refused: true},
		{name: "allowed by exact document URL", allowed: []string{doc}},
		{name: "allowed with surrounding whitespace", allowed: []string{" " + doc + " "}},
		{name: "allowed by wildcard", allowed: []string{"*"}},
		{name: "allowed among others", allowed: []string{"https://other.example.com/x.yaml", doc}},
		{name: "host alone is not enough", allowed: []string{"schemas.example.com"}, refused: true},
		{name: "host and port alone is not enough", allowed: []string{"http://schemas.example.com:8080"}, refused: true},
		{name: "other path refused", allowed: []string{"http://schemas.example.com:8080/other.yaml"}, refused: true},
		{name: "other port refused", allowed: []string{"http://schemas.example.com:9090/remote.yaml"}, refused: true},
		{name: "other scheme refused", allowed: []string{"https://schemas.example.com:8080/remote.yaml"}, refused: true},
		{name: "prefix refused", allowed: []string{"http://schemas.example.com:8080/"}, refused: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &fakeTransport{}
			swagger, err := loadSwagger(spec, tc.allowed, &http.Client{Transport: transport})
			if tc.refused {
				require.ErrorContains(t, err, `refusing to resolve remote reference "`+doc+`"`)
				assert.Empty(t, transport.requested, "no request must leave the process for a refused reference")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{doc}, transport.requested)
			remote := swagger.Components.Schemas["Local"].Value.Properties["remote"]
			require.NotNil(t, remote.Value)
			assert.Equal(t, "string", remote.Value.Type.Slice()[0])
		})
	}
}

func TestLoadSwagger_RemoteRefsRefusedThroughPublicAPI(t *testing.T) {
	// LoadSwaggerWithOverlay uses http.DefaultClient; the refusal happens
	// before any request is built, so this needs no network. On the 2.8
	// release line only an explicit empty list refuses.
	dir := t.TempDir()
	spec := writeSpec(t, dir, "spec.yaml", "http://169.254.169.254/latest/meta-data/#/x")

	_, err := LoadSwaggerWithOverlay(spec, LoadSwaggerWithOverlayOpts{AllowRemoteRefsFrom: []string{}})
	require.ErrorContains(t, err, `refusing to resolve remote reference "http://169.254.169.254/latest/meta-data/"`)
	assert.ErrorContains(t, err, "is not listed in allow-remote-refs-from")
}

func TestLoadSwagger_SpecFromURL(t *testing.T) {
	const top = "https://specs.example.com/v1/api.yaml"
	const sibling = "https://specs.example.com/v1/common.yaml"
	topDoc := func(ref string) string {
		return `openapi: "3.0.0"
info:
  title: top
  version: "1.0.0"
paths: {}
components:
  schemas:
    Top:
      type: object
      properties:
        remote:
          $ref: "` + ref + `"
`
	}

	t.Run("the document itself loads without being listed", func(t *testing.T) {
		transport := &fakeTransport{docs: map[string]string{top: remoteDoc}}
		_, err := loadSwagger(top, nil, &http.Client{Transport: transport})
		require.NoError(t, err)
		assert.Equal(t, []string{top}, transport.requested)
	})

	t.Run("a fragment on the document URL does not stop it loading", func(t *testing.T) {
		transport := &fakeTransport{docs: map[string]string{top: remoteDoc}}
		_, err := loadSwagger(top+"#overview", nil, &http.Client{Transport: transport})
		require.NoError(t, err)
		assert.Equal(t, []string{top}, transport.requested)
	})

	t.Run("a relative reference to the same host is refused until its absolute URL is listed", func(t *testing.T) {
		transport := &fakeTransport{docs: map[string]string{top: topDoc("common.yaml#/components/schemas/Remote")}}
		_, err := loadSwagger(top, []string{}, &http.Client{Transport: transport})
		require.ErrorContains(t, err, `refusing to resolve remote reference "`+sibling+`"`)
		assert.Equal(t, []string{top}, transport.requested)

		transport = &fakeTransport{docs: map[string]string{top: topDoc("common.yaml#/components/schemas/Remote")}}
		swagger, err := loadSwagger(top, []string{sibling}, &http.Client{Transport: transport})
		require.NoError(t, err)
		assert.Equal(t, []string{top, sibling}, transport.requested)
		remote := swagger.Components.Schemas["Top"].Value.Properties["remote"]
		require.NotNil(t, remote.Value)
		assert.Equal(t, "string", remote.Value.Type.Slice()[0])
	})
}

func TestLoadSwaggerWithOverlay_RemoteRefs(t *testing.T) {
	// The document is loaded twice when an overlay is applied; both loads
	// must apply the same policy, including the document's own URL.
	const top = "https://specs.example.com/v1/api.yaml"
	const sibling = "https://specs.example.com/v1/common.yaml"
	topDoc := `openapi: "3.0.0"
info:
  title: top
  version: "1.0.0"
paths: {}
components:
  schemas:
    Top:
      type: object
      properties:
        remote:
          $ref: "common.yaml#/components/schemas/Remote"
`
	overlay := filepath.Join(t.TempDir(), "overlay.yaml")
	require.NoError(t, os.WriteFile(overlay, []byte(`overlay: 1.0.0
info:
  title: add a description
  version: 1.0.0
actions:
  - target: $.info
    update:
      description: overlaid
`), 0o600))

	transport := &fakeTransport{docs: map[string]string{top: topDoc}}
	_, err := loadSwaggerWithOverlay(top, LoadSwaggerWithOverlayOpts{Path: overlay, Strict: true, AllowRemoteRefsFrom: []string{}}, &http.Client{Transport: transport})
	require.ErrorContains(t, err, `refusing to resolve remote reference "`+sibling+`"`)

	transport = &fakeTransport{docs: map[string]string{top: topDoc}}
	swagger, err := loadSwaggerWithOverlay(top, LoadSwaggerWithOverlayOpts{Path: overlay, Strict: true, AllowRemoteRefsFrom: []string{sibling}}, &http.Client{Transport: transport})
	require.NoError(t, err)
	assert.Equal(t, "overlaid", swagger.Info.Description)
	assert.Equal(t, []string{top, sibling, sibling}, transport.requested, "the overlaid document is loaded again and resolves the sibling again")

	// On the 2.8 release line an unset list allows every remote reference,
	// and that default must apply to the load after the overlay as well.
	transport = &fakeTransport{docs: map[string]string{top: topDoc}}
	swagger, err = loadSwaggerWithOverlay(top, LoadSwaggerWithOverlayOpts{Path: overlay, Strict: true}, &http.Client{Transport: transport})
	require.NoError(t, err)
	assert.Equal(t, "overlaid", swagger.Info.Description)
	assert.Equal(t, []string{top, sibling, sibling}, transport.requested)
}

func TestLoadSwagger_LocalFileRefsStillFollowed(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "remote.yaml"), []byte(remoteDoc), 0o600))
	spec := writeSpec(t, dir, "spec.yaml", "./remote.yaml#/components/schemas/Remote")

	transport := &fakeTransport{}
	swagger, err := loadSwagger(spec, nil, &http.Client{Transport: transport})
	require.NoError(t, err)
	assert.Empty(t, transport.requested)
	remote := swagger.Components.Schemas["Local"].Value.Properties["remote"]
	require.NotNil(t, remote.Value)
	assert.Equal(t, "string", remote.Value.Type.Slice()[0])
}

func TestRemoteRefAllowed(t *testing.T) {
	loc := func(s string) *url.URL {
		u, err := url.Parse(s)
		require.NoError(t, err)
		return u
	}
	assert.False(t, RemoteRefAllowed(nil, loc("http://a.example/x.yaml")))
	assert.True(t, RemoteRefAllowed([]string{"*"}, loc("http://a.example/x.yaml")))
	assert.True(t, RemoteRefAllowed([]string{"http://a.example/x.yaml"}, loc("http://a.example/x.yaml")))
	assert.True(t, RemoteRefAllowed([]string{"http://a.example/x.yaml"}, loc("http://a.example/x.yaml#/components/schemas/X")), "the fragment names a part of the document, not another document")
	assert.True(t, RemoteRefAllowed([]string{"http://[::1]:8080/x.yaml"}, loc("http://[::1]:8080/x.yaml")))
	assert.False(t, RemoteRefAllowed([]string{"http://a.example/x.yaml/"}, loc("http://a.example/x.yaml")))
	assert.False(t, RemoteRefAllowed([]string{"http://A.example/x.yaml"}, loc("http://a.example/x.yaml")), "nothing is normalised")
	assert.False(t, RemoteRefAllowed([]string{"a.example"}, loc("http://a.example/x.yaml")))
}
