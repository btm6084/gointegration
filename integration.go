package gointegration

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/btm6084/gojson"
	"github.com/spf13/cast"
)

var (
	// To facilitate the ability to pass multiple query parameters with the same name,
	// parameters can be named as "paramName{number}"
	// e.g. www.example.com?filter=abc&filter=def
	// becomes: map[string]interface{}{"filter": "abc", "filter{1}": "def"}
	multiParamPattern = regexp.MustCompile("\\{\\d+\\}")

	defaultIDHeader = "X-Integration-Tests"
	defaultScheme   = "http"
	defaultHost     = "localhost"
	defaultPort     = 4080
	defaultTimeout  = 0
)

// Client parses a swagger.json document and exposes an interface for creating
// API calls to the endpoint specified.
type Client struct {
	FollowRedirects bool
	Hostname        string
	IdentityHeader  string
	Port            int
	Scheme          string
	Endpoints       map[string]Endpoints

	// Time is in MS
	Timeout int

	client *http.Client
}

// Endpoints is a collection of swagger endpoints
type Endpoints map[string]Route

// Route represents an API path.
type Route struct {
	ID         string               `json:"id"`
	Method     string               `json:"method"`
	Parameters map[string]ParamSpec `json:"parameters"`
	Path       string               `json:"path"`
	Produces   []string             `json:"produces"`
}

