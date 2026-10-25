package apply

import (
	"strings"
	"testing"

	internalincus "github.com/abiosoft/incus-apply/internal/incus"
)

const missingNetwork = "kind: network\nname: missing-network\nnetworkType: bridge\n"

func TestRequireExistingRejectsMissingResourceForPlanAndExecute(t *testing.T) {
	for _, execute := range []bool{false, true} {
		backend := &fakeBackend{}
		client := NewNative(Options{RequireExisting: true, FailFast: true})
		client.backend = func(options Options) internalincus.Client {
			if !options.RequireExisting {
				t.Fatal("lost RequireExisting option before native backend")
			}
			return backend
		}
		if execute {
			if _, err := client.Execute(strings.NewReader(missingNetwork)); err == nil {
				t.Fatalf("Execute should refuse absent resource: %v", err)
			}
		} else {
			if _, err := client.Plan(strings.NewReader(missingNetwork)); err == nil {
				t.Fatalf("Plan should refuse absent resource: %v", err)
			}
		}
		if backend.created != 0 {
			t.Fatalf("require-existing executed %d creates", backend.created)
		}
	}
}

func TestRequireExistingRejectsReplaceAndNonUpsertBeforeBackend(t *testing.T) {
	for _, opts := range []Options{
		{RequireExisting: true, Replace: true},
		{RequireExisting: true, Operation: Reset},
		{RequireExisting: true, Operation: Delete},
	} {
		client := NewNative(opts)
		client.backend = func(Options) internalincus.Client {
			t.Fatal("invalid require-existing flags reached backend")
			return nil
		}
		if _, err := client.Plan(strings.NewReader(missingNetwork)); err == nil {
			t.Fatalf("Plan accepted invalid options: %#v", opts)
		}
		if _, err := client.Execute(strings.NewReader(missingNetwork)); err == nil {
			t.Fatalf("Execute accepted invalid options: %#v", opts)
		}
	}
}

func TestDefaultUpsertStillCreatesAbsentResource(t *testing.T) {
	backend := &fakeBackend{}
	client := NewNative(Options{})
	client.backend = func(Options) internalincus.Client { return backend }
	if _, err := client.Execute(strings.NewReader(missingNetwork)); err != nil {
		t.Fatalf("existing default behavior regressed: %v", err)
	}
	if backend.created != 1 {
		t.Fatalf("default upsert created %d missing resources, want 1", backend.created)
	}
}
