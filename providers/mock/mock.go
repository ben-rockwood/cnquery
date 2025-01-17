package mock

import (
	"errors"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"go.mondoo.com/cnquery/llx"
	"go.mondoo.com/cnquery/providers/proto"
	"go.mondoo.com/cnquery/resources"
)

// Unlike other providers, we are currently building this into the core
// of the providers library. In that, it is similar to the core providers.
// Both are optional and both will be removed from being built-in in the
// future, at least from some of the builds.
//
// The reason for this decision is that we want to use it for testing and
// recording/playback in all scenarios. Because core needs to support
// parsers at the moment anyway, we get the benefit of having those
// libraries anyway. So the overhead of this additional loader is very
// small.

type Mock struct {
	Inventory map[string]Resources
	Providers []string
	schema    *resources.Schema
}

type Resources map[string]Resource

type Resource struct {
	Fields map[string]proto.DataRes
}

func NewFromTomlFile(path string) (*Mock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return NewFromToml(data)
}

func loadRawDataRes(raw interface{}) (proto.DataRes, error) {
	switch v := raw.(type) {
	case string:
		return proto.DataRes{Data: llx.StringPrimitive(v)}, nil
	case int64:
		return proto.DataRes{Data: llx.IntPrimitive(v)}, nil
	default:
		return proto.DataRes{}, errors.New("failed to load value")
	}
}

func loadRawFields(resource *Resource, fields map[string]interface{}) error {
	var err error
	for field, raw := range fields {
		resource.Fields[field], err = loadRawDataRes(raw)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadMqlInfo(raw interface{}, m *Mock) error {
	info, ok := raw.(map[string]interface{})
	if !ok {
		return errors.New("mql info is not a map")
	}

	if p, ok := info["providers"]; ok {
		list := p.([]interface{})
		for _, v := range list {
			m.Providers = append(m.Providers, v.(string))
		}
	}

	return nil
}

func New() *Mock {
	return &Mock{
		Inventory: map[string]Resources{},
		Providers: []string{},
	}
}

func NewFromToml(raw []byte) (*Mock, error) {
	var tmp interface{}
	err := toml.Unmarshal(raw, &tmp)
	if err != nil {
		return nil, err
	}

	res := Mock{
		Inventory: map[string]Resources{},
	}
	err = nil

	rawResources, ok := tmp.(map[string]interface{})
	if !ok {
		return nil, errors.New("incorrect structure of recording TOML (outer layer should be resources)")
	}

	if mqlInfo, ok := rawResources["mql"]; ok {
		loadMqlInfo(mqlInfo, &res)
		delete(rawResources, "mql")
	}

	for name, v := range rawResources {
		resources := Resources{}
		res.Inventory[name] = resources

		rawList, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.New("incorrect structure of recording TOML (" + name + " resources should be followed by IDs)")
		}

		for id, vv := range rawList {
			resource := Resource{
				Fields: map[string]proto.DataRes{},
			}

			rawFields, ok := vv.(map[string]interface{})
			if !ok {
				return nil, errors.New("incorrect structure of recording TOML (resource " + name + " (id: " + id + ") should have fields set)")
			}

			if err = loadRawFields(&resource, rawFields); err != nil {
				return nil, err
			}

			resources[id] = resource
		}
	}

	return &res, err
}

func (m *Mock) Unregister(watcherUID string) error {
	// nothing will change, so nothing to watch or unregister
	return nil
}

func (m *Mock) CreateResource(name string, args map[string]*llx.Primitive) (llx.Resource, error) {
	resourceCache, ok := m.Inventory[name]
	if !ok {
		return nil, errors.New("resource '" + name + "' is not in recording")
	}

	// FIXME: we currently have no way of generating the ID that we need to get the right resource,
	// until we have a solid way to (1) connect the right provider and (2) use it to generate the ID on the fly.
	//
	// For now, we are just using a few hardcoded workaround...

	switch name {
	case "command":
		rid, ok := args["command"]
		if !ok {
			return nil, errors.New("cannot find '" + name + "' in recording")
		}

		id := string(rid.Value)
		_, ok = resourceCache[id]
		if !ok {
			return nil, errors.New("cannot find " + name + " '" + id + "' in recording")
		}

		return &llx.MockResource{Name: name, ID: id}, nil
	}

	return nil, errors.New("cannot create resource '" + name + "' from recording yet")
}

func (m *Mock) CreateResourceWithID(name string, id string, args map[string]*llx.Primitive) (llx.Resource, error) {
	resourceCache, ok := m.Inventory[name]
	if !ok {
		return nil, errors.New("resource '" + name + "' is not in recording")
	}

	_, ok = resourceCache[id]
	if !ok {
		return nil, errors.New("cannot find " + name + " '" + id + "' in recording")
	}

	return &llx.MockResource{Name: name, ID: id}, nil
}

func (m *Mock) WatchAndUpdate(resource llx.Resource, field string, watcherUID string, callback func(res interface{}, err error)) error {
	name := resource.MqlName()
	resourceCache, ok := m.Inventory[name]
	if !ok {
		return errors.New("resource '" + name + "' is not in recording")
	}

	id := resource.MqlID()
	x, ok := resourceCache[id]
	if !ok {
		return errors.New("cannot find " + name + " '" + id + "' in recording")
	}

	f, ok := x.Fields[field]
	if !ok {
		return errors.New("cannot find field '" + field + "' in resource " + name + " (id: " + id + ")")
	}

	if f.Error != "" {
		callback(nil, errors.New(f.Error))
	} else {
		callback(f.Data.RawData().Value, nil)
	}

	// nothing will change, so nothing to watch or unregister
	return nil
}

func (m *Mock) Resource(name string) (*resources.ResourceInfo, bool) {
	panic("not sure how to get resource info from mock yet...")
	return nil, false
}

func (m *Mock) Schema() llx.Schema {
	return m.schema
}

func (m *Mock) LoadSchemas(f func(name string) *resources.Schema) error {
	var errs []string
	m.schema = &resources.Schema{
		Resources: map[string]*resources.ResourceInfo{},
	}

	for _, name := range m.Providers {
		if schema := f(name); schema != nil {
			m.schema.Add(schema)
		} else {
			errs = append(errs, name)
		}
	}

	if len(errs) != 0 {
		return errors.New("failed to load schemas for recordings: " + strings.Join(errs, ", "))
	}
	return nil
}

func (m *Mock) Close() {
	// nothing to do yet...
}
