package main

import (
	"bytes"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
)

const (
	ServerStartupSleepTime = 50 * time.Millisecond
)

func TestServerForMappedHost(t *testing.T) {
	c := config{
		Mapping: map[string]string{
			"localhost": "http://test.local/path",
		},
	}
	tests := []struct {
		name          string
		requestMethod string
		requestRelUrl string
		requestBody   []byte
		wantResponse  *http.Response
	}{
		{
			name:          "Simple GET Request",
			requestMethod: "GET",
			requestRelUrl: "",
			wantResponse:  httpmock.NewStringResponse(200, "Hello, Test!"),
		},
		{
			name:          "GET Request with Extra Header",
			requestMethod: "GET",
			requestRelUrl: "with-header",
			wantResponse: updateResponse(
				httpmock.NewStringResponse(200, "Hello, Test!"),
				func(r *http.Response) {
					r.Header.Add("x-purpose", "test")
				},
			),
		},
		{
			name:          "GET Request with Query String",
			requestMethod: "GET",
			requestRelUrl: "with-query/?blah=blotz",
			wantResponse:  httpmock.NewStringResponse(404, "Hello, Test!"),
		},
		{
			name:          "POST with Body",
			requestMethod: "POST",
			requestRelUrl: "body-data?a=b&c=d",
			requestBody:   []byte("this is my request body"),
			wantResponse:  httpmock.NewStringResponse(201, "Hello, Test!"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultTransport := http.DefaultTransport
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(tt.requestMethod,
				"http://test.local/path/"+tt.requestRelUrl,
				httpmock.ResponderFromResponse(tt.wantResponse))

			wg := &sync.WaitGroup{}
			wg.Add(1)
			srv := runServer(wg, 8000, c)
			time.Sleep(ServerStartupSleepTime)

			req, err := http.NewRequest(tt.requestMethod,
				"http://localhost:8000/"+tt.requestRelUrl,
				bytes.NewReader(tt.requestBody),
			)
			if nil != err {
				t.Fatalf("failed to create request: %v", err)
			}

			client := &http.Client{
				Transport: defaultTransport,
			}
			got, err := client.Do(req)
			if nil != err {
				t.Fatalf("failed to send request: %v", err)
			}

			assertResponses(t, got, tt.wantResponse)

			if err := srv.Shutdown(t.Context()); nil != err {
				t.Errorf("graceful shutdown failed: %v", err)
			}
			wg.Wait()
		})
	}
}

func TestServerForMappedHostButUnavailbleHost(t *testing.T) {
	c := config{
		Mapping: map[string]string{
			"localhost": "http://test.local/path/",
		},
	}
	tests := []struct {
		name          string
		requestMethod string
		requestRelUrl string
		requestBody   []byte
		wantResponse  *http.Response
	}{
		{
			name:          "Simple GET Request",
			requestMethod: "GET",
			requestRelUrl: "",
			wantResponse:  httpmock.NewStringResponse(502, "Hello, Test!"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultTransport := http.DefaultTransport

			wg := &sync.WaitGroup{}
			wg.Add(1)
			srv := runServer(wg, 8001, c)
			time.Sleep(ServerStartupSleepTime)

			req, err := http.NewRequest(tt.requestMethod,
				"http://localhost:8001/"+tt.requestRelUrl,
				bytes.NewReader(tt.requestBody),
			)
			if nil != err {
				t.Fatalf("failed to create request: %v", err)
			}

			client := &http.Client{
				Transport: defaultTransport,
			}
			got, err := client.Do(req)
			if nil != err {
				t.Fatalf("failed to send request: %v", err)
			}

			assertResponses(t, got, tt.wantResponse)

			if err := srv.Shutdown(t.Context()); nil != err {
				t.Errorf("graceful shutdown failed: %v", err)
			}
			wg.Wait()
		})
	}
}

