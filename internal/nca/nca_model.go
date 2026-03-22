package nca

import (
	"encoding/json"
	"fmt"
	"strings"
)

type EventID struct {
	Level int
	Index int
}

type PropertyID struct {
	Level int
	Index int
}

type MethodID struct {
	Level int
	Index int
}

const (
	EventPropertyChangedLevel = 1
	EventPropertyChangedIndex = 1
)

const (
	MethodGetLevel                = 1
	MethodGetIndex                = 1
	MethodSetLevel                = 1
	MethodSetIndex                = 2
	MethodGetSequenceItemLevel    = 1
	MethodGetSequenceItemIndex    = 3
	MethodSetSequenceItemLevel    = 1
	MethodSetSequenceItemIndex    = 4
	MethodAddSequenceItemLevel    = 1
	MethodAddSequenceItemIndex    = 5
	MethodRemoveSequenceItemLevel = 1
	MethodRemoveSequenceItemIndex = 6
	MethodGetSequenceLengthLevel  = 1
	MethodGetSequenceLengthIndex  = 7
)

const (
	PropertyClassIDLevel            = 1
	PropertyClassIDIndex            = 1
	PropertyOIDLevel                = 1
	PropertyOIDIndex                = 2
	PropertyConstantOIDLevel        = 1
	PropertyConstantOIDIndex        = 3
	PropertyOwnerLevel              = 1
	PropertyOwnerIndex              = 4
	PropertyRoleLevel               = 1
	PropertyRoleIndex               = 5
	PropertyUserLabelLevel          = 1
	PropertyUserLabelIndex          = 6
	PropertyTouchpointsLevel        = 1
	PropertyTouchpointsIndex        = 7
	PropertyRuntimeConstraintsLevel = 1
	PropertyRuntimeConstraintsIndex = 8
)

const (
	BlockPropertyEnabledLevel = 2
	BlockPropertyEnabledIndex = 1
	BlockPropertyMembersLevel = 2
	BlockPropertyMembersIndex = 2
)

const (
	BlockMethodGetMemberDescriptorsLevel = 2
	BlockMethodGetMemberDescriptorsIndex = 1
	BlockMethodFindMembersByPathLevel    = 2
	BlockMethodFindMembersByPathIndex    = 2
	BlockMethodFindMembersByRoleLevel    = 2
	BlockMethodFindMembersByRoleIndex    = 3
	BlockMethodFindMembersByClassIDLevel = 2
	BlockMethodFindMembersByClassIDIndex = 4
)

const (
	WorkerPropertyEnabledLevel = 2
	WorkerPropertyEnabledIndex = 1
)

const (
	DeviceManagerPropertyNcVersionLevel     = 3
	DeviceManagerPropertyNcVersionIndex     = 1
	DeviceManagerPropertyManufacturerLevel  = 3
	DeviceManagerPropertyManufacturerIndex  = 2
	DeviceManagerPropertyProductLevel       = 3
	DeviceManagerPropertyProductIndex       = 3
	DeviceManagerPropertySerialNumberLevel  = 3
	DeviceManagerPropertySerialNumberIndex  = 4
	DeviceManagerPropertyUserInventoryLevel = 3
	DeviceManagerPropertyUserInventoryIndex = 5
	DeviceManagerPropertyDeviceNameLevel    = 3
	DeviceManagerPropertyDeviceNameIndex    = 6
	DeviceManagerPropertyDeviceRoleLevel    = 3
	DeviceManagerPropertyDeviceRoleIndex    = 7
	DeviceManagerPropertyOperationalLevel   = 3
	DeviceManagerPropertyOperationalIndex   = 8
	DeviceManagerPropertyResetCauseLevel    = 3
	DeviceManagerPropertyResetCauseIndex    = 9
	DeviceManagerPropertyMessageLevel       = 3
	DeviceManagerPropertyMessageIndex       = 10
)

const (
	ClassManagerPropertyControlClassesLevel = 3
	ClassManagerPropertyControlClassesIndex = 1
	ClassManagerPropertyDatatypesLevel      = 3
	ClassManagerPropertyDatatypesIndex      = 2
	ClassManagerMethodGetControlClassLevel  = 3
	ClassManagerMethodGetControlClassIndex  = 1
	ClassManagerMethodGetDatatypeLevel      = 3
	ClassManagerMethodGetDatatypeIndex      = 2
)

const (
	ChangeTypeValueChanged      = 0
	ChangeTypeSequenceItemAdded = 1
	ChangeTypeSequenceModified  = 2
	ChangeTypeSequenceRemoved   = 3
)

type NcMethodStatus int

const (
	StatusOk                     NcMethodStatus = 200
	StatusPropertyDeprecated     NcMethodStatus = 298
	StatusMethodDeprecated       NcMethodStatus = 299
	StatusBadCommandFormat       NcMethodStatus = 400
	StatusUnauthorized           NcMethodStatus = 401
	StatusBadOid                 NcMethodStatus = 404
	StatusReadonly               NcMethodStatus = 405
	StatusInvalidRequest         NcMethodStatus = 406
	StatusConflict               NcMethodStatus = 409
	StatusBufferOverflow         NcMethodStatus = 413
	StatusIndexOutOfBounds       NcMethodStatus = 414
	StatusParameterError         NcMethodStatus = 417
	StatusLocked                 NcMethodStatus = 423
	StatusDeviceError            NcMethodStatus = 500
	StatusMethodNotImplemented   NcMethodStatus = 501
	StatusPropertyNotImplemented NcMethodStatus = 502
	StatusNotReady               NcMethodStatus = 503
	StatusTimeout                NcMethodStatus = 504
)

