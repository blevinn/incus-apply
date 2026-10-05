// Package apply exposes incus-apply reconciliation as an embeddable Go API.
//
// The current default backend preserves upstream behavior by invoking the Incus
// CLI internally. That backend is transitional; callers should not depend on
// command execution details. The public API is designed so a native Incus Go
// backend can replace it without changing consumers.
package apply

import (
	"fmt"
	"io"
	"time"

	internalapply "github.com/abiosoft/incus-apply/internal/apply"
	internalincus "github.com/abiosoft/incus-apply/internal/incus"
)

type Operation string

const (
	Upsert Operation = "upsert"
	Delete Operation = "delete"
	Reset  Operation = "reset"
)

type Options struct {
	Operation       Operation
	Project         string
	Remote          string
	Replace         bool
	ShowEnv         bool
	Stop            bool
	NoLaunch        bool
	FailFast        bool
	NoWaitCloudInit bool
	ForceLocal      bool
	CommandTimeout  time.Duration
}

type Change struct {
	Path   string `json:"path"`
	Old    any    `json:"old,omitempty"`
	New    any    `json:"new,omitempty"`
	Action string `json:"action"`
}

type Item struct {
	ResourceID string   `json:"resource_id"`
	Changes    []Change `json:"changes,omitempty"`
	Note       string   `json:"note,omitempty"`
}

type Group struct {
	Action string `json:"action"`
	Items  []Item `json:"items"`
}

type Preview struct {
	ResourceCount int     `json:"resource_count,omitempty"`
	Groups        []Group `json:"groups"`
	Summary       string  `json:"summary,omitempty"`
}

type Result struct {
	Preview Preview
}

type backendFactory func(Options) internalincus.Client

type Client struct {
	options Options
	backend backendFactory
}

func New(options Options) *Client {
	return newClient(options, newCommandBackend)
}

func NewNative(options Options) *Client {
	return newClient(options, newNativeBackend)
}

func newClient(options Options, backend backendFactory) *Client {
	if options.Operation == "" {
		options.Operation = Upsert
	}
	return &Client{
		options: options,
		backend: backend,
	}
}

func (c *Client) Plan(reader io.Reader) (Preview, error) {
	return c.run(reader, true)
}

func (c *Client) Execute(reader io.Reader) (Result, error) {
	preview, err := c.run(reader, false)
	return Result{Preview: preview}, err
}

func (c *Client) run(reader io.Reader, planOnly bool) (Preview, error) {
	if reader == nil {
		return Preview{}, fmt.Errorf("configuration reader must not be nil")
	}
	if err := validateOperation(c.options.Operation); err != nil {
		return Preview{}, err
	}

	client := c.backend(c.options)
	if err := client.Ping(); err != nil {
		return Preview{}, err
	}

	renderer := &captureRenderer{}
	opts := internalapply.Options{
		Reader:          reader,
		CommandTimeout:  c.options.CommandTimeout,
		Project:         c.options.Project,
		Remote:          c.options.Remote,
		Replace:         c.options.Replace,
		ShowEnv:         c.options.ShowEnv,
		Stop:            c.options.Stop,
		Launch:          !c.options.NoLaunch,
		FailFast:        c.options.FailFast,
		NoWaitCloudInit: c.options.NoWaitCloudInit,
		ForceLocal:      c.options.ForceLocal,
		Quiet:           true,
		Yes:             true,
	}
	if planOnly {
		opts.Diff = "json"
	}

	executor := internalapply.NewExecutor(opts, client, renderer)
	var err error
	switch c.options.Operation {
	case Upsert:
		err = executor.Upsert()
	case Delete:
		err = executor.Delete()
	case Reset:
		err = executor.Reset()
	}
	return copyPreview(renderer.output), err
}

func validateOperation(operation Operation) error {
	switch operation {
	case Upsert, Delete, Reset:
		return nil
	default:
		return fmt.Errorf("unknown apply operation %q", operation)
	}
}

func newNativeBackend(options Options) internalincus.Client {
	return internalincus.NewNative(options.Remote, options.Stop)
}

func newCommandBackend(options Options) internalincus.Client {
	var globalFlags []string
	if options.ForceLocal {
		globalFlags = append(globalFlags, "--force-local")
	}
	return internalincus.New(globalFlags, options.Remote, options.Stop, false, options.CommandTimeout)
}

type captureRenderer struct {
	output internalapply.Output
}

func (r *captureRenderer) Render(output internalapply.Output) error {
	r.output = output
	return nil
}

func copyPreview(output internalapply.Output) Preview {
	preview := Preview{
		ResourceCount: output.ResourceCount,
		Summary:       output.Summary,
		Groups:        make([]Group, 0, len(output.Groups)),
	}
	for _, group := range output.Groups {
		copied := Group{
			Action: string(group.Action),
			Items:  make([]Item, 0, len(group.Items)),
		}
		for _, item := range group.Items {
			out := Item{
				ResourceID: item.ResourceID,
				Note:       item.Note,
				Changes:    make([]Change, 0, len(item.Changes)),
			}
			for _, change := range item.Changes {
				out.Changes = append(out.Changes, Change{
					Path:   change.Path,
					Old:    change.Old,
					New:    change.New,
					Action: change.Action,
				})
			}
			copied.Items = append(copied.Items, out)
		}
		preview.Groups = append(preview.Groups, copied)
	}
	return preview
}
