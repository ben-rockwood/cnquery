package vsphere

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/govc/host/esxcli"
	"github.com/vmware/govmomi/object"
)

func NewEsxiClient(c *govmomi.Client, host *object.HostSystem) *Esxi {
	return &Esxi{
		c:    c,
		host: host,
	}
}

type Esxi struct {
	c    *govmomi.Client
	host *object.HostSystem
}

type VSwitch map[string]interface{}

// (Get - EsxCli).network.vswitch.standard.list()
func (esxi *Esxi) VswitchStandard() ([]VSwitch, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"network", "vswitch", "standard", "list"})
	if err != nil {
		return nil, err
	}

	vswitches := make([]VSwitch, len(res.Values))
	for i, val := range res.Values {
		vswitch := VSwitch{}
		for k := range val {
			if len(val[k]) == 1 {
				vswitch[k] = val[k][0]
			} else {
				log.Error().Str("key", k).Msg("EsxiVswitchStandard> unsupported key")
			}
		}
		vswitches[i] = vswitch
	}
	return vswitches, nil
}

// (Get-EsxCli).network.vswitch.dvs.vmware.list()
func (esxi *Esxi) VswitchDvs() ([]VSwitch, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"network", "vswitch", "dvs", "vmware", "list"})
	if err != nil {
		return nil, err
	}

	vswitches := make([]VSwitch, len(res.Values))
	for i, val := range res.Values {
		vswitch := VSwitch{}
		for k := range val {
			if len(val[k]) == 1 {
				vswitch[k] = val[k][0]
			} else {
				log.Error().Str("key", k).Msg("EsxiVswitchDvs> unsupported key")
			}
		}
		vswitches[i] = vswitch
	}
	return vswitches, nil
}

type Adapter map[string]interface{}

// (Get-EsxCli).network.nic.list.Invoke()
func (esxi *Esxi) Adapters() ([]Adapter, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"network", "nic", "list"})
	if err != nil {
		return nil, err
	}

	nics := make([]Adapter, len(res.Values))
	for i, val := range res.Values {
		nic := Adapter{}
		for k := range val {
			if len(val[k]) == 1 {
				nic[k] = val[k][0]
			} else {
				log.Error().Str("key", k).Msg("EsxiAdapters> unsupported key")
			}
		}
		nics[i] = nic
	}
	return nics, nil
}

type VmKernelNic struct {
	Properties map[string]interface{}
	Ipv4       map[string]interface{}
	Ipv6       map[string]interface{}
}

// (Get-EsxCli).network.ip.interface.list()
func (esxi *Esxi) Vmknics() ([]VmKernelNic, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"network", "ip", "interface", "list"})
	if err != nil {
		return nil, err
	}

	vmknics := make([]VmKernelNic, len(res.Values))
	for i, val := range res.Values {
		nic := VmKernelNic{
			Properties: map[string]interface{}{},
		}
		for k := range val {
			if len(val[k]) == 1 {
				nic.Properties[k] = val[k][0]
			} else {
				log.Error().Str("key", k).Msg("EsxiVmknics> unsupported key")
			}
		}

		name := val["Name"][0]
		netstack := val["NetstackInstance"][0]

		// gather ipv4 information
		ipv4Params, err := esxi.VmknixIp(name, netstack, "ipv4")
		if err != nil {
			return nil, err
		}
		nic.Ipv4 = ipv4Params

		// gather ipv6 information
		ipv6Params, err := esxi.VmknixIp(name, netstack, "ipv6")
		if err != nil {
			return nil, err
		}
		nic.Ipv4 = ipv6Params

		vmknics[i] = nic
	}
	return vmknics, nil
}

// (Get-EsxCli).network.ip.interface.ipv4.get('vmk0', 'defaultTcpipStack')
func (esxi *Esxi) VmknixIp(interfacename string, netstack string, ipprotocol string) (map[string]interface{}, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"network", "ip", "interface", ipprotocol, "get"})
	if err != nil {
		return nil, err
	}

	properties := map[string]interface{}{}
	for _, val := range res.Values {
		for k := range val {
			if len(val[k]) == 1 {
				properties[k] = val[k][0]
			} else {
				log.Error().Str("key", k).Msg("EsxiVmknixIp> unsupported key")
			}
		}
	}
	return properties, nil
}

type EsxiVib struct {
	ID              string
	Name            string
	AcceptanceLevel string
	CreationDate    string
	InstallDate     string
	Status          string
	Vendor          string
	Version         string
}

