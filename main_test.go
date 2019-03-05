package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/Masterminds/sprig"
	"github.com/alexsasharegan/dotenv"
	"github.com/qri-io/jsonschema"
)

var (
	waitExtra  int64
	waitForUrl string

	data []HttpTestData
)

func init() {
	flag.Int64Var(&waitExtra, "wait-extra", 0, "Seconds to wait regardless of -wait-for-url status [env: BLACKBOX_WAIT_EXTRA]")
	flag.StringVar(&waitForUrl, "wait-for-url", "", "Wait for this url to become available (status 200) [env: BLACKBOX_WAIT_FOR_URL]")
}

type HttpRequestData struct {
	Content string `yaml:"content"`
}

type FromRequest struct {
	URL             string            `yaml:"url"`
	Method          string            `yaml:"method"`
	Data            HttpRequestData   `yaml:"data"`
	FollowRedirects bool              `yaml:"followRedirects"`
	BasicAuth       []string          `yaml:"basicAuth"`
	Headers         map[string]string `yaml:"headers"`
	HeaderWhitelist []string          `yaml:"headerWhitelist"`
}

type HttpTestData struct {
	Name            string             `yaml:"name"`
	URL             string             `yaml:"url"`
	Method          string             `yaml:"method"`
	Data            HttpRequestData    `yaml:"data"`
	FollowRedirects bool               `yaml:"followRedirects"`
	BasicAuth       []string           `yaml:"basicAuth"`
	Headers         map[string]string  `yaml:"headers"`
	Expect          HttpExpectTestData `yaml:"expect"`
}

type BodyData struct {
	Content    string `yaml:"content"`
	Regex      string `yaml:"regex"`
	Empty      bool   `yaml:"empty"`
	JsonSchema string `yaml:"jsonSchema"`
}

type HttpExpectTestData struct {
	Status      int               `yaml:"status"`
	Body        BodyData          `yaml:"body"`
	Headers     map[string]string `yaml:"headers"`
	FromRequest *FromRequest      `yaml:"fromRequest"`
}

func doRequest(t *testing.T, data *HttpTestData) (*http.Response, string, error) {
	var method string
	var reqBody io.Reader

	if data.Method != "" {
		method = data.Method
	} else if data.Data.Content != "" {
		method = "POST"
		reqBody = strings.NewReader(data.Data.Content)
		if _, ok := data.Headers["content-type"]; !ok {
			if data.Headers == nil {
				data.Headers = make(map[string]string)
			}
			data.Headers["content-type"] = "application/x-www-form-urlencoded"
		}
	} else {
		method = "GET"
	}

	req, err := http.NewRequest(method, data.URL, reqBody)
	if err != nil {
		return nil, "", err
	}

	for k, v := range data.Headers {
		req.Header.Add(k, v)
	}

	if len(data.BasicAuth) == 2 {
		req.SetBasicAuth(data.BasicAuth[0], data.BasicAuth[1])
	}

	var checkRedirect func(req *http.Request, via []*http.Request) error
	if !data.FollowRedirects {
		checkRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	client := http.Client{CheckRedirect: checkRedirect}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return resp, string(body), nil
}

func runTest(t *testing.T, data HttpTestData) {
	t.Log("URL:", data.URL)

	resp, body, err := doRequest(t, &data)
	if err != nil {
		t.Errorf("Test %s failed making request; %v", data.Name, err)
		return
	}

	if data.Expect.FromRequest != nil {
		req := HttpTestData{
			URL:             data.Expect.FromRequest.URL,
			Method:          data.Expect.FromRequest.Method,
			Data:            data.Expect.FromRequest.Data,
			FollowRedirects: data.Expect.FromRequest.FollowRedirects,
			BasicAuth:       data.Expect.FromRequest.BasicAuth,
			Headers:         data.Expect.FromRequest.Headers,
		}

		t.Log("Fetching fromRequest;", req.URL)
		expectResp, expectBody, err := doRequest(t, &req)
		if err != nil {
			t.Errorf("Test %s failed making fromRequest; %v", data.Name, err)
			return
		}

		data.Expect.Status = expectResp.StatusCode
		data.Expect.Body.Content = expectBody

		if data.Expect.Headers == nil {
			data.Expect.Headers = map[string]string{}
		}

		for _, h := range data.Expect.FromRequest.HeaderWhitelist {
			data.Expect.Headers[h] = expectResp.Header.Get(h)
		}
	}

	if data.Expect.Status != 0 {
		if resp.StatusCode != data.Expect.Status {
			t.Errorf("got status: '%+v', want: '%d'", resp.StatusCode, data.Expect.Status)
		}
	}

	if len(data.Expect.Headers) != 0 {
		for h, v := range data.Expect.Headers {
			if got := resp.Header.Get(h); got != v {
				t.Errorf("got: '%s', want header '%s': '%s'", got, h, v)
			}
		}
	}

	if data.Expect.Body.Empty {
		if body != "" {
			t.Errorf("got: '%s', want: '<empty>'", body)
		}
	}

	if data.Expect.Body.Content != "" {
		if body != data.Expect.Body.Content {
			t.Errorf("got: '%s', want: '%s'", body, data.Expect.Body.Content)
		}

	}

	if data.Expect.Body.Regex != "" {
		matched, err := regexp.MatchString(data.Expect.Body.Regex, body)
		if err != nil {
			t.Error("failed to use regex to match;", err)
		}

		if !matched {
			t.Errorf("got: '%s', want match: '%s'", body, data.Expect.Body.Regex)
		}
	}

	if data.Expect.Body.JsonSchema != "" {
		assertJsonSchema(t, data.Expect.Body.JsonSchema, body)
	}
}

func assertJsonSchema(t *testing.T, rawSchema string, body string) {
	rs := &jsonschema.RootSchema{}
	if err := json.Unmarshal([]byte(rawSchema), rs); err != nil {
		t.Fatal(err)
		return
	}

	if errs, _ := rs.ValidateBytes([]byte(body)); len(errs) > 0 {
		for _, err := range errs {
			t.Error("schema:", err)
		}
	}
}

func TestDataTests(t *testing.T) {
	for _, test := range data {
		t.Run(test.Name, func(t *testing.T) {
			runTest(t, test)
		})
	}
}

func getTestsFromFile(filename string) []HttpTestData {
	var yamlData []byte
	var err error

	if filename == "-" {
		yamlData, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal("failed to read stdin", err)
		}
	} else {
		yamlData, err = ioutil.ReadFile(filename)
		if err != nil {
			log.Fatal("failed to open file", filename, err)
		}
	}

	funcs := sprig.TxtFuncMap()
	funcs["urlencode"] = url.QueryEscape

	tmpl, err := template.New(filename).Funcs(funcs).Parse(string(yamlData))
	if err != nil {
		log.Fatal("failed to parse yaml as template")
	}

	var yamlBuf []byte
	buf := bytes.NewBuffer(yamlBuf)

	tmpl.Execute(buf, nil)

	td := []HttpTestData{}

	err = yaml.Unmarshal([]byte(buf.String()), &td)
	if err != nil {
		log.Fatalf("unmarshal test data: %v", err)
	}

	return td
}

