package incus

import (
	"errors"
	"net/http"
	"testing"

	incusapi "github.com/lxc/incus/v7/shared/api"

	"github.com/abiosoft/incus-apply/internal/config"
)

type fakeNativeOperation struct {
	err error
}

func (o fakeNativeOperation) Wait() error { return o.err }

type fakeNativeAPI struct {
	server      incusapi.Server
	network     *incusapi.Network
	etag        string
	getErr      error
	created     *incusapi.NetworksPost
	updatedName string
	updated     *incusapi.NetworkPut
	updatedETag string
	deleted     string

	instance            *incusapi.Instance
	instanceETag        string
	instanceErr         error
	createdInstance     *incusapi.InstancesPost
	updatedInstanceName string
	updatedInstance     *incusapi.InstancePut
	updatedInstanceETag string
	deletedInstance     string
	stateName           string
	state               *incusapi.InstanceStatePut
}

func (f *fakeNativeAPI) GetServer() (*incusapi.Server, string, error) {
	return &f.server, "", nil
}

func (f *fakeNativeAPI) GetNetwork(string) (*incusapi.Network, string, error) {
	if f.getErr != nil {
		return nil, "", f.getErr
	}
	if f.network == nil {
		return nil, "", errors.New("network missing")
	}
	return f.network, f.etag, nil
}

func (f *fakeNativeAPI) CreateNetwork(network incusapi.NetworksPost) error {
	f.created = &network
	return nil
}

func (f *fakeNativeAPI) UpdateNetwork(name string, network incusapi.NetworkPut, etag string) error {
	f.updatedName = name
	f.updated = &network
	f.updatedETag = etag
	return nil
}

func (f *fakeNativeAPI) DeleteNetwork(name string) error {
	f.deleted = name
	return nil
}

func (f *fakeNativeAPI) GetInstance(string) (*incusapi.Instance, string, error) {
	if f.instanceErr != nil {
		return nil, "", f.instanceErr
	}
	if f.instance == nil {
		return nil, "", errors.New("instance missing")
	}
	return f.instance, f.instanceETag, nil
}

func (f *fakeNativeAPI) CreateInstance(instance incusapi.InstancesPost) (nativeOperation, error) {
	f.createdInstance = &instance
	return fakeNativeOperation{}, nil
}

func (f *fakeNativeAPI) UpdateInstance(name string, instance incusapi.InstancePut, etag string) (nativeOperation, error) {
	f.updatedInstanceName = name
	f.updatedInstance = &instance
	f.updatedInstanceETag = etag
	return fakeNativeOperation{}, nil
}

func (f *fakeNativeAPI) DeleteInstance(name string) (nativeOperation, error) {
	f.deletedInstance = name
	return fakeNativeOperation{}, nil
}

func (f *fakeNativeAPI) UpdateInstanceState(name string, state incusapi.InstanceStatePut, _ string) (nativeOperation, error) {
	f.stateName = name
	f.state = &state
	if f.instance != nil {
		switch state.Action {
		case "start":
			f.instance.Status = "Running"
		case "stop":
			f.instance.Status = "Stopped"
		}
	}
	return fakeNativeOperation{}, nil
}

func nativeWithFake(api nativeNetworkAPI) *nativeClient {
	return &nativeClient{
		connect: func(string) (nativeNetworkAPI, error) {
			return api, nil
		},
	}
}

func TestNativeCreateNetwork(t *testing.T) {
	api := &fakeNativeAPI{}
	client := nativeWithFake(api)
	res := &config.Resource{
		Base: config.Base{
			Type:        "network",
			Name:        "aginctus-mgmt",
			Config:      map[string]string{"ipv4.address": "10.42.0.1/24"},
			Description: "management",
		},
		NetworkFields: config.NetworkFields{NetworkType: "bridge"},
	}

	result := client.Create(res)
	if result.Error != nil {
		t.Fatalf("Create() error = %v", result.Error)
	}
	if api.created == nil {
		t.Fatal("CreateNetwork was not called")
	}
	if api.created.Name != "aginctus-mgmt" || api.created.Type != "bridge" {
		t.Fatalf("created network = %#v", api.created)
	}
	if api.created.Config["ipv4.address"] != "10.42.0.1/24" {
		t.Fatalf("created config = %#v", api.created.Config)
	}
}