// ($ESXCli).software.vib.list()
// AcceptanceLevel : VMwareCertified
// CreationDate    : 2018-04-03
// ID              : VMware_bootbank_vmware-esx-esxcli-nvme-plugin_1.2.0.32-0.0.8169922
// InstallDate     : 2020-07-16
// Name            : vmware-esx-esxcli-nvme-plugin
// Status          :
// Vendor          : VMware
// Version         : 1.2.0.32-0.0.8169922
func (esxi *Esxi) Vibs() ([]EsxiVib, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"software", "vib", "list"})
	if err != nil {
		return nil, err
	}

	vibs := []EsxiVib{}
	for _, val := range res.Values {
		vib := EsxiVib{}
		for k := range val {
			if len(val[k]) == 1 {
				value := val[k][0]
				switch k {
				case "AcceptanceLevel":
					vib.AcceptanceLevel = value
				case "CreationDate":
					vib.CreationDate = value
				case "ID":
					vib.ID = value
				case "InstallDate":
					vib.InstallDate = value
				case "Name":
					vib.Name = value
				case "Status":
					vib.Status = value
				case "Vendor":
					vib.Vendor = value
				case "Version":
					vib.Version = value
				}
			} else {
				log.Error().Str("key", k).Msg("Vibs> unsupported key")
			}
		}
		vibs = append(vibs, vib)
	}
	return vibs, nil
}

// ($ESXCli).software.acceptance.get()
func (esxi *Esxi) SoftwareAcceptance() (string, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return "", err
	}

	res, err := e.Run([]string{"software", "acceptance", "get"})
	if err != nil {
		return "", err
	}

	if len(res.Values) == 0 {
		if res.String != "" {
			return res.String, nil
		}
	}

	return "", errors.New("unknown software acceptance level")
}

type EsxiKernelModule struct {
	Module               string
	ModuleFile           string
	ProvidedNamespaces   string
	RequiredNamespaces   string
	BuildType            string
	ContainingVIB        string
	FileVersion          string
	License              string
	Version              string
	SignatureDigest      string
	SignatureFingerPrint string
	SignatureIssuer      string
	SignedStatus         string
	VIBAcceptanceLevel   string
	Enabled              bool
	Loaded               bool
}

// ($ESXCli).system.module.list()
// IsEnabled IsLoaded Name
// --------- -------- ----
// true      true     vmkernel
// true      true     chardevs
// true      true     user
// true      true     procfs
func (esxi *Esxi) KernelModules() ([]*EsxiKernelModule, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	res, err := e.Run([]string{"system", "module", "list"})
	if err != nil {
		return nil, err
	}

	kernelmodules := []*EsxiKernelModule{}
	for _, val := range res.Values {
		var modulename string
		loaded := false
		enabled := false

		for k := range val {
			if len(val[k]) == 1 {
				value := val[k][0]

				switch k {
				case "IsEnabled":
					if value == "true" {
						enabled = true
					}
				case "IsLoaded":
					if value == "true" {
						loaded = true
					}
				case "Name":
					modulename = value
				}
			} else {
				log.Error().Str("key", k).Msg("Vibs> unsupported key")
			}
		}

		// gather module additional details
		// NOTE: not sure why but not all list entries have module details
		// e.g "vmkernel", "user" do not return any results
		module, err := esxi.KernelModuleDetails(modulename)
		if err == nil {
			module.Enabled = enabled
			module.Loaded = loaded
			kernelmodules = append(kernelmodules, module)
		} else {
			module = &EsxiKernelModule{
				Module:  modulename,
				Enabled: enabled,
				Loaded:  loaded,
			}
			kernelmodules = append(kernelmodules, module)
		}
	}
	return kernelmodules, nil
}

// $ESXCli.system.module.get("swapobj")
//
// BuildType            :
// ContainingVIB        : esx-base
// FileVersion          :
// License              : VMware
// Module               : swapobj
// ModuleFile           : /usr/lib/vmware/vmkmod/swapobj
// ProvidedNamespaces   : com.vmware.swapobj@0
// RequiredNamespaces   : {com.vmware.vmkapi@v2_5_0_0, com.vmware.vmkapi.incompat@v2_5_0_0, com.vmware.vmklinkmpi@0, vmkernel@nover}
// SignatureDigest      : 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000
// SignatureFingerPrint : 0000 0000 0000 0000 0000 0000 0000 0000
// SignatureIssuer      :
// SignedStatus         : Unsigned
// VIBAcceptanceLevel   : certified
// Version              :
func (esxi *Esxi) KernelModuleDetails(modulename string) (*EsxiKernelModule, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	// NOTE: do not use the powershell syntax, stick with the plain esxcli syntax
	// esxcli <conn_options> system module get --module=module_name
	res, err := e.Run([]string{"system", "module", "get", "--module", modulename})
	if err != nil {
		return nil, err
	}

	if len(res.Values) == 0 {
		return nil, errors.New("could not find esxi kernel module " + modulename)
	}

	if len(res.Values) > 1 {
		return nil, errors.New("ambiguous esxi kernel module name" + modulename)
	}

	module := EsxiKernelModule{}
	val := res.Values[0]
	for k := range val {
		if len(val[k]) == 1 {
			value := val[k][0]

			switch k {
			case "BuildType":
				module.BuildType = value
			case "ContainingVIB":
				module.ContainingVIB = value
			case "FileVersion":
				module.ContainingVIB = value
			case "License":
				module.License = value
			case "Module":
				module.Module = value
			case "ModuleFile":
				module.ModuleFile = value
			case "ProvidedNamespaces":
				module.ProvidedNamespaces = value
			case "SignatureDigest":
				module.SignatureDigest = value
			case "SignatureFingerPrint":
				module.SignatureFingerPrint = value
			case "SignatureIssuer":
				module.SignatureIssuer = value
			case "SignedStatus":
				module.SignedStatus = value
			case "VIBAcceptanceLevel":
				module.VIBAcceptanceLevel = value
			case "Version":
				module.Version = value
			}
		} else {
			log.Error().Str("key", k).Msg("kernelmodule> unsupported key")
		}
	}
	return &module, nil
}

