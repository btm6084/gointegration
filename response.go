package gointegration

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/btm6084/gojson"
	"github.com/stretchr/testify/assert"
)

// ClientResponse holds the pertinent information returned from a third party request.
type ClientResponse struct {
	Body        string            `json:"body"`
	Cookies     []*http.Cookie    `json:"cookies"`
	Error       error             `json:"error"`
	Headers     map[string]string `json:"headers"`
	RequestTime string            `json:"request_time"`
	RequestURL  string            `json:"request_url"`
	Status      string            `json:"status"`
	StatusCode  int               `json:"status_code"`
}

// ExpectError is used to assert that a certain error condition has occured.
func (c ClientResponse) ExpectError(t *testing.T, err error) ClientResponse {
	// To avoid a panic inside assert, we will handle nil values explicitly
	if err == nil {
		if c.Error == nil {
			return c
		}

		assert.True(t, false, fmt.Sprintf("expected no error, got error `%v` instead", c.Error))
		return c
	}

	if c.Error == nil {
		assert.True(t, false, fmt.Sprintf("expected error `%v`, had nil instead", err))
		return c
	}

	assert.Equal(t, err, c.Error, fmt.Sprintf("expected error with message `%v`, got error with message `%v`", err, c.Error))

	return c
}

// Expect allows custom assertions to be run.
// A error returned from the eval function will cause the test to be failed.
func (c ClientResponse) Expect(t *testing.T, eval func(c ClientResponse) error) ClientResponse {
	if c.Error != nil {
		return c
	}

	err := eval(c)

	msg := ""
	if err != nil {
		msg = err.Error()
	}
	assert.Nil(t, err, msg)

	return c
}

// ExpectStatus asserts that a specific status code was received.
func (c ClientResponse) ExpectStatus(t *testing.T, status int) ClientResponse {
	if c.Error != nil {
		return c
	}

	assert.Equal(t, status, c.StatusCode, fmt.Sprintf("expected statuscode '%d', got '%d' instead", status, c.StatusCode))

	return c
}

// ExpectHeaderEmpty asserts that there was no header value set at a given key.
func (c ClientResponse) ExpectHeaderEmpty(t *testing.T, key string) ClientResponse {
	if c.Error != nil {
		return c
	}

	if _, isset := c.Headers[key]; !isset {
		return c
	}

	assert.Fail(t, fmt.Sprintf("expected no header with key '%s' set", key))

	return c
}

// ExpectHeaderValue asserts that the header value at the given key will match the given value.
func (c ClientResponse) ExpectHeaderValue(t *testing.T, key string, value string) ClientResponse {
	if c.Error != nil {
		return c
	}

	if _, isset := c.Headers[key]; !isset {
		assert.True(t, isset, fmt.Sprintf("no header with key '%s' set", key))
		return c
	}

	assert.Equal(t, value, c.Headers[key], fmt.Sprintf("expected header '%s' to have value '%s', got '%s' instead", key, value, c.Headers[key]))

	return c
}

// OptionalHeaderValue differs from ExpectHeaderValue in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c ClientResponse) OptionalHeaderValue(t *testing.T, key string, value string) ClientResponse {
	if _, isset := c.Headers[key]; !isset {
		return c
	}

	return c.ExpectHeaderValue(t, key, value)
}

// ExpectHeaderMatch asserts that the header value at the given key will match the given regular expression.
func (c ClientResponse) ExpectHeaderMatch(t *testing.T, key string, re *regexp.Regexp) ClientResponse {
	if c.Error != nil {
		return c
	}

	if _, isset := c.Headers[key]; !isset {
		assert.True(t, isset, fmt.Sprintf("no header with key '%s' set", key))
		return c
	}

	val := c.Headers[key]
	assert.True(t, re.Match([]byte(val)), fmt.Sprintf("expect header match error: '%s' did not pass the regex test `%s`", val, re.String()))

	return c
}

// OptionalHeaderMatch differs from ExpectHeaderMatch in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c ClientResponse) OptionalHeaderMatch(t *testing.T, key string, re *regexp.Regexp) ClientResponse {
	if _, isset := c.Headers[key]; !isset {
		return c
	}

	return c.ExpectHeaderMatch(t, key, re)
}

// JSONResponse is a ClientResponse with added functionality specifically for dealing with json API responses.
type JSONResponse struct {
	ClientResponse
	Reader *gojson.JSONReader `json:"-"`
}

