package incus

import (
	"errors"
	"net/http"
	"testing"

	incusapi "github.com/lxc/incus/v7/shared/api"

	"github.com/abiosoft/incus-apply/internal/config"
)

type fakeNativeNetworkAPI struct {
	server      incusapi.Server
	network     *incusapi.Network
	etag        string
	getErr      error
	created     *incusapi.NetworksPost
	updatedName string
	updated     *incusapi.NetworkPut
	updatedETag string
	deleted     string
}

func (f *fakeNativeNetworkAPI) GetServer() (*incusapi.Server, string, error) {
	return &f.server, "", nil
}

func (f *fakeNativeNetworkAPI) GetNetwork(string) (*incusapi.Network, string, error) {
	if f.getErr != nil {
		return nil, "", f.getErr
	}
	if f.network == nil {
		return nil, "", errors.New("network missing")
	}
	return f.network, f.etag, nil
}

func (f *fakeNativeNetworkAPI) CreateNetwork(network incusapi.NetworksPost) error {
	f.created = &network
	return nil
}

func (f *fakeNativeNetworkAPI) UpdateNetwork(name string, network incusapi.NetworkPut, etag string) error {
	f.updatedName = name
	f.updated = &network
	f.updatedETag = etag
	return nil
}

func (f *fakeNativeNetworkAPI) DeleteNetwork(name string) error {
	f.deleted = name
	return nil
}

func nativeWithFake(api nativeNetworkAPI) *nativeClient {
	return &nativeClient{
		connect: func(string) (nativeNetworkAPI, error) {
			return api, nil
		},
	}
}

func TestNativeCreateNetwork(t *testing.T) {
	api := &fakeNativeNetworkAPI{}
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
	api := &fakeNativeNetworkAPI{
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
	api := &fakeNativeNetworkAPI{}
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
	api := &fakeNativeNetworkAPI{
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
	client := nativeWithFake(&fakeNativeNetworkAPI{})
	res := &config.Resource{Base: config.Base{Type: "profile", Name: "default"}}

	result := client.Create(res)
	if result.Error == nil {
		t.Fatal("Create() error = nil, want unsupported error")
	}
}