func TestNativeUpdateNetworkUsesETagAndPreservesCurrentConfig(t *testing.T) {
	api := &fakeNativeAPI{
		etag: "etag-1",
		network: &incusapi.Network{
			Name: "aginctus-mgmt",
			Type: "bridge",
			NetworkPut: incusapi.NetworkPut{
				Config: incusapi.ConfigMap{
					"ipv4.address": "10.42.0.1/24",
					"user.keep":    "yes",
				},
				Description: "old",
			},
		},
	}
	client := nativeWithFake(api)
	res := &config.Resource{
		Base: config.Base{
			Type:        "network",
			Name:        "aginctus-mgmt",
			Config:      map[string]string{"ipv4.address": "10.43.0.1/24"},
			Description: "new",
		},
		NetworkFields: config.NetworkFields{NetworkType: "bridge"},
	}

	result := client.Update(res)
	if result.Error != nil {
		t.Fatalf("Update() error = %v", result.Error)
	}
	if api.updatedName != "aginctus-mgmt" || api.updatedETag != "etag-1" {
		t.Fatalf("update target = %q etag = %q", api.updatedName, api.updatedETag)
	}
	if api.updated.Config["ipv4.address"] != "10.43.0.1/24" {
		t.Fatalf("updated config = %#v", api.updated.Config)
	}
	if api.updated.Config["user.keep"] != "yes" {
		t.Fatalf("existing config not preserved: %#v", api.updated.Config)
	}
	if api.updated.Description != "new" {
		t.Fatalf("description = %q", api.updated.Description)
	}
}

func TestNativeDeleteNetwork(t *testing.T) {
	api := &fakeNativeAPI{}
	client := nativeWithFake(api)
	res := &config.Resource{Base: config.Base{Type: "network", Name: "aginctus-mgmt"}}

	result := client.Delete(res)
	if result.Error != nil {
		t.Fatalf("Delete() error = %v", result.Error)
	}
	if api.deleted != "aginctus-mgmt" {
		t.Fatalf("deleted = %q", api.deleted)
	}
}

func TestNativeExistsNetworkNotFound(t *testing.T) {
	api := &fakeNativeAPI{
		getErr: incusapi.StatusErrorf(http.StatusNotFound, "not found"),
	}
	client := nativeWithFake(api)
	res := &config.Resource{Base: config.Base{Type: "network", Name: "missing"}}

	exists, err := client.Exists(res)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Fatal("Exists() = true, want false")
	}
}

func TestNativeRejectsUnsupportedResourceKind(t *testing.T) {
	client := nativeWithFake(&fakeNativeAPI{})
	res := &config.Resource{Base: config.Base{Type: "profile", Name: "default"}}

	result := client.Create(res)
	if result.Error == nil {
		t.Fatal("Create() error = nil, want unsupported error")
	}
}


func TestNativeCreateInstanceFromLocalAlias(t *testing.T) {
	api := &fakeNativeAPI{}
	client := nativeWithFake(api)
	res := &config.Resource{
		Base: config.Base{
			Type: "instance",
			Name: "aginctus-herdr",
			Config: map[string]string{
				"user.aginctus.managed": "true",
			},
			Devices: map[string]map[string]any{
				"root": {
					"type": "disk",
					"path": "/",
					"pool": "default",
				},
				"management": {
					"type": "nic",
					"network": "aginctus-mgmt",
					"name": "eth0",
				},
			},
		},
		InstanceFields: config.InstanceFields{
			Image:    "aginctus-herdr-client",
			Profiles: []string{},
		},
	}

	result := client.Create(res)
	if result.Error != nil {
		t.Fatalf("Create() error = %v", result.Error)
	}
	if api.createdInstance == nil {
		t.Fatal("CreateInstance was not called")
	}
	if api.createdInstance.Name != "aginctus-herdr" {
		t.Fatalf("created name = %q", api.createdInstance.Name)
	}
	if api.createdInstance.Source.Type != "image" || api.createdInstance.Source.Alias != "aginctus-herdr-client" {
		t.Fatalf("source = %#v", api.createdInstance.Source)
	}
	if got := api.createdInstance.Devices["management"]["network"]; got != "aginctus-mgmt" {
		t.Fatalf("management network = %q", got)
	}
}

