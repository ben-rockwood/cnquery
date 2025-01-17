package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	compute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"

	network "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	azure "go.mondoo.com/cnquery/motor/providers/microsoft/azure"
	"go.mondoo.com/cnquery/resources"
	"go.mondoo.com/cnquery/resources/packs/core"
)

func (a *mqlAzureSubscriptionComputeService) init(args *resources.Args) (*resources.Args, AzureSubscriptionComputeService, error) {
	if len(*args) > 0 {
		return args, nil, nil
	}

	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, nil, err
	}

	(*args)["subscriptionId"] = at.SubscriptionID()

	return args, nil, nil
}

func (a *mqlAzureSubscriptionComputeServiceVm) init(args *resources.Args) (*resources.Args, AzureSubscriptionComputeServiceVm, error) {
	if len(*args) > 1 {
		return args, nil, nil
	}

	if len(*args) == 0 {
		if ids := getAssetIdentifier(a.MqlResource().MotorRuntime); ids != nil {
			(*args)["id"] = ids.id
		}
	}

	if (*args)["id"] == nil {
		return nil, nil, errors.New("id required to fetch azure compute vm instance")
	}

	obj, err := a.MotorRuntime.CreateResource("azure.subscription.computeService")
	if err != nil {
		return nil, nil, err
	}
	computeSvc := obj.(AzureSubscriptionComputeService)

	rawResources, err := computeSvc.Vms()
	if err != nil {
		return nil, nil, err
	}

	id := (*args)["id"].(string)
	for i := range rawResources {
		instance := rawResources[i].(AzureSubscriptionComputeServiceVm)
		instanceId, err := instance.Id()
		if err != nil {
			return nil, nil, errors.New("azure compute instance does not exist")
		}
		if instanceId == id {
			return args, instance, nil
		}
	}
	return nil, nil, errors.New("azure compute instance does not exist")
}

func (a *mqlAzureSubscriptionComputeService) id() (string, error) {
	subId, err := a.SubscriptionId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/subscriptions/%s/computeService", subId), nil
}

func (a *mqlAzureSubscriptionComputeService) GetDisks() ([]interface{}, error) {
	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	token, err := at.GetTokenCredential()
	if err != nil {
		return nil, err
	}

	client, err := compute.NewDisksClient(at.SubscriptionID(), token, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}

	pager := client.NewListPager(&compute.DisksClientListOptions{})
	if err != nil {
		return nil, err
	}

	res := []interface{}{}
	for pager.More() {
		disks, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, disk := range disks.Value {
			mqlAzureDisk, err := diskToMql(a.MotorRuntime, *disk)
			if err != nil {
				return nil, err
			}
			res = append(res, mqlAzureDisk)
		}
	}

	return res, nil
}

