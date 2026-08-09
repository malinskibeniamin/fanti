package seed

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCEDICTRejectsEmptyDataset(t *testing.T) {
	t.Parallel()

	_, err := ParseCEDICT(strings.NewReader("# comments only\n"))
	if !errors.Is(err, errEmptyCEDICT) {
		t.Fatalf("ParseCEDICT() error = %v, want errEmptyCEDICT", err)
	}
}