type PropertyChangedData struct {
	PropertyID        PropertyID  `json:"propertyId"`
	ChangeType        int         `json:"changeType"`
	Value             interface{} `json:"value"`
	SequenceItemIndex *int        `json:"sequenceItemIndex,omitempty"`
}

type PropertyConstraints struct {
	PropertyID PropertyID    `json:"propertyId,omitempty"`
	Type       string        `json:"type,omitempty"`
	Minimum    *float64      `json:"minimum,omitempty"`
	Maximum    *float64      `json:"maximum,omitempty"`
	MinLength  *int          `json:"minLength,omitempty"`
	MaxLength  *int          `json:"maxLength,omitempty"`
	Pattern    string        `json:"pattern,omitempty"`
	EnumValues []interface{} `json:"enumValues,omitempty"`
}

type BlockMemberDescriptor struct {
	Role        string `json:"role"`
	OID         int    `json:"oid"`
	ConstantOID bool   `json:"constantOid"`
	ClassID     []int  `json:"classId"`
	UserLabel   string `json:"userLabel,omitempty"`
	Owner       *int   `json:"owner"`
	Description string `json:"description,omitempty"`
}

type VersionCode struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

type Manufacturer struct {
	Name string `json:"name"`
}

type Product struct {
	Manufacturer string `json:"manufacturer"`
	Name         string `json:"name"`
}

type ClassDescriptor struct {
	ClassID    []int                `json:"classId"`
	Name       string               `json:"name"`
	FixedRole  *string              `json:"fixedRole,omitempty"`
	Properties []PropertyDescriptor `json:"properties"`
	Methods    []MethodDescriptor   `json:"methods"`
	Events     []EventDescriptor    `json:"events"`
}

type PropertyDescriptor struct {
	ID           PropertyID `json:"id"`
	Name         string     `json:"name"`
	TypeName     string     `json:"typeName"`
	IsReadOnly   bool       `json:"isReadOnly"`
	IsNullable   bool       `json:"isNullable"`
	IsSequence   bool       `json:"isSequence"`
	IsDeprecated bool       `json:"isDeprecated"`
}

type MethodDescriptor struct {
	ID             MethodID              `json:"id"`
	Name           string                `json:"name"`
	ResultDatatype string                `json:"resultDatatype"`
	Parameters     []ParameterDescriptor `json:"parameters"`
	IsDeprecated   bool                  `json:"isDeprecated"`
}

type ParameterDescriptor struct {
	Name       string `json:"name"`
	TypeName   string `json:"typeName"`
	IsNullable bool   `json:"isNullable"`
	IsSequence bool   `json:"isSequence"`
}

type EventDescriptor struct {
	ID            EventID `json:"id"`
	Name          string  `json:"name"`
	EventDatatype string  `json:"eventDatatype"`
	IsDeprecated  bool    `json:"isDeprecated"`
}

type DatatypeDescriptor struct {
	Name        string      `json:"name"`
	Type        int         `json:"type"`
	Constraints interface{} `json:"constraints,omitempty"`
}

type Touchpoint struct {
	ContextNamespace string `json:"contextNamespace"`
}

type TouchpointNmos struct {
	Touchpoint
	Resource TouchpointResourceNmos `json:"resource"`
}

type TouchpointResourceNmos struct {
	UUID string `json:"uuid"`
}

type Object interface {
	GetOID() int
	GetClassID() []int
	GetRole() string
	GetProperty(id PropertyID) (interface{}, error)
	SetProperty(id PropertyID, value interface{}) error
	InvokeMethod(id MethodID, args json.RawMessage) (interface{}, error)
	GetDescriptor() BlockMemberDescriptor
	SetNotifyCallback(cb func(oid int, eventID EventID, eventData PropertyChangedData))
}

type BaseObject struct {
	OID                int
	ClassID            []int
	ConstantOID        bool
	Owner              *int
	Role               string
	UserLabel          string
	Touchpoints        []Touchpoint
	RuntimeConstraints []PropertyConstraints
	Notify             func(oid int, eventID EventID, eventData PropertyChangedData)
	Sequence           []interface{}
}

func (o *BaseObject) SetNotifyCallback(cb func(oid int, eventID EventID, eventData PropertyChangedData)) {
	o.Notify = cb
}

func (o *BaseObject) GetOID() int {
	return o.OID
}

func (o *BaseObject) GetClassID() []int {
	return o.ClassID
}

func (o *BaseObject) GetOwner() *int {
	return o.Owner
}

func (o *BaseObject) GetRole() string {
	return o.Role
}

func (o *BaseObject) GetUserLabel() string {
	return o.UserLabel
}

func (o *BaseObject) IsConstantOID() bool {
	return o.ConstantOID
}

func (o *BaseObject) GetTouchpoints() []Touchpoint {
	return o.Touchpoints
}

func (o *BaseObject) GetRuntimeConstraints() []PropertyConstraints {
	return o.RuntimeConstraints
}

