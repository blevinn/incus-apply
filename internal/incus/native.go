package incus

import (
	"fmt"
	"net/http"
	"strings"

	incusclient "github.com/lxc/incus/v7/client"
	incusapi "github.com/lxc/incus/v7/shared/api"
	"gopkg.in/yaml.v3"

	"github.com/abiosoft/incus-apply/internal/config"
	"github.com/abiosoft/incus-apply/internal/resource"
)

type nativeAPI interface {
	GetServer() (*incusapi.Server, string, error)
	GetNetwork(string) (*incusapi.Network, string, error)
	CreateNetwork(incusapi.NetworksPost) error
	UpdateNetwork(string, incusapi.NetworkPut, string) error
	DeleteNetwork(string) error

	GetInstance(string) (*incusapi.Instance, string, error)
	CreateInstance(incusapi.InstancesPost) (incusclient.Operation, error)
	UpdateInstance(string, incusapi.InstancePut, string) (incusclient.Operation, error)
	DeleteInstance(string) (incusclient.Operation, error)
	UpdateInstanceState(string, incusapi.InstanceStatePut, string) (incusclient.Operation, error)
}

type nativeClient struct {
	remote string
	stop   bool

	connect func(project string) (nativeAPI, error)
}

func NewNative(remote string, stop bool) Client {
	c := &nativeClient{
		remote: remote,
		stop:   stop,
	}
	var base incusclient.InstanceServer
	c.connect = func(project string) (nativeAPI, error) {
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

func (c *nativeClient) project(res *config.Resource) (nativeAPI, error) {
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
	var enc snapshotCodec = v1SnapshotCodec{}
	prepared, _, err := desiredForApply(res, enc)
	if err != nil {
		return &Result{Error: err}
	}
	server, err := c.project(prepared)
	if err != nil {
		return &Result{Error: err}
	}

	switch resource.Type(prepared.Type) {
	case resource.TypeNetwork:
		err = server.CreateNetwork(incusapi.NetworksPost{
			Name: prepared.Name,
			Type: prepared.NetworkType,
			NetworkPut: incusapi.NetworkPut{
				Config:      prepared.Config,
				Description: prepared.Description,
			},
		})
		return resultFromError(err)
	case resource.TypeInstance:
		request, err := nativeInstanceCreateRequest(prepared)
		if err != nil {
			return &Result{Error: err}
		}
		op, err := server.CreateInstance(request)
		if err != nil {
			return resultFromError(err)
		}
		return resultFromError(op.Wait())
	default:
		return unsupportedNative(res, "create")
	}
}

func (c *nativeClient) Update(res *config.Resource) *Result {
	server, err := c.project(res)
	if err != nil {
		return &Result{Error: err}
	}

	switch resource.Type(res.Type) {
	case resource.TypeNetwork:
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
		return resultFromError(server.UpdateNetwork(res.Name, desired.Writable(), etag))
	case resource.TypeInstance:
		return c.updateInstance(server, res)
	default:
		return unsupportedNative(res, "update")
	}
}

func (c *nativeClient) Delete(res *config.Resource) *Result {
	server, err := c.project(res)
	if err != nil {
		return &Result{Error: err}
	}
	switch resource.Type(res.Type) {
	case resource.TypeNetwork:
		return resultFromError(server.DeleteNetwork(res.Name))
	case resource.TypeInstance:
		op, err := server.DeleteInstance(res.Name)
		if err != nil {
			return resultFromError(err)
		}
		return resultFromError(op.Wait())
	default:
		return unsupportedNative(res, "delete")
	}
}

func (c *nativeClient) Exists(res *config.Resource) (bool, error) {
	server, err := c.project(res)
	if err != nil {
		return false, err
	}
	switch resource.Type(res.Type) {
	case resource.TypeNetwork:
		_, _, err = server.GetNetwork(res.Name)
	case resource.TypeInstance:
		_, _, err = server.GetInstance(res.Name)
	default:
		return false, unsupportedNative(res, "exists").Error
	}
	if err == nil {
		return true, nil
	}
	if incusapi.StatusErrorCheck(err, http.StatusNotFound) {
		return false, nil
	}
	return false, err
}

func (c *nativeClient) CurrentConfig(res *config.Resource) (string, error) {
	server, err := c.project(res)
	if err != nil {
		return "", err
	}
	var value any
	switch resource.Type(res.Type) {
	case resource.TypeNetwork:
		value, _, err = server.GetNetwork(res.Name)
	case resource.TypeInstance:
		value, _, err = server.GetInstance(res.Name)
	default:
		return "", unsupportedNative(res, "read current config").Error
	}
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode current %s config: %w", res.Type, err)
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
	if resource.Type(res.Type) != resource.TypeInstance {
		return unsupportedNative(res, "start")
	}
	server, err := c.project(res)
	if err != nil {
		return &Result{Error: err}
	}
	op, err := server.UpdateInstanceState(res.Name, incusapi.InstanceStatePut{Action: "start", Timeout: -1}, "")
	if err != nil {
		return resultFromError(err)
	}
	return resultFromError(op.Wait())
}

func (c *nativeClient) Stop(res *config.Resource) *Result {
	if resource.Type(res.Type) != resource.TypeInstance {
		return unsupportedNative(res, "stop")
	}
	server, err := c.project(res)
	if err != nil {
		return &Result{Error: err}
	}
	op, err := server.UpdateInstanceState(res.Name, incusapi.InstanceStatePut{Action: "stop", Timeout: -1, Force: true}, "")
	if err != nil {
		return resultFromError(err)
	}
	return resultFromError(op.Wait())
}

func (c *nativeClient) Running(res *config.Resource) bool {
	if resource.Type(res.Type) != resource.TypeInstance {
		return false
	}
	server, err := c.project(res)
	if err != nil {
		return false
	}
	instance, _, err := server.GetInstance(res.Name)
	if err != nil {
		return false
	}
	return strings.EqualFold(instance.Status, "running")
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


func nativeInstanceCreateRequest(res *config.Resource) (incusapi.InstancesPost, error) {
	instanceType := incusapi.InstanceTypeContainer
	if res.VM.Bool() {
		instanceType = incusapi.InstanceTypeVM
	}
	sourceType := "image"
	source := incusapi.InstanceSource{Type: sourceType}
	if res.Empty {
		source.Type = "none"
	} else {
		if res.Image == "" {
			return incusapi.InstancesPost{}, fmt.Errorf("instance %q requires image or empty=true", res.Name)
		}
		source.Alias = res.Image
	}

	devices, err := nativeDevices(res.Devices)
	if err != nil {
		return incusapi.InstancesPost{}, err
	}
	if res.Storage != "" {
		if devices == nil {
			devices = incusapi.DevicesMap{}
		}
		if _, ok := devices["root"]; !ok {
			devices["root"] = map[string]string{
				"type": "disk",
				"path": "/",
				"pool": res.Storage,
			}
		}
	}
	if res.Network != "" {
		if devices == nil {
			devices = incusapi.DevicesMap{}
		}
		if _, ok := devices["eth0"]; !ok {
			devices["eth0"] = map[string]string{
				"type":    "nic",
				"network": res.Network,
				"name":    "eth0",
			}
		}
	}

	return incusapi.InstancesPost{
		Name:   res.Name,
		Type:   instanceType,
		Source: source,
		InstancePut: incusapi.InstancePut{
			Config:      res.Config,
			Devices:     devices,
			Ephemeral:   res.Ephemeral,
			Profiles:    res.Profiles,
			Description: res.Description,
		},
	}, nil
}

func nativeDevices(devices map[string]map[string]any) (incusapi.DevicesMap, error) {
	if devices == nil {
		return nil, nil
	}
	result := make(incusapi.DevicesMap, len(devices))
	for name, device := range devices {
		out := make(map[string]string, len(device))
		for key, value := range device {
			switch typed := value.(type) {
			case string:
				out[key] = typed
			case fmt.Stringer:
				out[key] = typed.String()
			case nil:
				return nil, fmt.Errorf("device %q field %q must not be null", name, key)
			default:
				out[key] = fmt.Sprint(typed)
			}
		}
		result[name] = out
	}
	return result, nil
}

func (c *nativeClient) updateInstance(server nativeAPI, res *config.Resource) *Result {
	current, etag, err := server.GetInstance(res.Name)
	if err != nil {
		return &Result{Error: err}
	}
	currentYAML, err := yaml.Marshal(current)
	if err != nil {
		return &Result{Error: fmt.Errorf("encode current instance config: %w", err)}
	}
	merged, err := mergeConfigs(string(currentYAML), res)
	if err != nil {
		return &Result{Error: fmt.Errorf("merging configs: %w", err)}
	}
	var desired incusapi.Instance
	if err := yaml.Unmarshal(merged, &desired); err != nil {
		return &Result{Error: fmt.Errorf("decode merged instance config: %w", err)}
	}

	wasRunning := strings.EqualFold(current.Status, "running")
	if c.stop && wasRunning {
		if result := c.Stop(res); result.Error != nil {
			return &Result{Error: fmt.Errorf("stopping instance for update: %w", result.Error)}
		}
	}

	op, err := server.UpdateInstance(res.Name, desired.Writable(), etag)
	if err != nil {
		if c.stop && wasRunning {
			_ = c.Start(res)
		}
		return resultFromError(err)
	}
	if err := op.Wait(); err != nil {
		if c.stop && wasRunning {
			_ = c.Start(res)
		}
		return resultFromError(err)
	}

	if c.stop && wasRunning {
		if result := c.Start(res); result.Error != nil {
			return &Result{Error: fmt.Errorf("restarting instance after update: %w", result.Error)}
		}
	}
	return &Result{}
}