// ExpectError is used to assert that a certain error condition has occured.
func (c JSONResponse) ExpectError(t *testing.T, err error) JSONResponse {
	// To avoid a panic inside assert, we will handle nil values explicitly
	if err == nil {
		if c.Error == nil {
			return c
		}

		assert.True(t, false, fmt.Sprintf("expected no error, got error `%v` instead", c.Error))
		return c
	}

	if c.Error == nil {
		assert.True(t, false, fmt.Sprintf("expected error `%v`, had nil instead", err))
		return c
	}

	assert.Equal(t, err, c.Error, fmt.Sprintf("expected error with message `%v`, got error with message `%v`", err, c.Error))

	return c
}

// Expect allows custom assertions to be run.
// A error returned from the eval function will cause the test to be failed.
func (c JSONResponse) Expect(t *testing.T, eval func(c JSONResponse) error) JSONResponse {
	if c.Error != nil {
		return c
	}

	err := eval(c)

	msg := ""
	if err != nil {
		msg = err.Error()
	}
	assert.Nil(t, err, msg)

	return c
}

// ExpectStatus asserts that a specific status code was received.
func (c JSONResponse) ExpectStatus(t *testing.T, status int) JSONResponse {
	if c.Error != nil {
		return c
	}

	assert.Equal(t, status, c.StatusCode, fmt.Sprintf("expected statuscode '%d', got '%d' instead", status, c.StatusCode))

	return c
}

// ExpectType asserts the data type at the given key will match the given JSON data type.
func (c JSONResponse) ExpectType(t *testing.T, key, typ string) JSONResponse {
	if c.Error != nil {
		return c
	}

	r := c.Reader.Get(key)

	// Allow for int or float when it's not important.
	if typ == "number" && (r.Type == gojson.JSONInt || r.Type == gojson.JSONFloat) {
		return c
	}

	assert.Equal(t, typ, r.Type, fmt.Sprintf("expected value at key `%s` to be `%s`, got `%s` instead", key, typ, r.Type))

	return c
}

// ExpectTypes asserts the data type at the given key will match the given JSON data types.
func (c JSONResponse) ExpectTypes(t *testing.T, key string, typ ...string) JSONResponse {
	if c.Error != nil {
		return c
	}

	r := c.Reader.Get(key)

	for _, check := range typ {
		if check == r.Type {
			return c
		}

		// Allow for int or float when it's not important.
		if check == "number" && (r.Type == gojson.JSONInt || r.Type == gojson.JSONFloat) {
			return c
		}
	}

	assert.Equal(t, typ, r.Type, fmt.Sprintf("expected value at key `%s` to be `%s`, got `%s` instead", key, strings.Join(typ, `, `), r.Type))

	return c
}

// OptionalType differs from ExpectType in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c JSONResponse) OptionalType(t *testing.T, key, typ string) JSONResponse {
	if !c.Reader.KeyExists(key) {
		return c
	}

	return c.ExpectType(t, key, typ)
}

// OptionalTypes differs from ExpectTypes in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c JSONResponse) OptionalTypes(t *testing.T, key string, typ ...string) JSONResponse {
	if !c.Reader.KeyExists(key) {
		return c
	}

	return c.ExpectTypes(t, key, typ...)
}

// ExpectValue asserts the value at the given key will match the given value.
func (c JSONResponse) ExpectValue(t *testing.T, key string, b interface{}) JSONResponse {
	if c.Error != nil {
		return c
	}

	a := c.Reader.GetInterface(key)
	assert.Equal(t, b, a, fmt.Sprintf("expected '%s' to equal '%s'", b, a))

	return c
}

// ExpectValueString asserts the value at the given key will match the given value. All comparisons are done as string comparisons.
func (c JSONResponse) ExpectValueString(t *testing.T, key, b string) JSONResponse {
	if c.Error != nil {
		return c
	}

	a := c.Reader.GetString(key)
	assert.Equal(t, b, a, fmt.Sprintf("expected '%s' to equal '%s'", b, a))

	return c
}

// OptionalValue differs from ExpectValue in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c JSONResponse) OptionalValue(t *testing.T, key string, b interface{}) JSONResponse {
	if !c.Reader.KeyExists(key) {
		return c
	}

	return c.ExpectValue(t, key, b)
}

// ExpectValueMatch asserts that the value at the given key will match the given regular expression.
func (c JSONResponse) ExpectValueMatch(t *testing.T, key string, re *regexp.Regexp) JSONResponse {
	if c.Error != nil {
		return c
	}

	val := c.Reader.GetString(key)
	assert.True(t, re.Match([]byte(val)), fmt.Sprintf("expect value match error: '%s' did not pass the regex test `%s`", val, re.String()))

	return c
}