func (o *BaseObject) GetProperty(id PropertyID) (interface{}, error) {
	if id.Level == PropertyClassIDLevel && id.Index == PropertyClassIDIndex {
		return o.ClassID, nil
	}
	if id.Level == PropertyOIDLevel && id.Index == PropertyOIDIndex {
		return o.OID, nil
	}
	if id.Level == PropertyConstantOIDLevel && id.Index == PropertyConstantOIDIndex {
		return o.ConstantOID, nil
	}
	if id.Level == PropertyOwnerLevel && id.Index == PropertyOwnerIndex {
		return o.Owner, nil
	}
	if id.Level == PropertyRoleLevel && id.Index == PropertyRoleIndex {
		return o.Role, nil
	}
	if id.Level == PropertyUserLabelLevel && id.Index == PropertyUserLabelIndex {
		return o.UserLabel, nil
	}
	if id.Level == PropertyTouchpointsLevel && id.Index == PropertyTouchpointsIndex {
		return o.Touchpoints, nil
	}
	if id.Level == PropertyRuntimeConstraintsLevel && id.Index == PropertyRuntimeConstraintsIndex {
		return o.RuntimeConstraints, nil
	}
	return nil, fmt.Errorf("property %d:%d not found", id.Level, id.Index)
}

func (o *BaseObject) SetProperty(id PropertyID, value interface{}) error {
	if id.Level == PropertyUserLabelLevel && id.Index == PropertyUserLabelIndex {
		val, ok := value.(string)
		if !ok {
			return fmt.Errorf("invalid type for UserLabel")
		}
		o.UserLabel = val
		o.notifyChange(id, val)
		return nil
	}
	if id.Level == PropertyClassIDLevel && id.Index == PropertyClassIDIndex {
		return fmt.Errorf("readonly property")
	}
	if id.Level == PropertyOIDLevel && id.Index == PropertyOIDIndex {
		return fmt.Errorf("readonly property")
	}
	if id.Level == PropertyConstantOIDLevel && id.Index == PropertyConstantOIDIndex {
		return fmt.Errorf("readonly property")
	}
	if id.Level == PropertyOwnerLevel && id.Index == PropertyOwnerIndex {
		return fmt.Errorf("readonly property")
	}
	if id.Level == PropertyRoleLevel && id.Index == PropertyRoleIndex {
		return fmt.Errorf("readonly property")
	}
	return fmt.Errorf("property %d:%d not found", id.Level, id.Index)
}

func (o *BaseObject) notifyChange(id PropertyID, value interface{}) {
	if o.Notify != nil {
		o.Notify(o.OID, EventID{Level: EventPropertyChangedLevel, Index: EventPropertyChangedIndex}, PropertyChangedData{
			PropertyID: id,
			ChangeType: ChangeTypeValueChanged,
			Value:      value,
		})
	}
}

type methodResult struct {
	Status NcMethodStatus `json:"status"`
	Value  interface{}    `json:"value"` // Always present per MS-05-02, null when error
}

