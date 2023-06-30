package pump

import (
	"regexp"
	"testing"
)

func TestIncludeSchema(t *testing.T) {
	_globalInclude = regexp.MustCompile("^test$")
	if includeSchema("test_send") {
		t.Fatal()
	}
}