// OptionalValueMatch differs from ExpectValueMatch in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c JSONResponse) OptionalValueMatch(t *testing.T, key string, re *regexp.Regexp) JSONResponse {
	if !c.Reader.KeyExists(key) {
		return c
	}

	return c.ExpectValueMatch(t, key, re)
}

// ExpectValueCountCompare asserts the aggregate data type at the given key will have the given number of child nodes.
func (c JSONResponse) ExpectValueCountCompare(t *testing.T, key string, comp string, count int) JSONResponse {
	if c.Error != nil {
		return c
	}

	r := c.Reader.Get(key)

	switch comp {
	case "=":
		assert.Equal(t, count, len(r.Keys), fmt.Sprintf("expected count to not be %d items, found %d", count, len(r.Keys)))
	case "!=":
		assert.NotEqual(t, count, len(r.Keys), fmt.Sprintf("expected exactly %d items, found %d", count, len(r.Keys)))
	case ">":
		assert.True(t, len(r.Keys) > count, fmt.Sprintf("expected more than %d items, found %d", count, len(r.Keys)))
	case ">=":
		assert.True(t, len(r.Keys) >= count, fmt.Sprintf("expected at least %d items, found %d", count, len(r.Keys)))
	case "<":
		assert.True(t, len(r.Keys) < count, fmt.Sprintf("expected less than %d items, found %d", count, len(r.Keys)))
	case "<=":
		assert.True(t, len(r.Keys) <= count, fmt.Sprintf("expected a minimum of %d items, found %d", count, len(r.Keys)))

	}

	return c
}

// ExpectValueCount asserts the aggregate data type at the given key will have the given number of child nodes.
func (c JSONResponse) ExpectValueCount(t *testing.T, key string, count int) JSONResponse {
	if c.Error != nil {
		return c
	}

	r := c.Reader.Get(key)
	assert.Equal(t, count, len(r.Keys), fmt.Sprintf("expected exactly %d items, found %d", count, len(r.Keys)))

	return c
}

// ExpectHeaderEmpty asserts that there was no header value set at a given key.
func (c JSONResponse) ExpectHeaderEmpty(t *testing.T, key string) JSONResponse {
	if c.Error != nil {
		return c
	}

	if _, isset := c.Headers[key]; !isset {
		return c
	}

	assert.Fail(t, fmt.Sprintf("expected no header with key '%s' set", key))

	return c
}

// ExpectHeaderValue asserts that the header value at the given key will match the given value.
func (c JSONResponse) ExpectHeaderValue(t *testing.T, key string, value string) JSONResponse {
	if c.Error != nil {
		return c
	}

	if _, isset := c.Headers[key]; !isset {
		assert.True(t, isset, fmt.Sprintf("no header with key '%s' set", key))
		return c
	}

	assert.Equal(t, value, c.Headers[key], fmt.Sprintf("expected header '%s' to have value '%s', got '%s' instead", key, value, c.Headers[key]))

	return c
}

// OptionalHeaderValue differs from ExpectHeaderValue in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c JSONResponse) OptionalHeaderValue(t *testing.T, key string, value string) JSONResponse {
	if _, isset := c.Headers[key]; !isset {
		return c
	}

	return c.ExpectHeaderValue(t, key, value)
}

// ExpectHeaderMatch asserts that the header value at the given key will match the given regular expression.
func (c JSONResponse) ExpectHeaderMatch(t *testing.T, key string, re *regexp.Regexp) JSONResponse {
	if c.Error != nil {
		return c
	}

	if _, isset := c.Headers[key]; !isset {
		assert.True(t, isset, fmt.Sprintf("no header with key '%s' set", key))
		return c
	}

	val := c.Headers[key]
	assert.True(t, re.Match([]byte(val)), fmt.Sprintf("expect header match error: '%s' did not pass the regex test `%s`", val, re.String()))

	return c
}

// OptionalHeaderMatch differs from ExpectHeaderMatch in that it can only fail if the given key exists. If the key is missing entirely, the test will pass.
func (c JSONResponse) OptionalHeaderMatch(t *testing.T, key string, re *regexp.Regexp) JSONResponse {
	if _, isset := c.Headers[key]; !isset {
		return c
	}

	return c.ExpectHeaderMatch(t, key, re)
}