func (a *mqlAzureSubscriptionComputeServiceVm) GetPublicIpAddresses() ([]interface{}, error) {
	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, err
	}
	props, err := a.Properties()
	if err != nil {
		return nil, err
	}

	propsDict := (props).(map[string]interface{})
	networkInterface, ok := propsDict["networkProfile"]
	if !ok {
		return nil, errors.New("cannot find network profile on vm, not retrieving ip addresses")
	}
	var networkInterfaces compute.NetworkProfile

	data, err := json.Marshal(networkInterface)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(data), &networkInterfaces)
	if err != nil {
		return nil, err
	}
	res := []interface{}{}

	ctx := context.Background()
	token, err := at.GetTokenCredential()
	if err != nil {
		return nil, err
	}
	nicClient, err := network.NewInterfacesClient(at.SubscriptionID(), token, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}
	ipClient, err := network.NewPublicIPAddressesClient(at.SubscriptionID(), token, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}
	for _, iface := range networkInterfaces.NetworkInterfaces {
		resource, err := azure.ParseResourceID(*iface.ID)
		if err != nil {
			return nil, err
		}

		name, err := resource.Component("networkInterfaces")
		if err != nil {
			return nil, err
		}
		networkInterface, err := nicClient.Get(ctx, resource.ResourceGroup, name, &network.InterfacesClientGetOptions{})
		if err != nil {
			return nil, err
		}

		for _, config := range networkInterface.Interface.Properties.IPConfigurations {
			ip := config.Properties.PublicIPAddress
			if ip != nil {
				publicIPID := *ip.ID
				publicIpResource, err := azure.ParseResourceID(publicIPID)
				if err != nil {
					return nil, errors.New("invalid network information for resource " + publicIPID)
				}

				ipAddrName, err := publicIpResource.Component("publicIPAddresses")
				if err != nil {
					return nil, errors.New("invalid network information for resource " + publicIPID)
				}
				ipAddress, err := ipClient.Get(ctx, publicIpResource.ResourceGroup, ipAddrName, &network.PublicIPAddressesClientGetOptions{})
				if err != nil {
					return nil, err
				}
				mqlIpAddress, err := a.MotorRuntime.CreateResource("azure.subscription.networkService.ipAddress",
					"id", core.ToString(ipAddress.ID),
					"name", core.ToString(ipAddress.Name),
					"location", core.ToString(ipAddress.Location),
					"tags", azureTagsToInterface(ipAddress.Tags),
					"ipAddress", core.ToString(ipAddress.Properties.IPAddress),
					"type", core.ToString(ipAddress.Type),
				)
				if err != nil {
					return nil, err
				}
				res = append(res, mqlIpAddress)
			}
		}
	}

	return res, nil
}

func diskToMql(runtime *resources.Runtime, disk compute.Disk) (resources.ResourceType, error) {
	properties, err := core.JsonToDict(disk.Properties)
	if err != nil {
		return nil, err
	}

	sku, err := core.JsonToDict(disk.SKU)
	if err != nil {
		return nil, err
	}

	managedByExtended := []string{}
	for _, mbe := range disk.ManagedByExtended {
		if mbe != nil {
			managedByExtended = append(managedByExtended, *mbe)
		}
	}
	zones := []string{}
	for _, z := range disk.Zones {
		if z != nil {
			zones = append(zones, *z)
		}
	}
	return runtime.CreateResource("azure.subscription.computeService.disk",
		"id", core.ToString(disk.ID),
		"name", core.ToString(disk.Name),
		"location", core.ToString(disk.Location),
		"tags", azureTagsToInterface(disk.Tags),
		"type", core.ToString(disk.Type),
		"managedBy", core.ToString(disk.ManagedBy),
		"managedByExtended", core.ToStringSlice(&managedByExtended),
		"zones", core.ToStringSlice(&zones),
		"sku", sku,
		"properties", properties,
	)
}

func (a *mqlAzureSubscriptionComputeServiceDisk) id() (string, error) {
	return a.Id()
}

func (a *mqlAzureSubscriptionComputeService) GetVms() ([]interface{}, error) {
	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	token, err := at.GetTokenCredential()
	if err != nil {
		return nil, err
	}

	// list compute instances
	vmClient, err := compute.NewVirtualMachinesClient(at.SubscriptionID(), token, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}
	pager := vmClient.NewListAllPager(&compute.VirtualMachinesClientListAllOptions{})
	res := []interface{}{}
	for pager.More() {
		vms, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, vm := range vms.Value {
			properties, err := core.JsonToDict(vm.Properties)
			if err != nil {
				return nil, err
			}

			mqlAzureVm, err := a.MotorRuntime.CreateResource("azure.subscription.computeService.vm",
				"id", core.ToString(vm.ID),
				"name", core.ToString(vm.Name),
				"location", core.ToString(vm.Location),
				"tags", azureTagsToInterface(vm.Tags),
				"type", core.ToString(vm.Type),
				"properties", properties,
			)
			if err != nil {
				return nil, err
			}
			res = append(res, mqlAzureVm)
		}
	}

	return res, nil
}

func (a *mqlAzureSubscriptionComputeServiceVm) id() (string, error) {
	return a.Id()
}

