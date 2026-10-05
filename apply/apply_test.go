package apply

import (
	"strings"
	"testing"
	"time"

	internalconfig "github.com/abiosoft/incus-apply/internal/config"
	internalincus "github.com/abiosoft/incus-apply/internal/incus"
)

type fakeBackend struct {
	created int
}

func (f *fakeBackend) Ping() error { return nil }
func (f *fakeBackend) Create(*internalconfig.Resource) *internalincus.Result {
	f.created++
	return &internalincus.Result{}
}
func (f *fakeBackend) Update(*internalconfig.Resource) *internalincus.Result {
	return &internalincus.Result{}
}
func (f *fakeBackend) Delete(*internalconfig.Resource) *internalincus.Result {
	return &internalincus.Result{}
}
func (f *fakeBackend) Exists(*internalconfig.Resource) (bool, error) {
	return false, nil
}
func (f *fakeBackend) CurrentConfig(*internalconfig.Resource) (string, error) {
	return "", nil
}
func (f *fakeBackend) MergedConfig(*internalconfig.Resource) (string, error) {
	return "", nil
}
func (f *fakeBackend) Start(*internalconfig.Resource) *internalincus.Result {
	return &internalincus.Result{}
}
func (f *fakeBackend) Stop(*internalconfig.Resource) *internalincus.Result {
	return &internalincus.Result{}
}
func (f *fakeBackend) Running(*internalconfig.Resource) bool { return false }
func (f *fakeBackend) WaitInstanceAgent(*internalconfig.Resource) *internalincus.Result {
	return &internalincus.Result{}
}
func (f *fakeBackend) WaitCloudInit(*internalconfig.Resource) *internalincus.Result {
	return &internalincus.Result{}
}

func TestPlanDoesNotMutate(t *testing.T) {
	backend := &fakeBackend{}
	client := New(Options{})
	client.backend = func(Options) internalincus.Client { return backend }

	preview, err := client.Plan(strings.NewReader(`
kind: network
name: aginctus-mgmt
networkType: bridge
`))
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if backend.created != 0 {
		t.Fatalf("backend created = %d, want 0", backend.created)
	}
	if preview.ResourceCount != 1 {
		t.Fatalf("preview resource count = %d, want 1", preview.ResourceCount)
	}
	if len(preview.Groups) != 1 || preview.Groups[0].Action != "create" {
		t.Fatalf("preview groups = %#v", preview.Groups)
	}
}

func TestExecuteUsesInMemoryConfiguration(t *testing.T) {
	backend := &fakeBackend{}
	client := New(Options{CommandTimeout: time.Minute})
	client.backend = func(Options) internalincus.Client { return backend }

	result, err := client.Execute(strings.NewReader(`
kind: network
name: aginctus-mgmt
networkType: bridge
`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.created != 1 {
		t.Fatalf("backend created = %d, want 1", backend.created)
	}
	if result.Preview.ResourceCount != 1 {
		t.Fatalf("preview resource count = %d, want 1", result.Preview.ResourceCount)
	}
}

func TestRejectsUnknownOperation(t *testing.T) {
	client := New(Options{Operation: Operation("unknown")})
	client.backend = func(Options) internalincus.Client { return &fakeBackend{} }

	_, err := client.Plan(strings.NewReader("kind: network\nname: test\n"))
	if err == nil {
		t.Fatal("Plan() error = nil, want error")
	}
}

func TestNoLaunchOption(t *testing.T) {
	client := New(Options{NoLaunch: true})
	if !client.options.NoLaunch {
		t.Fatal("NoLaunch was not preserved")
	}
}

func TestRequireExistingConfigOption(t *testing.T) {
	required := map[string]string{
		"user.aginctus.managed":  "true",
		"user.aginctus.resource": "management-network",
	}
	client := NewNative(Options{RequireExistingConfig: required})
	if len(client.options.RequireExistingConfig) != 2 {
		t.Fatalf("RequireExistingConfig = %#v", client.options.RequireExistingConfig)
	}
}


func TestRejectUnsupportedChangesOption(t *testing.T) {
	client := NewNative(Options{RejectUnsupportedChanges: true})
	if !client.options.RejectUnsupportedChanges {
		t.Fatal("RejectUnsupportedChanges was not preserved")
	}
}
