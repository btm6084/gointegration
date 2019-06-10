# GoIntegration

GoIntegration is the Swagger Expectation Aassertion Runtime for go. GoIntegration is used to create integration test suites for ensuring your service API. GoIntegration reads your swagger.json file and exposes an interface for querying the API and setting expectations / assertions.

# Installation
go get github.com/btm6084/gointegration

# How To Use

The GoIntegration Client's Exec and ExecJSON functions perform the requested operation and return a ClientResponse or a JSONResponse, respectively. The Exec* functions take an operation ID which is in the form `tag.operationId`. In the sample swagger.json below, the /health endpoint has an operation ID of health.HealthCheck. When there are no tags defined, the tag "default" is used.

ClientResponse returns all the pertinent information about the API request enacted by calling Exec(). If any errors occured during the execution of the requested API endpoint, ClientResponse.Errors will be non-nil, containing instead the error that last occured.

JSONResponse provides everything that ClientResponse does, except that it also includes a JSONReader (github.com/btm6084/gojson) object which is pre-loaded with the body of the response. If the response was not JSON, JSONResponse.Errors will be non-nil. While you can access this directly to create whatever type of assertions you would like, the real power comes from the Expect* functions, which provide a very nice chained API for creating assertions about the response.

# Host, Port, and Scheme
When being constructed, Client will examine the local environment for HOST, PORT, and SCHEME. If any exist, they will override defaults.
Defaults are localhost, 4080, and http respectively.

# Parameters to the API
The parameter lists are parsed for each endpoint in the swagger.json file. Each parameter is passed by name in the map that is the second parameter to the Exec* functions. Missing required parameters will cause test failures. Extra parameters that a given endpoint doesn't specify will also cause a test failure.

# Example Use

Given a swagger.json file that looks like this:

```
{
   "host" : "localhost:4080",
   "paths" : {
      "/health" : {
         "get" : {
            "summary" : "Health monitoring endpoint",
            "operationId" : "HealthCheck",
            "tags" : [
               "health"
            ],
            "schemes" : [
               "http"
            ],
            "description" : "Returns a status of the health of the service",
            "produces" : [
               "application/hal+json"
            ],
            "responses" : {
               "200" : {
                  "$ref" : "#/responses/halResponseSwagger"
               }
            }
         }
      }
   },
   "basePath" : "/",
   "produces" : [
      "application/hal+json"
   ],
   "info" : {
      "title" : "GoIntegration Example Swagger File",
      "description" : ""
   },
   "swagger" : "2.0",
   "schemes" : [
      "http"
   ],
   "responses" : {
      "halResponseSwagger" : {
         "schema" : {
            "properties" : {
               "_embedded" : {
                  "type" : "object",
                  "x-go-name" : "Embeddded"
               },
               "_links" : {
                  "type" : "object",
                  "x-go-name" : "Links"
               },
               "_meta" : {
                  "x-go-name" : "Meta",
                  "type" : "object"
               }
            },
            "type" : "object"
         },
         "description" : "HAL only response containing api endpoints"
      }
   }
}
```

Which returns this JSON object:
```
{
	"name": "My Service Health Check",
	"version": "1.0.0",
	"checks": {
		"service": {
			"status": "OK",
			"message": "Service Ok"
		},
		"storage": {
			"status": "OK",
			"message": "Storage Ok"
		}
	}
}
```

We can construct an integration test that looks like this:
```
package integration

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/btm6084/gointegration"
)

func TestHealthEndpoint(t *testing.T) {
	client, err := gointegration.BuildClient("./swagger.json")
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	// Expect version 1
	versionRE := regexp.MustCompile(`1\.\d\.\d`)

	// Match a date in the format `Tue, 23 Jan 2018 01:06:08 GMT``
	dateRE   = regexp.MustCompile(`^[A-z]+, [0-9]{2} [A-z]+ \d{4} \d{2}:\d{2}:\d{2} [A-Z]+$`)

	client.ExecJSON("health.HealthCheck", map[string]interface{}{}).
		ExpectStatus(t, 200).
		ExpectValue(t, "name", "My Service Health Check").
		ExpectValueMatch(t, "version", versionRE).
		ExpectValue(t, "checks.service.status", "OK").
		ExpectValue(t, "checks.service.message", "Service Ok").
		ExpectValue(t, "checks.storage.status", "OK").
		ExpectValue(t, "checks.storage.message", "Storage Ok").
		ExpectHeaderValue(t, "Vary", "Accept-Encoding").
		ExpectHeaderValue(t, "Content-Type", "application/json").
		ExpectHeaderMatch(t, "Date", dateRE)

	// If you need something a little more complex, you can always take over control by capturing the response, instead of chaining it:
	resp := client.ExecJSON("health.HealthCheck", map[string]interface{}{})
	assert.Nil(t, resp.Error)

	version := resp.Reader.GetString("version")

	client.ExecJSON("example.TakesVersion", map[string]interface{}{
		"version": version,
	}).ExpectStatus(t, 201)
}

```