func wait(url string) {
	if url != "" {
		func() {
			log.Println("Waiting for URL to return 200;", url)

			for wait := 30; wait > 0; wait-- {
				resp, err := http.Get(url)
				if err == nil && resp.StatusCode == 200 {
					// Sleep a moment more to be sure.
					time.Sleep(time.Second)
					log.Println("Wait URL appears to be ready")
					return
				}

				if err != nil {
					log.Println("Not ready;", err)
				} else if resp.StatusCode != 200 {
					log.Printf("Not ready; status: %d", resp.StatusCode)
				}

				time.Sleep(time.Second)
			}

			log.Println("Wait URL took too long to become ready. Exiting with error.")

			os.Exit(69)
		}()
	}

	if waitExtra > 0 {
		log.Printf("Waiting extra as requested (%ds)...\n", waitExtra)
		time.Sleep(time.Duration(waitExtra) * time.Second)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if !os.IsNotExist(err) {
		log.Println("exists:", err)
	}
	return false
}

func TestMain(m *testing.M) {
	flag.Parse()

	var err error

	if exists(".env") {
		err = dotenv.Load()
		if err != nil {
			log.Fatal(err)
		}
	}

	if waitForUrl == "" {
		if v := os.Getenv("BLACKBOX_WAIT_FOR_URL"); v != "" {
			waitForUrl = v
		}
	}

	if waitExtra == 0 {
		if v := os.Getenv("BLACKBOX_WAIT_EXTRA"); v != "" {
			waitExtra, err = strconv.ParseInt(v, 10, 0)
			if err != nil {
				log.Fatal("BLACKBOX_WAIT_EXTRA:", err)
			}
		}
	}
	if waitForUrl != "" {
		if u, err := url.Parse(waitForUrl); err != nil || u.Scheme == "" || u.Host == "" {
			if err == nil {
				err = errors.New("-wait-for-url must be a valid http or https URL")
			}
			log.Fatal("wait for url:", err)
		}
	}

	args := flag.Args()
	if len(args) == 0 {
		var err error
		args, err = filepath.Glob("test*.yaml")
		if err != nil {
			log.Fatal("failed to find files:", err)
		}
	} else if len(args) == 1 && args[0] == "-" {
		log.Println("Loading from stdin")
		data = getTestsFromFile("-")
		args = []string{}
	}

	for _, file := range args {
		log.Println("Loading", file)
		data = append(data, getTestsFromFile(file)...)
	}

	wait(waitForUrl)

	os.Exit(m.Run())
}