func (a *mqlAzureSubscriptionComputeServiceVm) GetExtensions() ([]interface{}, error) {
	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, err
	}

	// id is a azure resource id
	id, err := a.Id()
	if err != nil {
		return nil, err
	}

	resourceID, err := azure.ParseResourceID(id)
	if err != nil {
		return nil, err
	}

	vm, err := resourceID.Component("virtualMachines")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	token, err := at.GetTokenCredential()
	if err != nil {
		return nil, err
	}

	client, err := compute.NewVirtualMachineExtensionsClient(resourceID.SubscriptionID, token, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}
	extensions, err := client.List(ctx, resourceID.ResourceGroup, vm, &compute.VirtualMachineExtensionsClientListOptions{})
	if err != nil {
		return nil, err
	}

	res := []interface{}{}

	if extensions.Value == nil {
		return res, nil
	}

	list := extensions.Value

	for i := range list {
		entry := list[i]

		dict, err := core.JsonToDict(entry.Properties)
		if err != nil {
			return nil, err
		}

		res = append(res, dict)
	}

	return res, nil
}

func (a *mqlAzureSubscriptionComputeServiceVm) GetOsDisk() (interface{}, error) {
	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, err
	}

	propertiesDict, err := a.Properties()
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(propertiesDict)
	if err != nil {
		return nil, err
	}

	var properties compute.VirtualMachineProperties
	err = json.Unmarshal(data, &properties)
	if err != nil {
		return nil, err
	}

	if properties.StorageProfile == nil || properties.StorageProfile.OSDisk == nil || properties.StorageProfile.OSDisk.ManagedDisk == nil || properties.StorageProfile.OSDisk.ManagedDisk.ID == nil {
		return nil, errors.New("could not determine os disk from vm storage profile")
	}

	resourceID, err := azure.ParseResourceID(*properties.StorageProfile.OSDisk.ManagedDisk.ID)
	if err != nil {
		return nil, err
	}

	diskName, err := resourceID.Component("disks")
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	token, err := at.GetTokenCredential()
	if err != nil {
		return nil, err
	}

	client, err := compute.NewDisksClient(resourceID.SubscriptionID, token, &arm.ClientOptions{})
	if err != nil {
		return nil, err
	}
	disk, err := client.Get(ctx, resourceID.ResourceGroup, diskName, &compute.DisksClientGetOptions{})
	if err != nil {
		return nil, err
	}

	return diskToMql(a.MotorRuntime, disk.Disk)
}

func (a *mqlAzureSubscriptionComputeServiceVm) GetDataDisks() ([]interface{}, error) {
	at, err := azureTransport(a.MotorRuntime.Motor.Provider)
	if err != nil {
		return nil, err
	}

	propertiesDict, err := a.Properties()
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(propertiesDict)
	if err != nil {
		return nil, err
	}

	var properties compute.VirtualMachineProperties
	err = json.Unmarshal(data, &properties)
	if err != nil {
		return nil, err
	}

	if properties.StorageProfile == nil || properties.StorageProfile.DataDisks == nil {
		return nil, errors.New("could not determine os disk from vm storage profile")
	}

	dataDisks := properties.StorageProfile.DataDisks

	res := []interface{}{}
	for i := range dataDisks {
		dataDisk := dataDisks[i]

		resourceID, err := azure.ParseResourceID(*dataDisk.ManagedDisk.ID)
		if err != nil {
			return nil, err
		}

		diskName, err := resourceID.Component("disks")
		if err != nil {
			return nil, err
		}

		ctx := context.Background()
		token, err := at.GetTokenCredential()
		if err != nil {
			return nil, err
		}

		client, err := compute.NewDisksClient(resourceID.SubscriptionID, token, &arm.ClientOptions{})
		if err != nil {
			return nil, err
		}
		disk, err := client.Get(ctx, resourceID.ResourceGroup, diskName, &compute.DisksClientGetOptions{})
		if err != nil {
			return nil, err
		}

		mqlDisk, err := diskToMql(a.MotorRuntime, disk.Disk)
		if err != nil {
			return nil, err
		}

		res = append(res, mqlDisk)
	}

	return res, nil
}
