package httpapi

import (
	"errors"
	"net/http"
	"testing"
)

func TestStorageSecurityErrorsReturnForbidden(t *testing.T) {
	for _, message := range []string{
		"path is outside configured storage roots",
		"path escapes configured storage root through a symbolic link",
		"symbolic links cannot be archived",
		"configured storage root cannot be deleted",
	} {
		if got := storageErrorStatus(errors.New(message)); got != http.StatusForbidden {
			t.Fatalf("%q status = %d, want 403", message, got)
		}
	}
}
