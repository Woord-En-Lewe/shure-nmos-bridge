package infrastructure

import (
	"encoding/json"
	"fmt"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/nca"
)

// Adapter functions to convert between nca types and NCP types

// blockMemberDescriptorToNcBlockMemberDescriptor converts nca.BlockMemberDescriptor to NcBlockMemberDescriptor
func blockMemberDescriptorToNcBlockMemberDescriptor(d nca.BlockMemberDescriptor) NcBlockMemberDescriptor {
	return NcBlockMemberDescriptor{
		Role:        d.Role,
		OID:         d.OID,
		ConstantOID: d.ConstantOID,
		ClassID:     d.ClassID,
		UserLabel:   d.UserLabel,
		Owner:       d.Owner,
		Description: d.Description,
	}
}

// NcBlockMemberDescriptorToBlockMemberDescriptor converts NcBlockMemberDescriptor to nca.BlockMemberDescriptor
func NcBlockMemberDescriptorToBlockMemberDescriptor(d NcBlockMemberDescriptor) nca.BlockMemberDescriptor {
	return nca.BlockMemberDescriptor{
		Role:        d.Role,
		OID:         d.OID,
		ConstantOID: d.ConstantOID,
		ClassID:     d.ClassID,
		UserLabel:   d.UserLabel,
		Owner:       d.Owner,
		Description: d.Description,
	}
}

// NcClassDescriptorToClassDescriptor converts NcClassDescriptor to nca.ClassDescriptor
func NcClassDescriptorToClassDescriptor(d NcClassDescriptor) nca.ClassDescriptor {
	props := make([]nca.PropertyDescriptor, len(d.Properties))
	for i, p := range d.Properties {
		props[i] = nca.PropertyDescriptor{
			ID:           nca.PropertyID(p.ID),
			Name:         p.Name,
			TypeName:     p.TypeName,
			IsReadOnly:   p.IsReadOnly,
			IsNullable:   p.IsNullable,
			IsSequence:   p.IsSequence,
			IsDeprecated: p.IsDeprecated,
		}
	}
	methods := make([]nca.MethodDescriptor, len(d.Methods))
	for i, m := range d.Methods {
		params := make([]nca.ParameterDescriptor, len(m.Parameters))
		for j, p := range m.Parameters {
			params[j] = nca.ParameterDescriptor{
				Name:       p.Name,
				TypeName:   p.TypeName,
				IsNullable: p.IsNullable,
				IsSequence: p.IsSequence,
			}
		}
		methods[i] = nca.MethodDescriptor{
			ID:             nca.MethodID(m.ID),
			Name:           m.Name,
			ResultDatatype: m.ResultDatatype,
			Parameters:     params,
			IsDeprecated:   m.IsDeprecated,
		}
	}
	events := make([]nca.EventDescriptor, len(d.Events))
	for i, e := range d.Events {
		events[i] = nca.EventDescriptor{
			ID:            nca.EventID(e.ID),
			Name:          e.Name,
			EventDatatype: e.EventDatatype,
			IsDeprecated:  e.IsDeprecated,
		}
	}
	var fixedRole *string
	if d.FixedRole != nil {
		fixedRole = new(string)
		*fixedRole = *d.FixedRole
	}
	return nca.ClassDescriptor{
		ClassID:    d.ClassID,
		Name:       d.Name,
		FixedRole:  fixedRole,
		Properties: props,
		Methods:    methods,
		Events:     events,
	}
}

// ClassDescriptorToNcClassDescriptor converts nca.ClassDescriptor to NcClassDescriptor
func ClassDescriptorToNcClassDescriptor(d nca.ClassDescriptor) NcClassDescriptor {
	props := make([]NcPropertyDescriptor, len(d.Properties))
	for i, p := range d.Properties {
		props[i] = NcPropertyDescriptor{
			ID:           NCPPropertyID(p.ID),
			Name:         p.Name,
			TypeName:     p.TypeName,
			IsReadOnly:   p.IsReadOnly,
			IsNullable:   p.IsNullable,
			IsSequence:   p.IsSequence,
			IsDeprecated: p.IsDeprecated,
		}
	}
	methods := make([]NcMethodDescriptor, len(d.Methods))
	for i, m := range d.Methods {
		params := make([]NcParameterDescriptor, len(m.Parameters))
		for j, p := range m.Parameters {
			params[j] = NcParameterDescriptor{
				Name:       p.Name,
				TypeName:   p.TypeName,
				IsNullable: p.IsNullable,
				IsSequence: p.IsSequence,
			}
		}
		methods[i] = NcMethodDescriptor{
			ID:             NCPMethodID(m.ID),
			Name:           m.Name,
			ResultDatatype: m.ResultDatatype,
			Parameters:     params,
			IsDeprecated:   m.IsDeprecated,
		}
	}
	events := make([]NcEventDescriptor, len(d.Events))
	for i, e := range d.Events {
		events[i] = NcEventDescriptor{
			ID:            NCPEventID(e.ID),
			Name:          e.Name,
			EventDatatype: e.EventDatatype,
			IsDeprecated:  e.IsDeprecated,
		}
	}
	var fixedRole *string
	if d.FixedRole != nil {
		fixedRole = new(string)
		*fixedRole = *d.FixedRole
	}
	return NcClassDescriptor{
		ClassID:    d.ClassID,
		Name:       d.Name,
		FixedRole:  fixedRole,
		Properties: props,
		Methods:    methods,
		Events:     events,
	}
}

