package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ValidateHttpCodes(t *testing.T) {
	assert.Equal(t, true, IsSuccessHTTPCode([]string{"2xx", "4xx"}, "200"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{"2xx"}, "300"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{"20x"}, "301"))
	assert.Equal(t, true, IsSuccessHTTPCode([]string{"401"}, "401"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{"402"}, "401"))
	assert.Equal(t, true, IsSuccessHTTPCode([]string{"xxx"}, "503"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{""}, "503"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{""}, "50"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{""}, "5"))
	assert.Equal(t, true, IsSuccessHTTPCode([]string{""}, ""))
	assert.Equal(t, true, IsSuccessHTTPCode([]string{"xx"}, "50"))
	assert.Equal(t, false, IsSuccessHTTPCode([]string{"x"}, "50"))
	assert.Equal(t, true, IsSuccessHTTPCode([]string{"x"}, "5"))
}