// ParamSpec represent API param specifications.
type ParamSpec struct {
	FoundIn     string `json:"found_in"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	ContentType string `json:"content_type"`
}

// BuildClient creates a new swagger dument from a file on the filesystem.
func BuildClient(path string) (*Client, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	scheme := defaultScheme
	if os.Getenv("SCHEME") != "" {
		scheme = os.Getenv("SCHEME")
	}

	host := defaultHost
	if os.Getenv("HOST") != "" {
		host = os.Getenv("HOST")
	}

	idHeader := defaultIDHeader
	if os.Getenv("IDENTITY") != "" {
		idHeader = os.Getenv("IDENTITY")
	}

	port := defaultPort
	if os.Getenv("PORT") != "" {
		var err error
		port, err = strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			fmt.Printf("Invalid Port '%s'.\n", os.Getenv("PORT"))
			port = defaultPort
		}
	}

	// Timeout should be an integer in milliseconds.
	timeout := defaultTimeout
	if os.Getenv("TIMEOUT") != "" {
		var err error
		timeout, err = strconv.Atoi(os.Getenv("TIMEOUT"))
		if err != nil {
			fmt.Printf("Invalid Timeout '%s'.\n", os.Getenv("TIMEOUT"))
			timeout = defaultTimeout
		}
	}

	sc := Client{}
	sc.Scheme = scheme
	sc.Hostname = host
	sc.IdentityHeader = idHeader
	sc.Port = port
	sc.Timeout = timeout

	sc.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if sc.FollowRedirects {
				return nil
			}
			return http.ErrUseLastResponse
		},
		Timeout: time.Duration(sc.Timeout) * time.Millisecond,
	}

	sc.load(data)

	return &sc, nil
}

func (sc *Client) load(data []byte) error {
	reader, err := gojson.NewJSONReader(data)
	if err != nil {
		return err
	}

	sc.Endpoints = make(map[string]Endpoints)

	paths := reader.Get("paths")
	for _, path := range paths.Keys {
		methods := paths.Get(path)

		for _, method := range methods.Keys {
			data := methods.Get(method)
			r := Route{}

			id := data.GetString("operationId")
			r.ID = id
			r.Method = method
			r.Path = path
			r.Produces = data.GetStringSlice("produces")

			paramList := data.GetCollection("parameters")
			r.Parameters = make(map[string]ParamSpec, len(paramList))
			for _, param := range paramList {
				var p ParamSpec
				p.FoundIn = param.GetString("in")
				p.Name = param.GetString("name")
				p.Required = param.GetBool("required")
				p.Type = param.GetString("type")

				r.Parameters[p.Name] = p
			}

			tags := data.GetStringSlice("tags")

			for _, t := range tags {
				if sc.Endpoints[t] == nil {
					sc.Endpoints[t] = make(Endpoints)
				}
				sc.Endpoints[t][r.ID] = r
			}

			if len(tags) == 0 {
				if sc.Endpoints["default"] == nil {
					sc.Endpoints["default"] = make(Endpoints)
				}
				sc.Endpoints["default"][r.ID] = r
			}

		}
	}

	return nil
}

// ExecJSON takes a path specifier and executes the corresponding functionality if found in the loaded swagger doc, returning a JSONResponse.
func (sc *Client) ExecJSON(specifier string, params map[string]interface{}) JSONResponse {

	resp := sc.Exec(specifier, params)
	reader, _ := gojson.NewJSONReader([]byte(resp.Body))

	return JSONResponse{
		ClientResponse: ClientResponse{
			Body:        resp.Body,
			Error:       resp.Error,
			Headers:     resp.Headers,
			Status:      resp.Status,
			RequestTime: resp.RequestTime,
			RequestURL:  resp.RequestURL,
			StatusCode:  resp.StatusCode,
		},
		Reader: reader,
	}
}

// Exec takes a path specifier and executes the corresponding functionality if found in the loaded swagger doc, returning a generic ClientResponse.
func (sc *Client) Exec(specifier string, params map[string]interface{}) ClientResponse {
	pieces := strings.Split(specifier, ".")
	var tag string
	var id string

	switch len(pieces) {
	default:
		return ClientResponse{Error: fmt.Errorf("Exec: Invalid path specifier %s", id)}
	case 1:
		tag = "default"
		id = pieces[0]
	case 2:
		tag = pieces[0]
		id = pieces[1]
	}

	if _, isset := sc.Endpoints[tag]; !isset {
		return ClientResponse{Error: fmt.Errorf("Exec: Route %s not found", specifier)}
	}

	if _, isset := sc.Endpoints[tag][id]; !isset {
		return ClientResponse{Error: fmt.Errorf("Exec: Route %s not found", specifier)}
	}

	route := sc.Endpoints[tag][id]

	// Reject if we're missing required parameters.
	for _, ps := range route.Parameters {
		if _, isset := params[ps.Name]; ps.Required && !isset {
			return ClientResponse{Error: fmt.Errorf("Exec: Required Parameter '%s' not provided", ps.Name)}
		}
	}

	var err error
	var postBody []byte
	var query []string
	headers := make(map[string]string)

	// Put the parameters into the correct place depending on the "in" value.
	for name, val := range params {
		// Remove any positional designators to allow for the same query parameter to be used multiple times.
		// Refer to comment on var declaration for multiParamPattern
		name = multiParamPattern.ReplaceAllString(name, "")

		if _, isset := route.Parameters[name]; !isset {
			return ClientResponse{Error: fmt.Errorf("[Extraneous Parameter] '%s.%s' has no parameter specification '%s'", tag, id, name)}
		}

		ps := route.Parameters[name]

		switch ps.FoundIn {
		case "path":
			route.Path = strings.Replace(route.Path, fmt.Sprintf("{%s}", name), url.PathEscape(cast.ToString(val)), -1)

		case "body":
			if reflect.TypeOf(val).String() == "[]uint8" {
				postBody = val.([]byte)
			} else {
				postBody, err = json.Marshal(val)
				if err != nil {
					return ClientResponse{Error: fmt.Errorf("Marshal of postBody failed with message: %s", err.Error())}
				}
			}

		case "query":
			query = append(query, fmt.Sprintf("%s=%s", name, url.QueryEscape(cast.ToString(val))))

		case "header":
			headers[name] = cast.ToString(val)

			//case "cookie":
			// @TODO
		}

	}

	// Construct the URL
	url := sc.buildURL(route.Path, query)

	// Build the request
	req, err := http.NewRequest(strings.ToUpper(route.Method), url, bytes.NewBuffer(postBody))
	if err != nil {
		return ClientResponse{Error: err}
	}

	// Set Content-Type header.
	contentType := "application/json"
	if len(route.Produces) > 0 {
		contentType = route.Produces[0]
	}
	req.Header.Set("Content-Type", contentType)

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Indentify ourself as an integration test to the service.
	// The service can handle / ignore this as it sees fit.
	req.Header.Set(sc.IdentityHeader, "true")

	// @TODO
	// Add cookies

	return sc.MakeRequest(req)
}

// buildURL returns the url based on the host, port, and scheme set in the Client
func (sc *Client) buildURL(path string, query []string) string {
	var separator string
	switch true {
	case len(query) > 0 && strings.Contains(path, "?"):
		separator = "&"
	case len(query) > 0:
		separator = "?"
	}

	path = strings.TrimLeft(path, "/")

	return fmt.Sprintf("%s://%s:%d/%s%s%s", sc.Scheme, sc.Hostname, sc.Port, path, separator, strings.Join(query, "&"))
}

// MakeRequest makes a request to a third party HTTP resource based on the given http.Request object.
func (sc *Client) MakeRequest(req *http.Request) ClientResponse {

	start := time.Now()
	res, err := sc.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return ClientResponse{Error: fmt.Errorf("Request to URL %s failed with error: %s", req.URL, err.Error())}
	}

	defer res.Body.Close()

	// Decompress gzip content
	var rawBody io.ReadCloser
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		rawBody, err = gzip.NewReader(res.Body)
		if err != nil {
			rawBody = res.Body
		}
	default:
		rawBody = res.Body
	}

	body, err := ioutil.ReadAll(rawBody)
	if err != nil {
		return ClientResponse{Error: fmt.Errorf("Unable to unpack body from request to URL %s: %s", req.URL, err.Error())}
	}

	headers := make(map[string]string)
	for k, set := range res.Header {
		if len(set) > 0 {
			headers[k] = set[0]
		}
	}

	out := ClientResponse{
		Body:        string(body),
		Cookies:     res.Cookies(),
		Error:       nil,
		Headers:     headers,
		RequestTime: fmt.Sprint(elapsed),
		RequestURL:  req.URL.String(),
		Status:      http.StatusText(res.StatusCode),
		StatusCode:  res.StatusCode,
	}

	return out
}