// NcBlock wraps nca.Block to implement the NcObject interface
// This allows the nca library Block to be used in IS-12 NCP contexts
type NcBlock struct {
	*nca.Block
}

func NewNcBlock(oid int, owner *int, role, label string) *NcBlock {
	return &NcBlock{
		Block: nca.NewBlock(oid, owner, role, label),
	}
}

func (b *NcBlock) GetOID() int {
	return b.Block.GetOID()
}

func (b *NcBlock) GetClassID() []int {
	return b.Block.GetClassID()
}

func (b *NcBlock) GetRole() string {
	return b.Block.GetRole()
}

func (b *NcBlock) GetDescriptor() NcBlockMemberDescriptor {
	return blockMemberDescriptorToNcBlockMemberDescriptor(b.Block.GetDescriptor())
}

func (b *NcBlock) SetNotifyCallback(cb func(oid int, eventID NCPEventID, eventData PropertyChangedEventData)) {
	b.Block.SetNotifyCallback(func(oid int, eventID nca.EventID, eventData nca.PropertyChangedData) {
		cb(oid, NCPEventID(eventID), PropertyChangedEventData(eventData))
	})
}

func (b *NcBlock) GetProperty(id NCPPropertyID) (interface{}, error) {
	return b.Block.GetProperty(nca.PropertyID(id))
}

func (b *NcBlock) SetProperty(id NCPPropertyID, value interface{}) error {
	return b.Block.SetProperty(nca.PropertyID(id), value)
}

func (b *NcBlock) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	return b.Block.InvokeMethod(nca.MethodID(id), args)
}

func (b *NcBlock) AddItem(oid int) {
	b.Block.AddItem(oid)
}

func (b *NcBlock) SetResolver(resolver func(oid int) NcObject) {
	b.Block.SetResolver(func(oid int) nca.Object {
		obj := resolver(oid)
		if obj == nil {
			return nil
		}
		return &ncpObjectAdapter{NcObject: obj}
	})
}

func (b *NcBlock) GetItems() []int {
	return b.Block.Items
}

// NcClassManager wraps nca.ClassManager to implement the NcObject interface
type NcClassManager struct {
	*nca.ClassManager
}

func NewNcClassManager(oid int, owner *int) *NcClassManager {
	return &NcClassManager{
		ClassManager: nca.NewClassManager(oid, owner),
	}
}

func (m *NcClassManager) GetOID() int {
	return m.ClassManager.GetOID()
}

func (m *NcClassManager) GetClassID() []int {
	return m.ClassManager.GetClassID()
}

func (m *NcClassManager) GetRole() string {
	return m.ClassManager.GetRole()
}

func (m *NcClassManager) GetDescriptor() NcBlockMemberDescriptor {
	return blockMemberDescriptorToNcBlockMemberDescriptor(m.ClassManager.GetDescriptor())
}

func (m *NcClassManager) SetNotifyCallback(cb func(oid int, eventID NCPEventID, eventData PropertyChangedEventData)) {
	m.ClassManager.SetNotifyCallback(func(oid int, eventID nca.EventID, eventData nca.PropertyChangedData) {
		cb(oid, NCPEventID(eventID), PropertyChangedEventData(eventData))
	})
}

func (m *NcClassManager) GetProperty(id NCPPropertyID) (interface{}, error) {
	return m.ClassManager.GetProperty(nca.PropertyID(id))
}

func (m *NcClassManager) SetProperty(id NCPPropertyID, value interface{}) error {
	return m.ClassManager.SetProperty(nca.PropertyID(id), value)
}

func (m *NcClassManager) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	return m.ClassManager.InvokeMethod(nca.MethodID(id), args)
}

// NcWorker wraps nca.Worker to implement the NcObject interface
type NcWorker struct {
	*nca.Worker
}

func NewNcWorker(oid int, classID []int, owner *int, role, label string) *NcWorker {
	return &NcWorker{
		Worker: nca.NewWorker(oid, classID, owner, role, label),
	}
}

