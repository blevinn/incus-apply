package apply

import (
	"io"
	"time"
)

// Options holds all CLI flags and configuration options.
type Options struct {
	// Reader supplies one in-memory configuration stream. When set it is loaded
	// before file/URL inputs and does not require stdin or a temporary file.
	Reader io.Reader

	// Input sources (positional arguments: files, directories, URLs, or '-' for stdin)
	Files     []string
	Recursive bool
	// Remote fetch timeout for URL-based configs. Zero disables the timeout.
	FetchTimeout time.Duration
	// Command execution timeout. Zero disables the timeout.
	CommandTimeout time.Duration

	// Operation modes
	Delete  bool
	Reset   bool
	Select  bool
	Yes     bool
	Diff    string
	Replace bool
	// RejectUnsupportedChanges fails planning when create-only drift cannot be
	// applied in place and Replace is not enabled.
	RejectUnsupportedChanges bool
	// RequireExistingConfig requires existing resources to contain the given
	// config key/value pairs before they may be updated or deleted. Creation is
	// unaffected. A nil or empty map preserves the default adoption behavior.
	RequireExistingConfig map[string]string
	ShowEnv               bool
	Stop                  bool
	Launch                bool
	FailFast              bool
	NoWaitCloudInit       bool

	// Internal state (not flags)
	FileCount int

	// Incus flags (passed through to incus commands)
	Project string
	// Remote is the target Incus remote server. Empty means the default remote configured
	// for the incus client is used. A remote specified per-resource (via "remote:name" in the
	// resource's name field) takes precedence over this value.
	Remote     string
	Verbose    bool
	Quiet      bool
	ForceLocal bool
}

// IsDiffOnly returns true when the user only wants to see the diff (no apply).
func (o Options) IsDiffOnly() bool {
	return o.Diff != ""
}

// IsJSONDiff returns true when the diff should be rendered as JSON.
func (o Options) IsJSONDiff() bool {
	return o.Diff == "json"
}
