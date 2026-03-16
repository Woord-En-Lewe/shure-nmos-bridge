package infrastructure

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// BaseNcObject provides a default implementation of NcObject
type BaseNcObject struct {
	OID         int
	ClassID     []int
	ConstantOID bool
	Owner       *int
	Role        string
	UserLabel   string
	Notify      func(oid int, eventID NCPEventID, data interface{})
}

func (o *BaseNcObject) SetNotifyCallback(cb func(oid int, eventID NCPEventID, data interface{})) {
	o.Notify = cb
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
			if o.Notify != nil {
				o.Notify(o.OID, NCPEventID{1, 1}, PropertyChangedEventData{
					PropertyID: id,
					ChangeType: 0,
					Value:      val,
				})
			}
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
	// Method 1,1: GetItems (Wait, NcBlock method level 2?)
	// According to MS-05-02, Block level 2 methods are:
	// 2m1: GetMemberDescriptors
	// 2m2: FindMembersByPath
	// 2m3: FindMembersByRole
	// 2m4: FindMembersByClassId
	if id.Level == 2 && id.Index == 1 {
		descriptors := make([]NcBlockMemberDescriptor, 0, len(b.Items))
		for _, itemOid := range b.Items {
			if b.Resolver != nil {
				if obj := b.Resolver(itemOid); obj != nil {
					descriptors = append(descriptors, obj.GetDescriptor())
				}
			}
		}
		return map[string]interface{}{"status": 200, "value": descriptors}, nil
	}
	return b.BaseNcObject.InvokeMethod(id, args)
}

// NcClassManager handles class discovery (OID 3)
type NcClassManager struct {
	BaseNcObject
	Classes map[string]NcClassDescriptor
}

func NewNcClassManager(oid int, owner *int) *NcClassManager {
	m := &NcClassManager{
		BaseNcObject: BaseNcObject{
			OID:         oid,
			ClassID:     []int{1, 3, 2}, // NcClassManager (1.3.2)
			ConstantOID: true,
			Owner:       owner,
			Role:        "ClassManager",
			UserLabel:   "Class Manager",
		},
		Classes: make(map[string]NcClassDescriptor),
	}
	m.registerStandardClasses()
	return m
}

func (m *NcClassManager) registerStandardClasses() {
	// NcObject
	m.Classes["1"] = NcClassDescriptor{
		Name:    "NcObject",
		ClassID: []int{1},
		Properties: []NcPropertyDescriptor{
			{Name: "classId", ID: NCPPropertyID{1, 1}, TypeName: "NcClassId", IsReadOnly: true},
			{Name: "oid", ID: NCPPropertyID{1, 2}, TypeName: "NcOid", IsReadOnly: true},
			{Name: "constantOid", ID: NCPPropertyID{1, 3}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "owner", ID: NCPPropertyID{1, 4}, TypeName: "NcOid", IsReadOnly: true, IsNullable: true},
			{Name: "role", ID: NCPPropertyID{1, 5}, TypeName: "NcString", IsReadOnly: true},
			{Name: "userLabel", ID: NCPPropertyID{1, 6}, TypeName: "NcString", IsNullable: true},
		},
		Methods: []NcMethodDescriptor{
			{Name: "Get", ID: NCPMethodID{1, 1}, ResultDatatype: "NcMethodResultPropertyValue"},
			{Name: "Set", ID: NCPMethodID{1, 2}, ResultDatatype: "NcMethodResult"},
		},
	}

	// NcBlock
	m.Classes["1.1"] = NcClassDescriptor{
		Name:    "NcBlock",
		ClassID: []int{1, 1},
		Properties: []NcPropertyDescriptor{
			{Name: "enabled", ID: NCPPropertyID{2, 1}, TypeName: "NcBoolean", IsReadOnly: true},
			{Name: "members", ID: NCPPropertyID{2, 2}, TypeName: "NcBlockMemberDescriptor", IsReadOnly: true, IsSequence: true},
		},
		Methods: []NcMethodDescriptor{
			{Name: "GetMemberDescriptors", ID: NCPMethodID{2, 1}, ResultDatatype: "NcMethodResultBlockMemberDescriptors"},
		},
	}

	// NcWorker
	m.Classes["1.2"] = NcClassDescriptor{
		Name:    "NcWorker",
		ClassID: []int{1, 2},
		Properties: []NcPropertyDescriptor{
			{Name: "enabled", ID: NCPPropertyID{2, 1}, TypeName: "NcBoolean"},
		},
	}

	// NcClassManager
	m.Classes["1.3.2"] = NcClassDescriptor{
		Name:    "NcClassManager",
		ClassID: []int{1, 3, 2},
		Properties: []NcPropertyDescriptor{
			{Name: "controlClasses", ID: NCPPropertyID{3, 1}, TypeName: "NcClassDescriptor", IsReadOnly: true, IsSequence: true},
			{Name: "datatypes", ID: NCPPropertyID{3, 2}, TypeName: "NcDatatypeDescriptor", IsReadOnly: true, IsSequence: true},
		},
		Methods: []NcMethodDescriptor{
			{Name: "GetControlClass", ID: NCPMethodID{3, 1}, ResultDatatype: "NcMethodResultClassDescriptor"},
		},
	}
}

func (m *NcClassManager) GetProperty(id NCPPropertyID) (interface{}, error) {
	if id.Level == 3 {
		switch id.Index {
		case 1: // controlClasses
			classes := make([]NcClassDescriptor, 0, len(m.Classes))
			for _, c := range m.Classes {
				classes = append(classes, c)
			}
			return classes, nil
		case 2: // datatypes
			return []interface{}{}, nil
		}
	}
	return m.BaseNcObject.GetProperty(id)
}

func (m *NcClassManager) InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error) {
	// Method 3,1: GetControlClass
	if id.Level == 3 && id.Index == 1 {
		var callArgs struct {
			ClassID []int `json:"classId"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return map[string]interface{}{"status": 400}, nil
		}

		classKey := ""
		for i, part := range callArgs.ClassID {
			if i > 0 {
				classKey += "."
			}
			classKey += fmt.Sprint(part)
		}

		if class, ok := m.Classes[classKey]; ok {
			return map[string]interface{}{"status": 200, "value": class}, nil
		}

		return map[string]interface{}{"status": 404}, nil
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
		if w.Notify != nil {
			w.Notify(w.OID, NCPEventID{1, 1}, PropertyChangedEventData{
				PropertyID: id,
				ChangeType: 0,
				Value:      value,
			})
		}
		return nil
	}
	return w.BaseNcObject.SetProperty(id, value)
}

// Helper to convert slices to string keys
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

func (m *NcClassManager) GetControlClass(classID []int) (NcClassDescriptor, bool) {
	key := classIDToKey(classID)
	c, ok := m.Classes[key]
	return c, ok
}

// Equal check for NCPPropertyID
func (id NCPPropertyID) Equal(other NCPPropertyID) bool {
	return id.Level == other.Level && id.Index == other.Index
}

func compareClassIDs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (m *NcClassManager) IsDerivedFrom(classID, parentID []int) bool {
	if len(classID) < len(parentID) {
		return false
	}
	return reflect.DeepEqual(classID[:len(parentID)], parentID)
}