func TestNativeUpdateInstanceUsesETagAndPreservesForeignConfig(t *testing.T) {
	api := &fakeNativeAPI{
		instanceETag: "instance-etag",
		instance: &incusapi.Instance{
			Name:   "aginctus-herdr",
			Type:   string(incusapi.InstanceTypeContainer),
			Status: "Stopped",
			InstancePut: incusapi.InstancePut{
				Config: incusapi.ConfigMap{
					"user.aginctus.managed": "true",
					"user.keep":             "yes",
				},
				Devices:  incusapi.DevicesMap{},
				Profiles: []string{},
			},
		},
	}
	client := nativeWithFake(api)
	res := &config.Resource{
		Base: config.Base{
			Type: "instance",
			Name: "aginctus-herdr",
			Config: map[string]string{
				"user.aginctus.managed": "true",
				"user.value":            "new",
			},
		},
		InstanceFields: config.InstanceFields{Profiles: []string{}},
	}

	result := client.Update(res)
	if result.Error != nil {
		t.Fatalf("Update() error = %v", result.Error)
	}
	if api.updatedInstanceName != "aginctus-herdr" || api.updatedInstanceETag != "instance-etag" {
		t.Fatalf("update target = %q etag = %q", api.updatedInstanceName, api.updatedInstanceETag)
	}
	if api.updatedInstance.Config["user.keep"] != "yes" || api.updatedInstance.Config["user.value"] != "new" {
		t.Fatalf("updated config = %#v", api.updatedInstance.Config)
	}
}

func TestNativeInstanceStartStopAndRunning(t *testing.T) {
	api := &fakeNativeAPI{
		instance: &incusapi.Instance{
			Name:   "aginctus-herdr",
			Type:   string(incusapi.InstanceTypeContainer),
			Status: "Stopped",
		},
	}
	client := nativeWithFake(api)
	res := &config.Resource{Base: config.Base{Type: "instance", Name: "aginctus-herdr"}}

	if client.Running(res) {
		t.Fatal("Running() = true before start")
	}
	if result := client.Start(res); result.Error != nil {
		t.Fatalf("Start() error = %v", result.Error)
	}
	if !client.Running(res) {
		t.Fatal("Running() = false after start")
	}
	if result := client.Stop(res); result.Error != nil {
		t.Fatalf("Stop() error = %v", result.Error)
	}
	if client.Running(res) {
		t.Fatal("Running() = true after stop")
	}
	if api.state == nil || api.state.Action != "stop" || !api.state.Force {
		t.Fatalf("last state request = %#v", api.state)
	}
}

func TestNativeDeleteInstance(t *testing.T) {
	api := &fakeNativeAPI{}
	client := nativeWithFake(api)
	res := &config.Resource{Base: config.Base{Type: "instance", Name: "aginctus-herdr"}}

	result := client.Delete(res)
	if result.Error != nil {
		t.Fatalf("Delete() error = %v", result.Error)
	}
	if api.deletedInstance != "aginctus-herdr" {
		t.Fatalf("deleted instance = %q", api.deletedInstance)
	}
}

func TestNativeExistsInstanceNotFound(t *testing.T) {
	api := &fakeNativeAPI{instanceErr: incusapi.StatusErrorf(http.StatusNotFound, "not found")}
	client := nativeWithFake(api)
	res := &config.Resource{Base: config.Base{Type: "instance", Name: "missing"}}

	exists, err := client.Exists(res)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Fatal("Exists() = true, want false")
	}
}
