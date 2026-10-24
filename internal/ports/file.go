package ports

import "os"

// FileInput connects a file without transferring ownership unless requested.
func FileInput(f *os.File, binary, owned bool) *Resource {
	options := Options{Ready: func() (bool, error) { return fileReady(f) }}
	if owned {
		options.Owned = f
	}
	return Input(f, binary, options)
}