func TestServerForUnmappedHost(t *testing.T) {
	c := config{}
	tests := []struct {
		name          string
		requestMethod string
		requestRelUrl string
		requestBody   []byte
		wantResponse  *http.Response
	}{
		{
			name:          "Simple GET Request",
			requestMethod: "GET",
			requestRelUrl: "",
			wantResponse:  httpmock.NewStringResponse(404, "This page does not exist"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultTransport := http.DefaultTransport

			wg := &sync.WaitGroup{}
			wg.Add(1)
			srv := runServer(wg, 8002, c)
			time.Sleep(ServerStartupSleepTime)

			req, err := http.NewRequest(tt.requestMethod,
				"http://localhost:8002/"+tt.requestRelUrl,
				bytes.NewReader(tt.requestBody),
			)
			if nil != err {
				t.Fatalf("failed to create request: %v", err)
			}

			client := &http.Client{
				Transport: defaultTransport,
			}
			got, err := client.Do(req)
			if nil != err {
				t.Fatalf("failed to send request: %v", err)
			}

			assertResponses(t, got, tt.wantResponse)

			if err := srv.Shutdown(t.Context()); nil != err {
				t.Errorf("graceful shutdown failed: %v", err)
			}
			wg.Wait()
		})
	}
}

func Test_readConfig(t *testing.T) {
	errorNilOrPanic(os.WriteFile("/tmp/sirup_Test_readConfig_01.yaml", nil, 0644))
	errorNilOrPanic(os.WriteFile("/tmp/sirup_Test_readConfig_02.yaml", []byte(`
mapping:
`), 0644))
	errorNilOrPanic(os.WriteFile("/tmp/sirup_Test_readConfig_03.yaml", []byte(`
mapping:
  foo: http://bar.com/baz
`), 0644))
	errorNilOrPanic(os.WriteFile("/tmp/sirup_Test_readConfig_04.yaml", []byte(`
mapping:
  abc: http://def.com/
  xyz: https://hello.world/xyz/
`), 0644))
	errorNilOrPanic(os.WriteFile("/tmp/sirup_Test_readConfig_05.yaml", []byte(`mapping: a string value`), 0644))

	type args struct {
		path string
	}
	tests := []struct {
		name    string
		path    string
		want    *config
		wantErr bool
	}{
		{
			name:    "Inexistent file",
			path:    "/blah",
			wantErr: true,
		},
		{
			name: "Empty file",
			path: "/tmp/sirup_Test_readConfig_01.yaml",
			want: &config{},
		},
		{
			name: "Empty mapping",
			path: "/tmp/sirup_Test_readConfig_02.yaml",
			want: &config{},
		},
		{
			name: "Mapping with one host",
			path: "/tmp/sirup_Test_readConfig_03.yaml",
			want: &config{
				Mapping: map[string]string{
					"foo": "http://bar.com/baz",
				},
			},
		},
		{
			name: "Mapping with multiple hosts",
			path: "/tmp/sirup_Test_readConfig_04.yaml",
			want: &config{
				Mapping: map[string]string{
					"abc": "http://def.com/",
					"xyz": "https://hello.world/xyz/",
				},
			},
		},
		{
			name:    "Invalid config",
			path:    "/tmp/sirup_Test_readConfig_05.yaml",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("readConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func assertResponses(t *testing.T, got, want *http.Response) {
	t.Helper()

	if got.StatusCode != want.StatusCode {
		t.Errorf("StatusCode: got = %d, want = %d", got.StatusCode, want.StatusCode)
	}

	for name, wantVals := range want.Header {
		gotVals, found := got.Header[name]
		if !found || !reflect.DeepEqual(wantVals, gotVals) {
			t.Errorf("Header[%q]: got = %q, want = %q", name, gotVals, wantVals)
		}
	}
}

func updateResponse(r *http.Response, opts ...func(*http.Response)) *http.Response {
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func errorNilOrPanic(err error) {
	if nil != err {
		panic(err)
	}
}
