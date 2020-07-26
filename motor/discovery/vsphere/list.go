package vsphere

import (
	"context"
	"errors"
	"fmt"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.mondoo.io/mondoo/motor/asset"
	"go.mondoo.io/mondoo/motor/platform"
	vsphere_transport "go.mondoo.io/mondoo/motor/transports/vsphere"
)

func New(client *govmomi.Client) *VSphere {
	return &VSphere{
		Client: client,
	}
}

type VSphere struct {
	Client *govmomi.Client
}

func (v *VSphere) ListEsxiHosts() ([]*asset.Asset, error) {
	dcs, err := v.listDatacenters()
	if err != nil {
		return nil, err
	}

	res := []*asset.Asset{}
	for i := range dcs {
		dc := dcs[i]
		hostList, err := v.listHosts(dc)
		if err != nil {
			return nil, err
		}
		hostsAsAssets, err := hostsToAssetList(hostList)
		if err != nil {
			return nil, err
		}
		res = append(res, hostsAsAssets...)
	}
	return res, nil
}

func hostsToAssetList(hosts []*object.HostSystem) ([]*asset.Asset, error) {
	res := []*asset.Asset{}
	for i := range hosts {
		host := hosts[i]

		// TODO: Determine full platform information eg. esxi
		esxi_version := ""
		// we do not abort in case of error because the simulator does not support esxi interface for the host
		ver, err := vsphere_transport.EsxiVersion(host)
		if err == nil {
			esxi_version = ver.Version
		}

		props, err := hostProperties(host)
		if err != nil {
			return nil, err
		}

		ha := &asset.Asset{
			Name: host.Name(),
			// TODO: sync with detector
			Platform: &platform.Platform{
				Name:    "vmware-esxi",
				Title:   "VMware ESXi",
				Release: esxi_version,
			},
			Runtime: asset.RUNTIME_VSPHERE_HOSTS,
			Kind:    asset.Kind_KIND_BARE_METAL,
			State:   mapHostPowerstateToState(props.Runtime.PowerState),
			Labels: map[string]string{
				"vsphere.vmware.com/reference-type": host.Reference().Type,
				"vsphere.vmware.com/inventorypath":  host.InventoryPath,
			},
		}
		res = append(res, ha)
	}
	return res, nil
}

func hostProperties(host *object.HostSystem) (*mo.HostSystem, error) {
	ctx := context.Background()
	var props mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func mapHostPowerstateToState(hostPowerState types.HostSystemPowerState) asset.State {
	switch hostPowerState {
	case types.HostSystemPowerStatePoweredOn:
		return asset.State_STATE_RUNNING
	case types.HostSystemPowerStatePoweredOff:
		return asset.State_STATE_STOPPED
	case types.HostSystemPowerStateStandBy:
		return asset.State_STATE_PENDING
	case types.HostSystemPowerStateUnknown:
		return asset.State_STATE_UNKNOWN
	default:
		return asset.State_STATE_UNKNOWN
	}
}

func (v *VSphere) ListVirtualMachines() ([]*asset.Asset, error) {
	dcs, err := v.listDatacenters()
	if err != nil {
		return nil, err
	}

	res := []*asset.Asset{}
	for i := range dcs {
		dc := dcs[i]
		vmList, err := v.listVirtualMachines(dc)
		if err != nil {
			return nil, err
		}
		vmsAsAssets, err := vmsToAssetList(vmList)
		if err != nil {
			return nil, err
		}
		res = append(res, vmsAsAssets...)
	}

	return res, nil
}

func vmsToAssetList(vms []*object.VirtualMachine) ([]*asset.Asset, error) {
	res := []*asset.Asset{}
	for i := range vms {
		vm := vms[i]

		props, err := vmProperties(vm)
		if err != nil {
			return nil, err
		}
		ha := &asset.Asset{
			Name:    vm.Name(),
			Runtime: asset.RUNTIME_VSPHERE_VM,
			Kind:    asset.Kind_KIND_VIRTUAL_MACHINE,
			State:   mapVmGuestState(props.Guest.GuestState),
			Labels: map[string]string{
				"vsphere.vmware.com/reference-type": vm.Reference().Type,
				"vsphere.vmware.com/inventorypath":  vm.InventoryPath,
			},
		}
		res = append(res, ha)
	}
	return res, nil
}

func mapVmGuestState(vsphereGuestState string) asset.State {
	switch types.VirtualMachineGuestState(vsphereGuestState) {
	case types.VirtualMachineGuestStateRunning:
		return asset.State_STATE_RUNNING
	case types.VirtualMachineGuestStateShuttingDown:
		return asset.State_STATE_STOPPING
	case types.VirtualMachineGuestStateResetting:
		return asset.State_STATE_REBOOT
	case types.VirtualMachineGuestStateStandby:
		return asset.State_STATE_PENDING
	case types.VirtualMachineGuestStateNotRunning:
		return asset.State_STATE_STOPPED
	case types.VirtualMachineGuestStateUnknown:
		return asset.State_STATE_UNKNOWN
	default:
		return asset.State_STATE_UNKNOWN
	}
}

func vmProperties(vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	ctx := context.Background()
	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func (v *VSphere) listDatacenters() ([]*object.Datacenter, error) {
	finder := find.NewFinder(v.Client.Client, true)
	l, err := finder.ManagedObjectListChildren(context.Background(), "/")
	if err != nil {
		return nil, nil
	}
	var dcs []*object.Datacenter
	for _, item := range l {
		if item.Object.Reference().Type == "Datacenter" {
			dc, err := v.getDatacenter(item.Path)
			if err != nil {
				return nil, err
			}
			dcs = append(dcs, dc)
		}
	}
	return dcs, nil
}

func (v *VSphere) getDatacenter(dc string) (*object.Datacenter, error) {
	finder := find.NewFinder(v.Client.Client, true)
	t := v.Client.ServiceContent.About.ApiType
	switch t {
	case "HostAgent":
		return finder.DefaultDatacenter(context.Background())
	case "VirtualCenter":
		if dc != "" {
			return finder.Datacenter(context.Background(), dc)
		}
		return finder.DefaultDatacenter(context.Background())
	}
	return nil, fmt.Errorf("unsupported ApiType: %s", t)
}

func (c *VSphere) listHosts(dc *object.Datacenter) ([]*object.HostSystem, error) {
	finder := find.NewFinder(c.Client.Client, true)
	finder.SetDatacenter(dc)
	res, err := finder.HostSystemList(context.Background(), "*")
	if err != nil && IsNotFound(err) {
		return []*object.HostSystem{}, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *VSphere) listVirtualMachines(dc *object.Datacenter) ([]*object.VirtualMachine, error) {
	finder := find.NewFinder(c.Client.Client, true)
	finder.SetDatacenter(dc)
	res, err := finder.VirtualMachineList(context.Background(), "*")
	if err != nil && IsNotFound(err) {
		return []*object.VirtualMachine{}, nil
	} else if err != nil {
		return nil, err
	}
	return res, nil
}

// IsNotFound returns a boolean indicating whether the error is a not found error.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var e *find.NotFoundError
	return errors.As(err, &e)
}

func (c *VSphere) Host(path string) (*object.HostSystem, error) {
	finder := find.NewFinder(c.Client.Client, true)
	return finder.HostSystem(context.Background(), path)
}

func (c *VSphere) VirtualMachine(path string) (*object.VirtualMachine, error) {
	finder := find.NewFinder(c.Client.Client, true)
	return finder.VirtualMachine(context.Background(), path)
}