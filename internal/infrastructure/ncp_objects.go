package infrastructure

import (
	"encoding/json"
	"fmt"
)

// BaseNcObject provides a default implementation of NcObject
type BaseNcObject struct {
	OID         int
	ClassID     []int
	ConstantOID bool
	Owner       *int
	Role        string
	UserLabel   string
}

func (o *BaseNcObject) GetOID() int {
	return o.OID
}

func (o *BaseNcObject) GetClassID() []int {
	return o.ClassID
}

func (o *BaseNcObject) GetProperty(id NCPPropertyID) (interface{}, error) {
	switch id {
	case NCPropertyClassID:
		return o.ClassID, nil
	case NCPropertyOID:
		return o.OID, nil
	case NCPropertyConstantOID:
		return o.ConstantOID, nil
	case NCPropertyOwner:
		return o.Owner, nil
	case NCPropertyRole:
		return o.Role, nil
	case NCPropertyUserLabel:
		return o.UserLabel, nil
	default:
		return nil, fmt.Errorf("property %d:%d not found", id.Level, id.Index)
	}
}

func (o *BaseNcObject) SetProperty(id NCPPropertyID, value interface{}) error {
	switch id {
	case NCPropertyUserLabel:
		if val, ok := value.(string); ok {
			o.UserLabel = val
			return nil
		}
		return fmt.Errorf("invalid type for UserLabel")
	default:
		return fmt.Errorf("property %d:%d not found or read-only", id.Level, id.Index)
	}
}

func (o *BaseNcObject) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	return nil, fmt.Errorf("method %d:%d not implemented", id.Level, id.Index)
}

func (o *BaseNcObject) GetDescriptor() NcBlockMemberDescriptor {
	return NcBlockMemberDescriptor{
		Role:        o.Role,
		OID:         o.OID,
		ConstantOID: o.ConstantOID,
		ClassID:     o.ClassID,
		UserLabel:   o.UserLabel,
		Owner:       o.Owner,
	}
}

// NcBlock represents an NMOS Control Block
type NcBlock struct {
	BaseNcObject
	Items    []int // OIDs of items in this block
	Resolver func(oid int) NcObject
}

func NewNcBlock(oid int, owner *int, role, label string) *NcBlock {
	return &NcBlock{
		BaseNcObject: BaseNcObject{
			OID:         oid,
			ClassID:     []int{1, 1}, // NcBlock
			ConstantOID: true,
			Owner:       owner,
			Role:        role,
			UserLabel:   label,
		},
		Items: []int{},
	}
}

func (b *NcBlock) GetProperty(id NCPPropertyID) (interface{}, error) {
	if id.Level == 2 {
		switch id.Index {
		case 1: // enabled
			return true, nil
		case 2: // members
			descriptors := make([]NcBlockMemberDescriptor, 0, len(b.Items))
			for _, itemOid := range b.Items {
				if b.Resolver != nil {
					if obj := b.Resolver(itemOid); obj != nil {
						descriptors = append(descriptors, obj.GetDescriptor())
					}
				}
			}
			return descriptors, nil
		}
	}
	return b.BaseNcObject.GetProperty(id)
}

func (b *NcBlock) AddItem(oid int) {
	for _, item := range b.Items {
		if item == oid {
			return
		}
	}
	b.Items = append(b.Items, oid)
}

func (b *NcBlock) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	// Method 1,1: GetItems
	if id.Level == 1 && id.Index == 1 {
		return b.Items, nil
	}
	return b.BaseNcObject.InvokeMethod(id, args)
}

// NcClassManager handles class discovery (OID 3)
type NcClassManager struct {
	BaseNcObject
}

func NewNcClassManager(oid int, owner *int) *NcClassManager {
	return &NcClassManager{
		BaseNcObject: BaseNcObject{
			OID:         oid,
			ClassID:     []int{1, 3, 1}, // NcClassManager
			ConstantOID: true,
			Owner:       owner,
			Role:        "ClassManager",
			UserLabel:   "Class Manager",
		},
	}
}

func (m *NcClassManager) GetProperty(id NCPPropertyID) (interface{}, error) {
	if id.Level == 3 {
		switch id.Index {
		case 1: // controlClasses
			return []interface{}{}, nil
		case 2: // datatypes
			return []interface{}{}, nil
		}
	}
	return m.BaseNcObject.GetProperty(id)
}

func (m *NcClassManager) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	// Method 3,1: GetControlClass
	if id.Level == 3 && id.Index == 1 {
		// Mock implementation for now
		return map[string]interface{}{
			"status": 200,
			"value": map[string]interface{}{
				"name": "MockClass",
			},
		}, nil
	}
	return m.BaseNcObject.InvokeMethod(id, args)
}

// NcWorker represents an NMOS Control Worker (e.g., for a parameter)
type NcWorker struct {
	BaseNcObject
	Value interface{}
	// Callback to sync back to the actual device
	OnSet func(val interface{}) error
}

func NewNcWorker(oid int, classID []int, owner *int, role, label string) *NcWorker {
	return &NcWorker{
		BaseNcObject: BaseNcObject{
			OID:         oid,
			ClassID:     classID,
			ConstantOID: true,
			Owner:       owner,
			Role:        role,
			UserLabel:   label,
		},
	}
}

func (w *NcWorker) GetProperty(id NCPPropertyID) (interface{}, error) {
	// Level 2, Index 1 is often the primary value in many worker classes
	if id.Level == 2 && id.Index == 1 {
		return w.Value, nil
	}
	return w.BaseNcObject.GetProperty(id)
}

func (w *NcWorker) SetProperty(id NCPPropertyID, value interface{}) error {
	if id.Level == 2 && id.Index == 1 {
		if w.OnSet != nil {
			if err := w.OnSet(value); err != nil {
				return err
			}
		}
		w.Value = value
		return nil
	}
	return w.BaseNcObject.SetProperty(id, value)
}
