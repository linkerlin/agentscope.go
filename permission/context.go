package permission

// Context holds the permission configuration: mode, working directories,
// and dangerous path lists. It is used by the Engine for tool-specific
// safety checks and ACCEPT_EDITS mode logic.
type Context struct {
	Mode           Mode
	WorkingDirs    []string // Additional directories allowed in ACCEPT_EDITS mode
	DangerousFiles []string // Sensitive files protected from auto-editing
	DangerousDirs  []string // Sensitive directories protected from auto-editing
}

// NewContext creates a permission context with sensible defaults.
func NewContext(mode Mode) *Context {
	if mode == "" {
		mode = ModeExplore
	}
	return &Context{
		Mode:           mode,
		DangerousFiles: DefaultDangerousFiles,
		DangerousDirs:  DefaultDangerousDirs,
	}
}

// WithWorkingDirs sets additional working directories for ACCEPT_EDITS mode.
func (c *Context) WithWorkingDirs(dirs ...string) *Context {
	c.WorkingDirs = append(c.WorkingDirs, dirs...)
	return c
}

// WithDangerousFiles overrides the list of sensitive files.
func (c *Context) WithDangerousFiles(files ...string) *Context {
	c.DangerousFiles = files
	return c
}

// WithDangerousDirs overrides the list of sensitive directories.
func (c *Context) WithDangerousDirs(dirs ...string) *Context {
	c.DangerousDirs = dirs
	return c
}