type EsxiAdvancedSetting struct {
	Key         string
	Path        string
	Description string
	Default     string
	Value       string
}

func (s EsxiAdvancedSetting) Overridden() bool {
	return s.Default != s.Value
}

// $ESXCli.system.settings.advanced.list()
// DefaultIntValue    : 1
// DefaultStringValue :
// Description        : Enable hardware accelerated VMFS data movement (requires compliant hardware)
// IntValue           : 1
// MaxValue           : 1
// MinValue           : 0
// Path               : /DataMover/HardwareAcceleratedMove
// StringValue        :
// Type               : integer
// ValidCharacters    :
//
// supported types are `integer` and `string`, both are converted to string
func (esxi *Esxi) AdvancedSettings() ([]EsxiAdvancedSetting, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	// fetch system settings
	res, err := e.Run([]string{"system", "settings", "advanced", "list"})
	if err != nil {
		return nil, err
	}

	settings := []EsxiAdvancedSetting{}
	for _, val := range res.Values {
		setting := EsxiAdvancedSetting{}

		for k := range val {
			if len(val[k]) == 1 {
				value := val[k][0]
				switch k {
				case "Path":
					setting.Path = value
					setting.Key = strings.ReplaceAll(strings.TrimPrefix(value, "/"), "/", ".")
				case "Description":
					setting.Description = value
				case "DefaultIntValue":
					setting.Default = value
				case "DefaultStringValue":
					setting.Default = value
				case "StringValue":
					setting.Value = value
				case "IntValue":
					setting.Value = value
				}
			} else {
				log.Error().Str("key", k).Msg("Vibs> unsupported key")
			}
		}
		settings = append(settings, setting)
	}

	// fetch kernel settings
	// $ESXCli.system.settings.kernel.list()
	res, err = e.Run([]string{"system", "settings", "kernel", "list"})
	if err != nil {
		return nil, err
	}

	for _, val := range res.Values {
		setting := EsxiAdvancedSetting{}

		for k := range val {
			if len(val[k]) == 1 {
				value := val[k][0]
				switch k {
				case "Name":
					setting.Path = value
					setting.Key = "VMkernel.Boot." + value
				case "Description":
					setting.Description = value
				case "Default":
					setting.Default = value
				case "Configured":
					setting.Value = value
				}
			} else {
				log.Error().Str("key", k).Msg("Vibs> unsupported key")
			}
		}
		settings = append(settings, setting)
	}

	return settings, nil
}

type EsxiSystemVersion struct {
	Build   string
	Patch   string
	Product string
	Update  string
	Version string
}

// $ESXCli.system.version.get()
// Build   : Releasebuild-8169922
// Patch   : 0
// Product : VMware ESXi
// Update  : 0
// Version : 6.7.0
func (esxi *Esxi) SystemVersion() (*EsxiSystemVersion, error) {
	e, err := esxcli.NewExecutor(esxi.c.Client, esxi.host)
	if err != nil {
		return nil, err
	}

	// NOTE: do not use the powershell syntax, stick with the plain esxcli syntax
	// esxcli <conn_options> system module get --module=module_name
	res, err := e.Run([]string{"system", "version", "get"})
	if err != nil {
		return nil, err
	}

	if len(res.Values) == 0 {
		return nil, errors.New("could not detect esxi system version ")
	}

	if len(res.Values) > 1 {
		return nil, errors.New("ambiguous esxi system version")
	}

	version := EsxiSystemVersion{}
	val := res.Values[0]
	for k := range val {
		if len(val[k]) == 1 {
			value := val[k][0]

			switch k {
			case "Build":
				version.Build = value
			case "Patch":
				version.Patch = value
			case "Product":
				version.Product = value
			case "Update":
				version.Update = value
			case "Version":
				version.Version = value
			}
		} else {
			log.Error().Str("key", k).Msg("system version> unsupported key")
		}
	}
	return &version, nil
}