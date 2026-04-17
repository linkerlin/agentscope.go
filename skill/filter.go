package skill

// FileFilter decides whether a skill resource should be uploaded.
type FileFilter func(resourcePath string) bool

// DefaultFileFilter returns a filter that matches specific folders and extensions.
func DefaultFileFilter(folders, extensions []string) FileFilter {
	return func(path string) bool {
		if path == "" {
			return false
		}
		normalized := path
		for _, f := range folders {
			if len(f) > 0 && len(normalized) >= len(f) && normalized[:len(f)] == f {
				return true
			}
		}
		for _, ext := range extensions {
			if len(ext) > 0 && len(normalized) >= len(ext) && normalized[len(normalized)-len(ext):] == ext {
				return true
			}
		}
		return false
	}
}

// AcceptAllFilter accepts every resource path.
func AcceptAllFilter() FileFilter {
	return func(string) bool { return true }
}