func (o *BaseObject) InvokeMethod(id MethodID, args json.RawMessage) (interface{}, error) {
	if id.Level == MethodGetLevel && id.Index == MethodGetIndex {
		var callArgs struct {
			PropertyID PropertyID `json:"propertyId"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		val, err := o.GetProperty(callArgs.PropertyID)
		if err != nil {
			return methodResult{Status: StatusPropertyNotImplemented, Value: nil}, nil
		}
		return methodResult{Status: StatusOk, Value: val}, nil
	}

	if id.Level == MethodSetLevel && id.Index == MethodSetIndex {
		var callArgs struct {
			PropertyID PropertyID  `json:"propertyId"`
			Value      interface{} `json:"value"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		if err := o.SetProperty(callArgs.PropertyID, callArgs.Value); err != nil {
			return methodResult{Status: StatusDeviceError}, nil
		}
		return methodResult{Status: StatusOk}, nil
	}

	if id.Level == MethodGetSequenceItemLevel && id.Index == MethodGetSequenceItemIndex {
		var callArgs struct {
			PropertyID PropertyID `json:"propertyId"`
			Index      int        `json:"index"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		val, err := o.GetProperty(callArgs.PropertyID)
		if err != nil {
			return methodResult{Status: StatusPropertyNotImplemented, Value: nil}, nil
		}
		seq, ok := val.([]interface{})
		if !ok {
			return methodResult{Status: StatusPropertyNotImplemented, Value: nil}, nil
		}
		if callArgs.Index < 0 || callArgs.Index >= len(seq) {
			return methodResult{Status: StatusIndexOutOfBounds, Value: nil}, nil
		}
		return methodResult{Status: StatusOk, Value: seq[callArgs.Index]}, nil
	}

	if id.Level == MethodSetSequenceItemLevel && id.Index == MethodSetSequenceItemIndex {
		var callArgs struct {
			PropertyID PropertyID  `json:"propertyId"`
			Index      int         `json:"index"`
			Value      interface{} `json:"value"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		propVal, err := o.GetProperty(callArgs.PropertyID)
		if err != nil {
			return methodResult{Status: StatusPropertyNotImplemented}, nil
		}
		seq, ok := propVal.([]interface{})
		if !ok {
			return methodResult{Status: StatusPropertyNotImplemented}, nil
		}
		if callArgs.Index < 0 || callArgs.Index >= len(seq) {
			return methodResult{Status: StatusIndexOutOfBounds}, nil
		}
		seq[callArgs.Index] = callArgs.Value
		idx := callArgs.Index
		o.notifySequenceChange(callArgs.PropertyID, ChangeTypeSequenceModified, callArgs.Value, &idx)
		return methodResult{Status: StatusOk}, nil
	}

	if id.Level == MethodAddSequenceItemLevel && id.Index == MethodAddSequenceItemIndex {
		var callArgs struct {
			PropertyID PropertyID  `json:"propertyId"`
			Value      interface{} `json:"value"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		propVal, err := o.GetProperty(callArgs.PropertyID)
		if err != nil {
			return methodResult{Status: StatusPropertyNotImplemented}, nil
		}
		seq, ok := propVal.([]interface{})
		if !ok {
			return methodResult{Status: StatusPropertyNotImplemented}, nil
		}
		idx := len(seq)
		seq = append(seq, callArgs.Value)
		o.notifySequenceChange(callArgs.PropertyID, ChangeTypeSequenceItemAdded, callArgs.Value, &idx)
		return methodResult{Status: StatusOk, Value: idx}, nil
	}

	if id.Level == MethodRemoveSequenceItemLevel && id.Index == MethodRemoveSequenceItemIndex {
		var callArgs struct {
			PropertyID PropertyID `json:"propertyId"`
			Index      int        `json:"index"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		propVal, err := o.GetProperty(callArgs.PropertyID)
		if err != nil {
			return methodResult{Status: StatusPropertyNotImplemented}, nil
		}
		seq, ok := propVal.([]interface{})
		if !ok {
			return methodResult{Status: StatusPropertyNotImplemented}, nil
		}
		if callArgs.Index < 0 || callArgs.Index >= len(seq) {
			return methodResult{Status: StatusIndexOutOfBounds}, nil
		}
		removed := seq[callArgs.Index]
		seq = append(seq[:callArgs.Index], seq[callArgs.Index+1:]...)
		idx := callArgs.Index
		o.notifySequenceChange(callArgs.PropertyID, ChangeTypeSequenceRemoved, removed, &idx)
		return methodResult{Status: StatusOk}, nil
	}

	if id.Level == MethodGetSequenceLengthLevel && id.Index == MethodGetSequenceLengthIndex {
		var callArgs struct {
			PropertyID PropertyID `json:"propertyId"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		val, err := o.GetProperty(callArgs.PropertyID)
		if err != nil {
			return methodResult{Status: StatusPropertyNotImplemented, Value: nil}, nil
		}
		seq, ok := val.([]interface{})
		if !ok {
			return methodResult{Status: StatusPropertyNotImplemented, Value: 0}, nil
		}
		return methodResult{Status: StatusOk, Value: len(seq)}, nil
	}

	return nil, fmt.Errorf("method %d:%d not found", id.Level, id.Index)
}

func (o *BaseObject) notifySequenceChange(propertyID PropertyID, changeType int, value interface{}, index *int) {
	if o.Notify != nil {
		o.Notify(o.OID, EventID{Level: EventPropertyChangedLevel, Index: EventPropertyChangedIndex}, PropertyChangedData{
			PropertyID:        propertyID,
			ChangeType:        changeType,
			Value:             value,
			SequenceItemIndex: index,
		})
	}
}

func (o *BaseObject) GetDescriptor() BlockMemberDescriptor {
	return BlockMemberDescriptor{
		Role:        o.Role,
		OID:         o.OID,
		ConstantOID: o.ConstantOID,
		ClassID:     o.ClassID,
		UserLabel:   o.UserLabel,
		Owner:       o.Owner,
	}
}

type Block struct {
	BaseObject
	Items    []int
	Resolver func(oid int) Object
}

func NewBlock(oid int, owner *int, role, label string) *Block {
	return &Block{
		BaseObject: BaseObject{
			OID:         oid,
			ClassID:     []int{1, 1},
			ConstantOID: true,
			Owner:       owner,
			Role:        role,
			UserLabel:   label,
		},
		Items: []int{},
	}
}

func (b *Block) GetProperty(id PropertyID) (interface{}, error) {
	if id.Level == BlockPropertyEnabledLevel && id.Index == BlockPropertyEnabledIndex {
		return true, nil
	}
	if id.Level == BlockPropertyMembersLevel && id.Index == BlockPropertyMembersIndex {
		descriptors := make([]BlockMemberDescriptor, 0, len(b.Items))
		for _, itemOid := range b.Items {
			if b.Resolver != nil {
				if obj := b.Resolver(itemOid); obj != nil {
					descriptors = append(descriptors, obj.GetDescriptor())
				}
			}
		}
		return descriptors, nil
	}
	return b.BaseObject.GetProperty(id)
}

func (b *Block) InvokeMethod(id MethodID, args json.RawMessage) (interface{}, error) {
	if id.Level == BlockMethodGetMemberDescriptorsLevel && id.Index == BlockMethodGetMemberDescriptorsIndex {
		var callArgs struct {
			Recurse bool `json:"recurse"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		descriptors := b.getMemberDescriptors(callArgs.Recurse)
		return methodResult{Status: StatusOk, Value: descriptors}, nil
	}

	if id.Level == BlockMethodFindMembersByPathLevel && id.Index == BlockMethodFindMembersByPathIndex {
		var callArgs struct {
			Path []string `json:"path"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		descriptors := b.findMembersByPath(callArgs.Path)
		return methodResult{Status: StatusOk, Value: descriptors}, nil
	}

	if id.Level == BlockMethodFindMembersByRoleLevel && id.Index == BlockMethodFindMembersByRoleIndex {
		var callArgs struct {
			Role          string `json:"role"`
			CaseSensitive bool   `json:"caseSensitive"`
			MatchWhole    bool   `json:"matchWholeString"`
			Recurse       bool   `json:"recurse"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		descriptors := b.findMembersByRole(callArgs.Role, callArgs.CaseSensitive, callArgs.MatchWhole, callArgs.Recurse)
		return methodResult{Status: StatusOk, Value: descriptors}, nil
	}

	if id.Level == BlockMethodFindMembersByClassIDLevel && id.Index == BlockMethodFindMembersByClassIDIndex {
		var callArgs struct {
			ClassID        []int `json:"classId"`
			IncludeDerived bool  `json:"includeDerived"`
			Recurse        bool  `json:"recurse"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}
		descriptors := b.findMembersByClassID(callArgs.ClassID, callArgs.IncludeDerived, callArgs.Recurse)
		return methodResult{Status: StatusOk, Value: descriptors}, nil
	}

	return b.BaseObject.InvokeMethod(id, args)
}

func (b *Block) getMemberDescriptors(recurse bool) []BlockMemberDescriptor {
	descriptors := make([]BlockMemberDescriptor, 0)
	for _, itemOid := range b.Items {
		if b.Resolver != nil {
			if obj := b.Resolver(itemOid); obj != nil {
				descriptors = append(descriptors, obj.GetDescriptor())
				if recurse {
					if block, ok := obj.(*Block); ok {
						descriptors = append(descriptors, block.getMemberDescriptors(true)...)
					}
				}
			}
		}
	}
	return descriptors
}

func (b *Block) findMembersByPath(path []string) []BlockMemberDescriptor {
	if len(path) == 0 {
		return nil
	}
	firstRole := path[0]
	for _, itemOid := range b.Items {
		if b.Resolver != nil {
			if obj := b.Resolver(itemOid); obj != nil {
				if obj.GetRole() == firstRole {
					if len(path) == 1 {
						return []BlockMemberDescriptor{obj.GetDescriptor()}
					}
					if block, ok := obj.(*Block); ok {
						return block.findMembersByPath(path[1:])
					}
					return nil
				}
			}
		}
	}
	return nil
}

func (b *Block) findMembersByRole(role string, caseSensitive, matchWhole, recurse bool) []BlockMemberDescriptor {
	descriptors := make([]BlockMemberDescriptor, 0)
	for _, itemOid := range b.Items {
		if b.Resolver != nil {
			if obj := b.Resolver(itemOid); obj != nil {
				matches := false
				objRole := obj.GetRole()
				if matchWhole {
					if caseSensitive && objRole == role {
						matches = true
					} else if !caseSensitive && strings.EqualFold(objRole, role) {
						matches = true
					}
				} else {
					if caseSensitive && strings.Contains(objRole, role) {
						matches = true
					} else if !caseSensitive && strings.Contains(strings.ToLower(objRole), strings.ToLower(role)) {
						matches = true
					}
				}
				if matches {
					descriptors = append(descriptors, obj.GetDescriptor())
				}
				if recurse {
					if block, ok := obj.(*Block); ok {
						descriptors = append(descriptors, block.findMembersByRole(role, caseSensitive, matchWhole, true)...)
					}
				}
			}
		}
	}
	return descriptors
}

func (b *Block) findMembersByClassID(classID []int, includeDerived, recurse bool) []BlockMemberDescriptor {
	descriptors := make([]BlockMemberDescriptor, 0)
	for _, itemOid := range b.Items {
		if b.Resolver != nil {
			if obj := b.Resolver(itemOid); obj != nil {
				if matchesClassID(obj.GetClassID(), classID, includeDerived) {
					descriptors = append(descriptors, obj.GetDescriptor())
				}
				if recurse {
					if block, ok := obj.(*Block); ok {
						descriptors = append(descriptors, block.findMembersByClassID(classID, includeDerived, true)...)
					}
				}
			}
		}
	}
	return descriptors
}

func matchesClassID(objClassID, searchClassID []int, includeDerived bool) bool {
	if len(objClassID) < len(searchClassID) {
		return false
	}
	for i := range searchClassID {
		if objClassID[i] != searchClassID[i] {
			return false
		}
	}
	if includeDerived {
		return true
	}
	return len(objClassID) == len(searchClassID)
}

func (b *Block) AddItem(oid int) {
	for _, item := range b.Items {
		if item == oid {
			return
		}
	}
	b.Items = append(b.Items, oid)
}

func (b *Block) SetResolver(resolver func(oid int) Object) {
	b.Resolver = resolver
}

type Worker struct {
	BaseObject
	Value interface{}
	OnSet func(val interface{}) error
}

func NewWorker(oid int, classID []int, owner *int, role, label string) *Worker {
	return &Worker{
		BaseObject: BaseObject{
			OID:         oid,
			ClassID:     classID,
			ConstantOID: true,
			Owner:       owner,
			Role:        role,
			UserLabel:   label,
		},
	}
}

func (w *Worker) GetProperty(id PropertyID) (interface{}, error) {
	if id.Level == WorkerPropertyEnabledLevel && id.Index == WorkerPropertyEnabledIndex {
		return true, nil
	}
	if id.Level == 3 && id.Index == 1 {
		return w.Value, nil
	}
	return w.BaseObject.GetProperty(id)
}

func (w *Worker) SetProperty(id PropertyID, value interface{}) error {
	if id.Level == 3 && id.Index == 1 {
		if w.OnSet != nil {
			if err := w.OnSet(value); err != nil {
				return err
			}
		}
		w.Value = value
		w.notifyChange(id, value)
		return nil
	}
	return w.BaseObject.SetProperty(id, value)
}

func (w *Worker) SetValue(value interface{}) {
	w.Value = value
	w.notifyChange(PropertyID{Level: 3, Index: 1}, value)
}

func (w *Worker) IsEnabled() bool {
	return true
}

func (w *Worker) notifyChange(id PropertyID, value interface{}) {
	if w.Notify != nil {
		w.Notify(w.OID, EventID{Level: EventPropertyChangedLevel, Index: EventPropertyChangedIndex}, PropertyChangedData{
			PropertyID: id,
			ChangeType: ChangeTypeValueChanged,
			Value:      value,
		})
	}
}

type Manager struct {
	BaseObject
}

func NewManager(oid int, classID []int, owner *int, role, label string) *Manager {
	return &Manager{
		BaseObject: BaseObject{
			OID:         oid,
			ClassID:     classID,
			ConstantOID: true,
			Owner:       owner,
			Role:        role,
			UserLabel:   label,
		},
	}
}

type DeviceManager struct {
	Manager
	ncVersion         VersionCode
	manufacturer      Manufacturer
	product           Product
	serialNumber      string
	userInventoryCode string
	deviceName        string
	deviceRole        string
	operationalState  int
	resetCause        int
	message           string
}

func NewDeviceManager(oid int, owner *int) *DeviceManager {
	return &DeviceManager{
		Manager: Manager{
			BaseObject: BaseObject{
				OID:         oid,
				ClassID:     []int{1, 3, 1},
				ConstantOID: true,
				Owner:       owner,
				Role:        "DeviceManager",
				UserLabel:   "Device Manager",
			},
		},
		ncVersion:        VersionCode{Major: 1, Minor: 0},
		operationalState: 1,
		resetCause:       1,
	}
}

func (m *DeviceManager) GetProperty(id PropertyID) (interface{}, error) {
	if id.Level == DeviceManagerPropertyNcVersionLevel && id.Index == DeviceManagerPropertyNcVersionIndex {
		return m.ncVersion, nil
	}
	if id.Level == DeviceManagerPropertyManufacturerLevel && id.Index == DeviceManagerPropertyManufacturerIndex {
		return m.manufacturer, nil
	}
	if id.Level == DeviceManagerPropertyProductLevel && id.Index == DeviceManagerPropertyProductIndex {
		return m.product, nil
	}
	if id.Level == DeviceManagerPropertySerialNumberLevel && id.Index == DeviceManagerPropertySerialNumberIndex {
		return m.serialNumber, nil
	}
	if id.Level == DeviceManagerPropertyUserInventoryLevel && id.Index == DeviceManagerPropertyUserInventoryIndex {
		return m.userInventoryCode, nil
	}
	if id.Level == DeviceManagerPropertyDeviceNameLevel && id.Index == DeviceManagerPropertyDeviceNameIndex {
		return m.deviceName, nil
	}
	if id.Level == DeviceManagerPropertyDeviceRoleLevel && id.Index == DeviceManagerPropertyDeviceRoleIndex {
		return m.deviceRole, nil
	}
	if id.Level == DeviceManagerPropertyOperationalLevel && id.Index == DeviceManagerPropertyOperationalIndex {
		return m.operationalState, nil
	}
	if id.Level == DeviceManagerPropertyResetCauseLevel && id.Index == DeviceManagerPropertyResetCauseIndex {
		return m.resetCause, nil
	}
	if id.Level == DeviceManagerPropertyMessageLevel && id.Index == DeviceManagerPropertyMessageIndex {
		return m.message, nil
	}
	return m.BaseObject.GetProperty(id)
}

func (m *DeviceManager) SetProperty(id PropertyID, value interface{}) error {
	if id.Level == DeviceManagerPropertyUserInventoryLevel && id.Index == DeviceManagerPropertyUserInventoryIndex {
		if val, ok := value.(string); ok {
			m.userInventoryCode = val
			m.notifyChange(id, val)
			return nil
		}
		return fmt.Errorf("invalid type for userInventoryCode")
	}
	if id.Level == DeviceManagerPropertyDeviceNameLevel && id.Index == DeviceManagerPropertyDeviceNameIndex {
		if val, ok := value.(string); ok {
			m.deviceName = val
			m.notifyChange(id, val)
			return nil
		}
		return fmt.Errorf("invalid type for deviceName")
	}
	if id.Level == DeviceManagerPropertyDeviceRoleLevel && id.Index == DeviceManagerPropertyDeviceRoleIndex {
		if val, ok := value.(string); ok {
			m.deviceRole = val
			m.notifyChange(id, val)
			return nil
		}
		return fmt.Errorf("invalid type for deviceRole")
	}
	return m.BaseObject.SetProperty(id, value)
}

func (m *DeviceManager) SetDeviceName(name string) {
	m.deviceName = name
	m.notifyChange(PropertyID{Level: DeviceManagerPropertyDeviceNameLevel, Index: DeviceManagerPropertyDeviceNameIndex}, name)
}

func (m *DeviceManager) SetDeviceRole(role string) {
	m.deviceRole = role
	m.notifyChange(PropertyID{Level: DeviceManagerPropertyDeviceRoleLevel, Index: DeviceManagerPropertyDeviceRoleIndex}, role)
}

func (m *DeviceManager) SetMessage(msg string) {
	m.message = msg
	m.notifyChange(PropertyID{Level: DeviceManagerPropertyMessageLevel, Index: DeviceManagerPropertyMessageIndex}, msg)
}

func (m *DeviceManager) notifyChange(id PropertyID, value interface{}) {
	if m.Notify != nil {
		m.Notify(m.OID, EventID{Level: EventPropertyChangedLevel, Index: EventPropertyChangedIndex}, PropertyChangedData{
			PropertyID: id,
			ChangeType: ChangeTypeValueChanged,
			Value:      value,
		})
	}
}

type ClassManager struct {
	Manager
	Classes   map[string]ClassDescriptor
	Datatypes map[string]DatatypeDescriptor
}

func NewClassManager(oid int, owner *int) *ClassManager {
	m := &ClassManager{
		Manager: Manager{
			BaseObject: BaseObject{
				OID:         oid,
				ClassID:     []int{1, 3, 2},
				ConstantOID: true,
				Owner:       owner,
				Role:        "ClassManager",
				UserLabel:   "Class Manager",
			},
		},
		Classes:   make(map[string]ClassDescriptor),
		Datatypes: make(map[string]DatatypeDescriptor),
	}
	m.registerStandardClasses()
	return m
}

func (m *ClassManager) registerStandardClasses() {
	m.Classes["1"] = ClassDescriptor{
		Name:    "NcObject",
		ClassID: []int{1},
		Properties: []PropertyDescriptor{
			{ID: PropertyID{1, 1}, Name: "classId", TypeName: "NcClassId", IsReadOnly: true},
			{ID: PropertyID{1, 2}, Name: "oid", TypeName: "NcOid", IsReadOnly: true},
			{ID: PropertyID{1, 3}, Name: "constantOid", TypeName: "NcBoolean", IsReadOnly: true},
			{ID: PropertyID{1, 4}, Name: "owner", TypeName: "NcOid", IsReadOnly: true, IsNullable: true},
			{ID: PropertyID{1, 5}, Name: "role", TypeName: "NcString", IsReadOnly: true},
			{ID: PropertyID{1, 6}, Name: "userLabel", TypeName: "NcString", IsNullable: true},
			{ID: PropertyID{1, 7}, Name: "touchpoints", TypeName: "NcTouchpoint", IsReadOnly: true, IsSequence: true},
			{ID: PropertyID{1, 8}, Name: "runtimePropertyConstraints", TypeName: "NcPropertyConstraints", IsReadOnly: true, IsSequence: true},
		},
		Methods: []MethodDescriptor{
			{ID: MethodID{1, 1}, Name: "Get", ResultDatatype: "NcMethodResultPropertyValue"},
			{ID: MethodID{1, 2}, Name: "Set", ResultDatatype: "NcMethodResult"},
			{ID: MethodID{1, 3}, Name: "GetSequenceItem", ResultDatatype: "NcMethodResultPropertyValue"},
			{ID: MethodID{1, 4}, Name: "SetSequenceItem", ResultDatatype: "NcMethodResult"},
			{ID: MethodID{1, 5}, Name: "AddSequenceItem", ResultDatatype: "NcMethodResult"},
			{ID: MethodID{1, 6}, Name: "RemoveSequenceItem", ResultDatatype: "NcMethodResult"},
			{ID: MethodID{1, 7}, Name: "GetSequenceLength", ResultDatatype: "NcMethodResultPropertyValue"},
		},
		Events: []EventDescriptor{
			{ID: EventID{1, 1}, Name: "PropertyChanged", EventDatatype: "NcPropertyChangedEventData"},
		},
	}

	m.Classes["1.1"] = ClassDescriptor{
		Name:    "NcBlock",
		ClassID: []int{1, 1},
		Properties: []PropertyDescriptor{
			{ID: PropertyID{2, 1}, Name: "enabled", TypeName: "NcBoolean", IsReadOnly: true},
			{ID: PropertyID{2, 2}, Name: "members", TypeName: "NcBlockMemberDescriptor", IsReadOnly: true, IsSequence: true},
		},
		Methods: []MethodDescriptor{
			{ID: MethodID{2, 1}, Name: "GetMemberDescriptors", ResultDatatype: "NcMethodResultBlockMemberDescriptors"},
			{ID: MethodID{2, 2}, Name: "FindMembersByPath", ResultDatatype: "NcMethodResultBlockMemberDescriptors"},
			{ID: MethodID{2, 3}, Name: "FindMembersByRole", ResultDatatype: "NcMethodResultBlockMemberDescriptors"},
			{ID: MethodID{2, 4}, Name: "FindMembersByClassId", ResultDatatype: "NcMethodResultBlockMemberDescriptors"},
		},
	}

	m.Classes["1.2"] = ClassDescriptor{
		Name:    "NcWorker",
		ClassID: []int{1, 2},
		Properties: []PropertyDescriptor{
			{ID: PropertyID{2, 1}, Name: "enabled", TypeName: "NcBoolean"},
		},
	}

	m.Classes["1.3.1"] = ClassDescriptor{
		Name:      "NcDeviceManager",
		ClassID:   []int{1, 3, 1},
		FixedRole: ptrString("DeviceManager"),
		Properties: []PropertyDescriptor{
			{ID: PropertyID{3, 1}, Name: "ncVersion", TypeName: "NcVersionCode", IsReadOnly: true},
			{ID: PropertyID{3, 2}, Name: "manufacturer", TypeName: "NcManufacturer", IsReadOnly: true},
			{ID: PropertyID{3, 3}, Name: "product", TypeName: "NcProduct", IsReadOnly: true},
			{ID: PropertyID{3, 4}, Name: "serialNumber", TypeName: "NcString", IsReadOnly: true},
			{ID: PropertyID{3, 5}, Name: "userInventoryCode", TypeName: "NcString", IsNullable: true},
			{ID: PropertyID{3, 6}, Name: "deviceName", TypeName: "NcString", IsNullable: true},
			{ID: PropertyID{3, 7}, Name: "deviceRole", TypeName: "NcString", IsNullable: true},
			{ID: PropertyID{3, 8}, Name: "operationalState", TypeName: "NcDeviceOperationalState", IsReadOnly: true},
			{ID: PropertyID{3, 9}, Name: "resetCause", TypeName: "NcResetCause", IsReadOnly: true},
			{ID: PropertyID{3, 10}, Name: "message", TypeName: "NcString", IsReadOnly: true, IsNullable: true},
		},
	}

	m.Classes["1.3.2"] = ClassDescriptor{
		Name:      "NcClassManager",
		ClassID:   []int{1, 3, 2},
		FixedRole: ptrString("ClassManager"),
		Properties: []PropertyDescriptor{
			{ID: PropertyID{3, 1}, Name: "controlClasses", TypeName: "NcClassDescriptor", IsReadOnly: true, IsSequence: true},
			{ID: PropertyID{3, 2}, Name: "datatypes", TypeName: "NcDatatypeDescriptor", IsReadOnly: true, IsSequence: true},
		},
		Methods: []MethodDescriptor{
			{ID: MethodID{3, 1}, Name: "GetControlClass", ResultDatatype: "NcMethodResultClassDescriptor"},
			{ID: MethodID{3, 2}, Name: "GetDatatype", ResultDatatype: "NcMethodResultDatatypeDescriptor"},
		},
	}
}

func (m *ClassManager) GetProperty(id PropertyID) (interface{}, error) {
	if id.Level == ClassManagerPropertyControlClassesLevel && id.Index == ClassManagerPropertyControlClassesIndex {
		classes := make([]ClassDescriptor, 0, len(m.Classes))
		for _, c := range m.Classes {
			classes = append(classes, c)
		}
		return classes, nil
	}
	if id.Level == ClassManagerPropertyDatatypesLevel && id.Index == ClassManagerPropertyDatatypesIndex {
		return []interface{}{}, nil
	}
	return m.BaseObject.GetProperty(id)
}

func (m *ClassManager) InvokeMethod(id MethodID, args json.RawMessage) (interface{}, error) {
	if id.Level == ClassManagerMethodGetControlClassLevel && id.Index == ClassManagerMethodGetControlClassIndex {
		var callArgs struct {
			ClassID          []int `json:"classId"`
			IncludeInherited bool  `json:"includeInherited"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}

		key := classIDToKey(callArgs.ClassID)
		if class, ok := m.Classes[key]; ok {
			return methodResult{Status: StatusOk, Value: class}, nil
		}
		return methodResult{Status: StatusBadOid, Value: nil}, nil
	}

	if id.Level == ClassManagerMethodGetDatatypeLevel && id.Index == ClassManagerMethodGetDatatypeIndex {
		var callArgs struct {
			Name             string `json:"name"`
			IncludeInherited bool   `json:"includeInherited"`
		}
		if err := json.Unmarshal(args, &callArgs); err != nil {
			return methodResult{Status: StatusBadCommandFormat}, nil
		}

		if dt, ok := m.Datatypes[callArgs.Name]; ok {
			return methodResult{Status: StatusOk, Value: dt}, nil
		}
		return methodResult{Status: StatusBadOid, Value: nil}, nil
	}

	return m.BaseObject.InvokeMethod(id, args)
}

func (m *ClassManager) RegisterClass(classID []int, descriptor ClassDescriptor) {
	m.Classes[classIDToKey(classID)] = descriptor
}

func classIDToKey(classID []int) string {
	key := ""
	for i, v := range classID {
		if i > 0 {
			key += "."
		}
		key += fmt.Sprint(v)
	}
	return key
}

func ptrString(s string) *string {
	return &s
}

func PtrInt(i int) *int {
	return &i
}
