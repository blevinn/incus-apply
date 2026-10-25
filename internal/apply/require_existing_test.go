package apply

import (
	"context"
	"testing"

	"github.com/abiosoft/incus-apply/internal/config"
	internalincus "github.com/abiosoft/incus-apply/internal/incus"
)

type mutationSpy struct {
	creates, deletes, updates, starts int
}

func (s *mutationSpy) Ping() error { return nil }
func (s *mutationSpy) Create(*config.Resource) *internalincus.Result {
	s.creates++
	return &internalincus.Result{}
}
func (s *mutationSpy) Update(*config.Resource) *internalincus.Result {
	s.updates++
	return &internalincus.Result{}
}
func (s *mutationSpy) Delete(*config.Resource) *internalincus.Result {
	s.deletes++
	return &internalincus.Result{}
}
func (s *mutationSpy) Exists(*config.Resource) (bool, error) { return true, nil }
func (s *mutationSpy) CurrentConfig(*config.Resource) (string, error) { return "", nil }
func (s *mutationSpy) MergedConfig(*config.Resource) (string, error) { return "", nil }
func (s *mutationSpy) Start(*config.Resource) *internalincus.Result {
	s.starts++
	return &internalincus.Result{}
}
func (s *mutationSpy) Stop(*config.Resource) *internalincus.Result { return &internalincus.Result{} }
func (s *mutationSpy) Running(*config.Resource) bool { return true }
func (s *mutationSpy) WaitInstanceAgent(*config.Resource) *internalincus.Result {
	return &internalincus.Result{}
}
func (s *mutationSpy) WaitCloudInit(*config.Resource) *internalincus.Result {
	return &internalincus.Result{}
}

var _ internalincus.Client = (*mutationSpy)(nil)

func TestRequireExistingRunnerRefusesPlannedCreateAndReplace(t *testing.T) {
	for _, action := range []upsertAction{upsertCreate, upsertReplace} {
		t.Run(map[upsertAction]string{upsertCreate: "create", upsertReplace: "replace"}[action], func(t *testing.T) {
			spy := &mutationSpy{}
			opts := &Options{RequireExisting: true, Quiet: true, FailFast: true}
			r := &runner{ctx: context.Background(), opts: opts, client: spy, printer: upsertPrinter{}}
			res := &config.Resource{Base: config.Base{Type: "instance", Name: "aginctus-dev"}}
			if err := r.upsert(upsertPlan{res: res, action: action}); err == nil {
				t.Fatal("existing-only dispatch accepted creation or replacement")
			}
			if spy.creates != 0 || spy.deletes != 0 || spy.updates != 0 || spy.starts != 0 {
				t.Fatalf("existing-only dispatch mutated resource: %+v", spy)
			}
		})
	}
}

func TestRequireExistingRunnerStillPermitsUpdateAndStart(t *testing.T) {
	for _, tc := range []struct {
		action upsertAction
		wantUpdate, wantStart int
	}{
		{upsertUpdate, 1, 0},
		{upsertStart, 0, 1},
	} {
		spy := &mutationSpy{}
		opts := &Options{RequireExisting: true, Quiet: true}
		r := &runner{ctx: context.Background(), opts: opts, client: spy, printer: upsertPrinter{}}
		res := &config.Resource{Base: config.Base{Type: "instance", Name: "aginctus-dev"}}
		if err := r.upsert(upsertPlan{res: res, action: tc.action}); err != nil {
			t.Fatalf("existing-only transition unexpectedly refused: %v", err)
		}
		if spy.creates != 0 || spy.deletes != 0 ||
			spy.updates != tc.wantUpdate || spy.starts != tc.wantStart {
			t.Fatalf("wrong mutation dispatch: %+v", spy)
		}
	}
}
