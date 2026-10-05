package incus

import (
	"fmt"
	"net/http"

	incusclient "github.com/lxc/incus/v7/client"
	incusapi "github.com/lxc/incus/v7/shared/api"
	"gopkg.in/yaml.v3"

	"github.com/abiosoft/incus-apply/internal/config"
	"github.com/abiosoft/incus-apply/internal/resource"
)

type nativeNetworkAPI interface {
	GetServer() (*incusapi.Server, string, error)
	GetNetwork(string) (*incusapi.Network, string, error)
	CreateNetwork(incusapi.NetworksPost) error
	UpdateNetwork(string, incusapi.NetworkPut, string) error
	DeleteNetwork(string) error
}

type nativeClient struct {
	remote string
	stop   bool

	connect func(project string) (nativeNetworkAPI, error)
}

func NewNative(remote string, stop bool) Client {
	c := &nativeClient{
		remote: remote,
		stop:   stop,
	}
	var base incusclient.InstanceServer
	c.connect = func(project string) (nativeNetworkAPI, error) {
		if remote != "" {
			return nil, fmt.Errorf("native Incus backend does not yet support named remote %q", remote)
		}
		if base == nil {
			server, err := incusclient.ConnectIncusUnix("", nil)
			if err != nil {
				return nil, fmt.Errorf("connect to local Incus daemon: %w", err)
			}
			base = server
		}
		if project != "" {
			return base.UseProject(project), nil
		}
		return base, nil
	}
	return c
}

func (c *nativeClient) project(res *config.Resource) (nativeNetworkAPI, error) {
	project := ""
	if res != nil {
		project = res.Project
	}
	return c.connect(project)
}

func (c *nativeClient) Ping() error {
	server, err := c.connect("")
	if err != nil {
		return err
	}
	if _, _, err := server.GetServer(); err != nil {
		return fmt.Errorf("cannot connect to Incus daemon: %w", err)
	}
	return nil
}

func (c *nativeClient) Create(res *config.Resource) *Result {
	if resource.Type(res.Type) != resource.TypeNetwork {
		return unsupportedNative(res, "create")
	}

	var enc snapshotCodec = v1SnapshotCodec{}
	prepared, _, err := desiredForApply(res, enc)
	if err != nil {
		return &Result{Error: err}
	}
	server, err := c.project(prepared)
	if err != nil {
		return &Result{Error: err}
	}
	err = server.CreateNetwork(incusapi.NetworksPost{
		Name: prepared.Name,
		Type: prepared.NetworkType,
		NetworkPut: incusapi.NetworkPut{
			Config:      prepared.Config,
			Description: prepared.Description,
		},
	})
	return resultFromError(err)
}

func (c *nativeClient) Update(res *config.Resource) *Result {
	if resource.Type(res.Type) != resource.TypeNetwork {
		return unsupportedNative(res, "update")
	}
	server, err := c.project(res)
	if err != nil {
		return &Result{Error: err}
	}
	current, etag, err := server.GetNetwork(res.Name)
	if err != nil {
		return &Result{Error: err}
	}
	currentYAML, err := yaml.Marshal(current)
	if err != nil {
		return &Result{Error: fmt.Errorf("encode current network config: %w", err)}
	}
	merged, err := mergeConfigs(string(currentYAML), res)
	if err != nil {
		return &Result{Error: fmt.Errorf("merging configs: %w", err)}
	}
	var desired incusapi.Network
	if err := yaml.Unmarshal(merged, &desired); err != nil {
		return &Result{Error: fmt.Errorf("decode merged network config: %w", err)}
	}
	err = server.UpdateNetwork(res.Name, desired.Writable(), etag)
	return resultFromError(err)
}

func (c *nativeClient) Delete(res *config.Resource) *Result {
	if resource.Type(res.Type) != resource.TypeNetwork {
		return unsupportedNative(res, "delete")
	}
	server, err := c.project(res)
	if err != nil {
		return &Result{Error: err}
	}
	return resultFromError(server.DeleteNetwork(res.Name))
}

func (c *nativeClient) Exists(res *config.Resource) (bool, error) {
	if resource.Type(res.Type) != resource.TypeNetwork {
		return false, unsupportedNative(res, "exists").Error
	}
	server, err := c.project(res)
	if err != nil {
		return false, err
	}
	_, _, err = server.GetNetwork(res.Name)
	if err == nil {
		return true, nil
	}
	if incusapi.StatusErrorCheck(err, http.StatusNotFound) {
		return false, nil
	}
	return false, err
}

func (c *nativeClient) CurrentConfig(res *config.Resource) (string, error) {
	if resource.Type(res.Type) != resource.TypeNetwork {
		return "", unsupportedNative(res, "read current config").Error
	}
	server, err := c.project(res)
	if err != nil {
		return "", err
	}
	network, _, err := server.GetNetwork(res.Name)
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(network)
	if err != nil {
		return "", fmt.Errorf("encode current network config: %w", err)
	}
	return string(data), nil
}

func (c *nativeClient) MergedConfig(res *config.Resource) (string, error) {
	current, err := c.CurrentConfig(res)
	if err != nil {
		return "", err
	}
	merged, err := mergeConfigs(current, res)
	if err != nil {
		return "", err
	}
	return string(merged), nil
}

func (c *nativeClient) Start(res *config.Resource) *Result {
	return unsupportedNative(res, "start")
}

func (c *nativeClient) Stop(res *config.Resource) *Result {
	return unsupportedNative(res, "stop")
}

func (c *nativeClient) Running(*config.Resource) bool {
	return false
}

func (c *nativeClient) WaitInstanceAgent(res *config.Resource) *Result {
	return unsupportedNative(res, "wait for instance agent")
}

func (c *nativeClient) WaitCloudInit(res *config.Resource) *Result {
	return unsupportedNative(res, "wait for cloud-init")
}

func unsupportedNative(res *config.Resource, operation string) *Result {
	kind := ""
	if res != nil {
		kind = res.Type
	}
	return &Result{Error: fmt.Errorf("native Incus backend does not support %s for resource kind %q", operation, kind)}
}

func resultFromError(err error) *Result {
	if err != nil {
		return &Result{Error: err, ExitCode: 1}
	}
	return &Result{}
}