func (w *NcWorker) GetOID() int {
	return w.Worker.GetOID()
}

func (w *NcWorker) GetClassID() []int {
	return w.Worker.GetClassID()
}

func (w *NcWorker) GetRole() string {
	return w.Worker.GetRole()
}

func (w *NcWorker) GetDescriptor() NcBlockMemberDescriptor {
	return blockMemberDescriptorToNcBlockMemberDescriptor(w.Worker.GetDescriptor())
}

func (w *NcWorker) SetNotifyCallback(cb func(oid int, eventID NCPEventID, eventData PropertyChangedEventData)) {
	w.Worker.SetNotifyCallback(func(oid int, eventID nca.EventID, eventData nca.PropertyChangedData) {
		cb(oid, NCPEventID(eventID), PropertyChangedEventData(eventData))
	})
}

func (w *NcWorker) GetProperty(id NCPPropertyID) (interface{}, error) {
	return w.Worker.GetProperty(nca.PropertyID(id))
}

func (w *NcWorker) SetProperty(id NCPPropertyID, value interface{}) error {
	return w.Worker.SetProperty(nca.PropertyID(id), value)
}

func (w *NcWorker) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	return w.Worker.InvokeMethod(nca.MethodID(id), args)
}

// SetValue sets the worker's value and triggers notification
func (w *NcWorker) SetValue(value interface{}) {
	w.Worker.SetValue(value)
}

// IsEnabled returns the worker's enabled status
func (w *NcWorker) IsEnabled() bool {
	return w.Worker.IsEnabled()
}

// ncpObjectAdapter wraps an NcObject to also implement nca.Object
// This allows using nca.Block.SetResolver with NcObject types
type ncpObjectAdapter struct {
	NcObject
}

func (a *ncpObjectAdapter) GetRole() string {
	return a.NcObject.GetRole()
}

func (a *ncpObjectAdapter) GetDescriptor() nca.BlockMemberDescriptor {
	d := a.NcObject.GetDescriptor()
	return nca.BlockMemberDescriptor{
		Role:        d.Role,
		OID:         d.OID,
		ConstantOID: d.ConstantOID,
		ClassID:     d.ClassID,
		UserLabel:   d.UserLabel,
		Owner:       d.Owner,
		Description: d.Description,
	}
}

// BaseNcObject provides backward compatibility - now just aliases nca.BaseObject
// Deprecated: Use nca.BaseObject directly or wrap with NcBlock/NcWorker/NcClassManager
type BaseNcObject = nca.BaseObject

// Helper to convert classID slice to string key
func classIDToKey(classID []int) string {
	res := ""
	for i, v := range classID {
		if i > 0 {
			res += "."
		}
		res += fmt.Sprint(v)
	}
	return res
}

// NCPObjectToNcObjectAdapter wraps an nca.Object to implement NcObject
type NCPObjectToNcObjectAdapter struct {
	obj nca.Object
}

func NewNCPObjectToNcObjectAdapter(obj nca.Object) *NCPObjectToNcObjectAdapter {
	return &NCPObjectToNcObjectAdapter{obj: obj}
}

func (a *NCPObjectToNcObjectAdapter) GetOID() int {
	return a.obj.GetOID()
}

func (a *NCPObjectToNcObjectAdapter) GetClassID() []int {
	return a.obj.GetClassID()
}

func (a *NCPObjectToNcObjectAdapter) GetRole() string {
	return a.obj.GetRole()
}

func (a *NCPObjectToNcObjectAdapter) GetProperty(id NCPPropertyID) (interface{}, error) {
	return a.obj.GetProperty(nca.PropertyID(id))
}

func (a *NCPObjectToNcObjectAdapter) SetProperty(id NCPPropertyID, value interface{}) error {
	return a.obj.SetProperty(nca.PropertyID(id), value)
}

func (a *NCPObjectToNcObjectAdapter) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	return a.obj.InvokeMethod(nca.MethodID(id), args)
}

func (a *NCPObjectToNcObjectAdapter) GetDescriptor() NcBlockMemberDescriptor {
	d := a.obj.GetDescriptor()
	return NcBlockMemberDescriptor{
		Role:        d.Role,
		OID:         d.OID,
		ConstantOID: d.ConstantOID,
		ClassID:     d.ClassID,
		UserLabel:   d.UserLabel,
		Owner:       d.Owner,
		Description: d.Description,
	}
}

func (a *NCPObjectToNcObjectAdapter) SetNotifyCallback(cb func(oid int, eventID NCPEventID, eventData PropertyChangedEventData)) {
	a.obj.SetNotifyCallback(func(oid int, eventID nca.EventID, eventData nca.PropertyChangedData) {
		cb(oid, NCPEventID(eventID), PropertyChangedEventData(eventData))
	})
}
