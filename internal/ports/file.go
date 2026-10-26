package ports

import "os"

// FileInput connects a file without transferring ownership unless requested.
func FileInput(f *os.File, binary, owned bool) *Resource {
	stream, probe := fileInputStream(f)
	options := Options{Filename: f.Name(), Ready: probe}
	if owned {
		options.Owned = f
	}
	return Input(stream, binary, options)
}
