package infrastructure

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Database interface {
	Close() error

	GetDevices() ([]Device, error)
	GetDevice(id string) (*Device, error)
	UpsertDevice(device Device) error
	UpdateDeviceTags(id string, tags map[string]interface{}) error
	DeleteDevice(id string) error

	GetSources() ([]Source, error)
	GetSource(id string) (*Source, error)
	GetSourcesByDevice(deviceID string) ([]Source, error)
	UpsertSource(source Source) error
	DeleteSource(id string) error

	GetFlows() ([]Flow, error)
	GetFlow(id string) (*Flow, error)
	GetFlowsBySource(sourceID string) ([]Flow, error)
	UpsertFlow(flow Flow) error
	DeleteFlow(id string) error

	GetSenders() ([]Sender, error)
	GetSender(id string) (*Sender, error)
	GetSendersByDevice(deviceID string) ([]Sender, error)
	UpsertSender(sender Sender) error
	DeleteSender(id string) error

	GetReceivers() ([]Receiver, error)
	GetReceiver(id string) (*Receiver, error)
	UpsertReceiver(receiver Receiver) error
	DeleteReceiver(id string) error

	GetShureDevices() ([]ShureDevice, error)
	GetShureDevice(address string) (*ShureDevice, error)
	UpsertShureDevice(dev ShureDevice) error
	UpdateShureDeviceLastSeen(address string) error
	DeleteShureDevice(address string) error

	GetNCPObject(oid int) (*NCPObject, error)
	GetNCPObjectsByOwner(ownerOID int) ([]NCPObject, error)
	UpsertNCPObject(obj NCPObject) error
	UpdateNCPObjectProperty(oid int, propertyID string, value interface{}) error
	DeleteNCPObject(oid int) error

	GetLastEvent(sourceID string) (*IS07Event, error)
	UpsertIS07Event(event IS07Event) error

	OnDeviceChange(callback func(eventType string, device *Device))
	OnSourceChange(callback func(eventType string, source *Source))
	OnFlowChange(callback func(eventType string, flow *Flow))
	OnSenderChange(callback func(eventType string, sender *Sender))
	OnNCPObjectChange(callback func(eventType string, obj *NCPObject))
}

type Device struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Label       string                 `json:"label"`
	Description string                 `json:"description"`
	NodeID      string                 `json:"node_id"`
	Tags        map[string]interface{} `json:"tags"`
	Senders     []string               `json:"senders"`
	Receivers   []string               `json:"receivers"`
	Controls    []interface{}          `json:"controls"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Source struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Label       string                 `json:"label"`
	Description string                 `json:"description"`
	Format      string                 `json:"format"`
	DeviceID    string                 `json:"device_id"`
	EventType   string                 `json:"event_type"`
	Tags        map[string]interface{} `json:"tags"`
	GrainRate   map[string]int        `json:"grain_rate"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Flow struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Label       string                 `json:"label"`
	Description string                 `json:"description"`
	Format      string                 `json:"format"`
	SourceID    string                 `json:"source_id"`
	DeviceID    string                 `json:"device_id"`
	Parents     []string               `json:"parents"`
	Tags        map[string]interface{} `json:"tags"`
	MediaType   string                 `json:"media_type"`
	EventType   string                 `json:"event_type"`
	GrainRate   map[string]int        `json:"grain_rate"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type Sender struct {
	ID                 string                 `json:"id"`
	Version            string                 `json:"version"`
	Label              string                 `json:"label"`
	Description        string                 `json:"description"`
	DeviceID           string                 `json:"device_id"`
	FlowID             string                 `json:"flow_id"`
	Transport          string                 `json:"transport"`
	InterfaceBindings  []string               `json:"interface_bindings"`
	ManifestHref       *string                `json:"manifest_href"`
	Tags               map[string]interface{} `json:"tags"`
	TransportParams    []map[string]interface{} `json:"transport_params"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type Receiver struct {
	ID                 string                 `json:"id"`
	Version            string                 `json:"version"`
	Label              string                 `json:"label"`
	Description        string                 `json:"description"`
	DeviceID           string                 `json:"device_id"`
	Transport          string                 `json:"transport"`
	InterfaceBindings  []string               `json:"interface_bindings"`
	ManifestHref       *string                `json:"manifest_href"`
	Tags               map[string]interface{} `json:"tags"`
	TransportParams    []map[string]interface{} `json:"transport_params"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type ShureDevice struct {
	Address         string            `json:"address"`
	DeviceID       string            `json:"device_id"`
	DeviceOID      int               `json:"device_oid"`
	DeviceInstance string            `json:"device_instance"`
	ModelFamily    string            `json:"model_family"`
	LastSeen       time.Time         `json:"last_seen"`
	ParameterOIDs  map[string]int   `json:"parameter_oids"`
	ChannelOIDs    map[int]int       `json:"channel_oids"`
	SourceIDs      map[int]map[string]string `json:"source_ids"`
	FlowIDs        map[int]map[string]string `json:"flow_ids"`
	SenderIDs      map[int]map[string]string `json:"sender_ids"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type NCPObject struct {
	OID        int                      `json:"oid"`
	ClassID    []int                    `json:"class_id"`
	Role       string                   `json:"role"`
	Label      string                   `json:"label"`
	Properties map[string]interface{}    `json:"properties"`
	OwnerOID   *int                     `json:"owner_oid"`
	CreatedAt  time.Time                `json:"created_at"`
	UpdatedAt  time.Time                `json:"updated_at"`
}

type IS07Event struct {
	SourceID  string                 `json:"source_id"`
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time               `json:"timestamp"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type sqliteDatabase struct {
	db *sql.DB
	mu sync.RWMutex

	deviceCallbacks    []func(eventType string, device *Device)
	sourceCallbacks    []func(eventType string, source *Source)
	flowCallbacks      []func(eventType string, flow *Flow)
	senderCallbacks    []func(eventType string, sender *Sender)
	ncpObjectCallbacks []func(eventType string, obj *NCPObject)
}

func NewSQLiteDatabase(path string) (Database, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB := &sqliteDatabase{db: db}

	if err := sqlDB.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return sqlDB, nil
}

func NewInMemorySQLiteDatabase() (Database, error) {
	return NewSQLiteDatabase(":memory:")
}

func (d *sqliteDatabase) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		version TEXT NOT NULL,
		label TEXT NOT NULL,
		description TEXT,
		node_id TEXT,
		tags TEXT,
		senders TEXT,
		receivers TEXT,
		controls TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sources (
		id TEXT PRIMARY KEY,
		version TEXT NOT NULL,
		label TEXT NOT NULL,
		description TEXT,
		format TEXT NOT NULL,
		device_id TEXT NOT NULL,
		event_type TEXT,
		tags TEXT,
		grain_rate TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (device_id) REFERENCES devices(id)
	);

	CREATE TABLE IF NOT EXISTS flows (
		id TEXT PRIMARY KEY,
		version TEXT NOT NULL,
		label TEXT NOT NULL,
		description TEXT,
		format TEXT NOT NULL,
		source_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		parents TEXT,
		tags TEXT,
		media_type TEXT,
		event_type TEXT,
		grain_rate TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (source_id) REFERENCES sources(id),
		FOREIGN KEY (device_id) REFERENCES devices(id)
	);

	CREATE TABLE IF NOT EXISTS senders (
		id TEXT PRIMARY KEY,
		version TEXT NOT NULL,
		label TEXT NOT NULL,
		description TEXT,
		device_id TEXT NOT NULL,
		flow_id TEXT NOT NULL,
		transport TEXT NOT NULL,
		interface_bindings TEXT,
		manifest_href TEXT,
		tags TEXT,
		transport_params TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (device_id) REFERENCES devices(id),
		FOREIGN KEY (flow_id) REFERENCES flows(id)
	);

	CREATE TABLE IF NOT EXISTS receivers (
		id TEXT PRIMARY KEY,
		version TEXT NOT NULL,
		label TEXT NOT NULL,
		description TEXT,
		device_id TEXT NOT NULL,
		transport TEXT NOT NULL,
		interface_bindings TEXT,
		manifest_href TEXT,
		tags TEXT,
		transport_params TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (device_id) REFERENCES devices(id)
	);

	CREATE TABLE IF NOT EXISTS shure_devices (
		address TEXT PRIMARY KEY,
		device_id TEXT NOT NULL,
		device_oid INTEGER,
		device_instance TEXT,
		model_family TEXT,
		last_seen DATETIME,
		parameter_oids TEXT,
		channel_oids TEXT,
		source_ids TEXT,
		flow_ids TEXT,
		sender_ids TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (device_id) REFERENCES devices(id)
	);

	CREATE TABLE IF NOT EXISTS ncp_objects (
		oid INTEGER PRIMARY KEY,
		class_id TEXT NOT NULL,
		role TEXT NOT NULL,
		label TEXT,
		properties TEXT,
		owner_oid INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS is07_events (
		source_id TEXT PRIMARY KEY,
		event_type TEXT,
		data TEXT,
		timestamp DATETIME,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_sources_device_id ON sources(device_id);
	CREATE INDEX IF NOT EXISTS idx_flows_source_id ON flows(source_id);
	CREATE INDEX IF NOT EXISTS idx_flows_device_id ON flows(device_id);
	CREATE INDEX IF NOT EXISTS idx_senders_device_id ON senders(device_id);
	CREATE INDEX IF NOT EXISTS idx_senders_flow_id ON senders(flow_id);
	CREATE INDEX IF NOT EXISTS idx_shure_devices_device_id ON shure_devices(device_id);
	CREATE INDEX IF NOT EXISTS idx_ncp_objects_owner_oid ON ncp_objects(owner_oid);
	`

	_, err := d.db.Exec(schema)
	return err
}

func (d *sqliteDatabase) Close() error {
	return d.db.Close()
}

func (d *sqliteDatabase) jsonMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func (d *sqliteDatabase) jsonUnmarshalMap(str string) map[string]interface{} {
	var result map[string]interface{}
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalSlice(str string) []string {
	var result []string
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalIntMap(str string) map[string]int {
	var result map[string]int
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalStringToStringMap(str string) map[int]map[string]string {
	var result map[int]map[string]string
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalStringToIntMap(str string) map[int]int {
	var result map[int]int
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalInterfaceSlice(str string) []interface{} {
	var result []interface{}
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalIntSlice(str string) []int {
	var result []int
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalStringIntMap(str string) map[string]int {
	var result map[string]int
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalIntIntMap(str string) map[int]int {
	var result map[int]int
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalStringStringMapMap(str string) map[int]map[string]string {
	var result map[int]map[string]string
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

func (d *sqliteDatabase) jsonUnmarshalStringStringMap(str string) map[string]string {
	var result map[string]string
	if str == "" || str == "null" {
		return result
	}
	json.Unmarshal([]byte(str), &result)
	return result
}

// Device operations

func (d *sqliteDatabase) GetDevices() ([]Device, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, node_id, tags, senders, receivers, controls, created_at, updated_at
		FROM devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var dev Device
		var tags, senders, receivers, controls sql.NullString
		err := rows.Scan(&dev.ID, &dev.Version, &dev.Label, &dev.Description, &dev.NodeID,
			&tags, &senders, &receivers, &controls, &dev.CreatedAt, &dev.UpdatedAt)
		if err != nil {
			return nil, err
		}
		dev.Tags = d.jsonUnmarshalMap(tags.String)
		dev.Senders = d.jsonUnmarshalSlice(senders.String)
		dev.Receivers = d.jsonUnmarshalSlice(receivers.String)
		dev.Controls = d.jsonUnmarshalInterfaceSlice(controls.String)
		devices = append(devices, dev)
	}
	return devices, rows.Err()
}

func (d *sqliteDatabase) GetDevice(id string) (*Device, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var dev Device
	var tags, senders, receivers, controls sql.NullString
	err := d.db.QueryRow(`
		SELECT id, version, label, description, node_id, tags, senders, receivers, controls, created_at, updated_at
		FROM devices WHERE id = ?`, id).Scan(
		&dev.ID, &dev.Version, &dev.Label, &dev.Description, &dev.NodeID,
		&tags, &senders, &receivers, &controls, &dev.CreatedAt, &dev.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dev.Tags = d.jsonUnmarshalMap(tags.String)
	dev.Senders = d.jsonUnmarshalSlice(senders.String)
	dev.Receivers = d.jsonUnmarshalSlice(receivers.String)
	dev.Controls = d.jsonUnmarshalInterfaceSlice(controls.String)
	return &dev, nil
}

func (d *sqliteDatabase) UpsertDevice(device Device) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if device.CreatedAt.IsZero() {
		device.CreatedAt = now
	}
	device.UpdatedAt = now

	_, err := d.db.Exec(`
		INSERT INTO devices (id, version, label, description, node_id, tags, senders, receivers, controls, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			label = excluded.label,
			description = excluded.description,
			node_id = excluded.node_id,
			tags = excluded.tags,
			senders = excluded.senders,
			receivers = excluded.receivers,
			controls = excluded.controls,
			updated_at = excluded.updated_at`,
		device.ID, device.Version, device.Label, device.Description, device.NodeID,
		d.jsonMarshal(device.Tags), d.jsonMarshal(device.Senders), d.jsonMarshal(device.Receivers), d.jsonMarshal(device.Controls),
		device.CreatedAt, device.UpdatedAt)

	if err != nil {
		return err
	}

	for _, cb := range d.deviceCallbacks {
		go cb("upsert", &device)
	}

	return nil
}

func (d *sqliteDatabase) UpdateDeviceTags(id string, tags map[string]interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		UPDATE devices SET tags = ?, updated_at = ? WHERE id = ?`,
		d.jsonMarshal(tags), time.Now(), id)
	return err
}

func (d *sqliteDatabase) DeleteDevice(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM devices WHERE id = ?", id)
	return err
}

// Source operations

func (d *sqliteDatabase) GetSources() ([]Source, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, format, device_id, event_type, tags, grain_rate, created_at, updated_at
		FROM sources ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		var tags, grainRate sql.NullString
		err := rows.Scan(&src.ID, &src.Version, &src.Label, &src.Description, &src.Format,
			&src.DeviceID, &src.EventType, &tags, &grainRate, &src.CreatedAt, &src.UpdatedAt)
		if err != nil {
			return nil, err
		}
		src.Tags = d.jsonUnmarshalMap(tags.String)
		src.GrainRate = d.jsonUnmarshalStringIntMap(grainRate.String)
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func (d *sqliteDatabase) GetSource(id string) (*Source, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var src Source
	var tags, grainRate sql.NullString
	err := d.db.QueryRow(`
		SELECT id, version, label, description, format, device_id, event_type, tags, grain_rate, created_at, updated_at
		FROM sources WHERE id = ?`, id).Scan(
		&src.ID, &src.Version, &src.Label, &src.Description, &src.Format,
		&src.DeviceID, &src.EventType, &tags, &grainRate, &src.CreatedAt, &src.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	src.Tags = d.jsonUnmarshalMap(tags.String)
	src.GrainRate = d.jsonUnmarshalStringIntMap(grainRate.String)
	return &src, nil
}

func (d *sqliteDatabase) GetSourcesByDevice(deviceID string) ([]Source, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, format, device_id, event_type, tags, grain_rate, created_at, updated_at
		FROM sources WHERE device_id = ? ORDER BY created_at DESC`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		var tags, grainRate sql.NullString
		err := rows.Scan(&src.ID, &src.Version, &src.Label, &src.Description, &src.Format,
			&src.DeviceID, &src.EventType, &tags, &grainRate, &src.CreatedAt, &src.UpdatedAt)
		if err != nil {
			return nil, err
		}
		src.Tags = d.jsonUnmarshalMap(tags.String)
		src.GrainRate = d.jsonUnmarshalStringIntMap(grainRate.String)
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func (d *sqliteDatabase) UpsertSource(source Source) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if source.CreatedAt.IsZero() {
		source.CreatedAt = now
	}
	source.UpdatedAt = now

	_, err := d.db.Exec(`
		INSERT INTO sources (id, version, label, description, format, device_id, event_type, tags, grain_rate, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			label = excluded.label,
			description = excluded.description,
			format = excluded.format,
			device_id = excluded.device_id,
			event_type = excluded.event_type,
			tags = excluded.tags,
			grain_rate = excluded.grain_rate,
			updated_at = excluded.updated_at`,
		source.ID, source.Version, source.Label, source.Description, source.Format,
		source.DeviceID, source.EventType, d.jsonMarshal(source.Tags), d.jsonMarshal(source.GrainRate),
		source.CreatedAt, source.UpdatedAt)

	if err != nil {
		return err
	}

	for _, cb := range d.sourceCallbacks {
		go cb("upsert", &source)
	}

	return nil
}

func (d *sqliteDatabase) DeleteSource(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM sources WHERE id = ?", id)
	return err
}

// Flow operations

func (d *sqliteDatabase) GetFlows() ([]Flow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, format, source_id, device_id, parents, tags, media_type, event_type, grain_rate, created_at, updated_at
		FROM flows ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []Flow
	for rows.Next() {
		var flow Flow
		var parents, tags, grainRate sql.NullString
		err := rows.Scan(&flow.ID, &flow.Version, &flow.Label, &flow.Description, &flow.Format,
			&flow.SourceID, &flow.DeviceID, &parents, &tags, &flow.MediaType, &flow.EventType,
			&grainRate, &flow.CreatedAt, &flow.UpdatedAt)
		if err != nil {
			return nil, err
		}
		flow.Parents = d.jsonUnmarshalSlice(parents.String)
		flow.Tags = d.jsonUnmarshalMap(tags.String)
		flow.GrainRate = d.jsonUnmarshalStringIntMap(grainRate.String)
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

func (d *sqliteDatabase) GetFlow(id string) (*Flow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var flow Flow
	var parents, tags, grainRate sql.NullString
	err := d.db.QueryRow(`
		SELECT id, version, label, description, format, source_id, device_id, parents, tags, media_type, event_type, grain_rate, created_at, updated_at
		FROM flows WHERE id = ?`, id).Scan(
		&flow.ID, &flow.Version, &flow.Label, &flow.Description, &flow.Format,
		&flow.SourceID, &flow.DeviceID, &parents, &tags, &flow.MediaType, &flow.EventType,
		&grainRate, &flow.CreatedAt, &flow.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	flow.Parents = d.jsonUnmarshalSlice(parents.String)
	flow.Tags = d.jsonUnmarshalMap(tags.String)
	flow.GrainRate = d.jsonUnmarshalStringIntMap(grainRate.String)
	return &flow, nil
}

func (d *sqliteDatabase) GetFlowsBySource(sourceID string) ([]Flow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, format, source_id, device_id, parents, tags, media_type, event_type, grain_rate, created_at, updated_at
		FROM flows WHERE source_id = ? ORDER BY created_at DESC`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []Flow
	for rows.Next() {
		var flow Flow
		var parents, tags, grainRate sql.NullString
		err := rows.Scan(&flow.ID, &flow.Version, &flow.Label, &flow.Description, &flow.Format,
			&flow.SourceID, &flow.DeviceID, &parents, &tags, &flow.MediaType, &flow.EventType,
			&grainRate, &flow.CreatedAt, &flow.UpdatedAt)
		if err != nil {
			return nil, err
		}
		flow.Parents = d.jsonUnmarshalSlice(parents.String)
		flow.Tags = d.jsonUnmarshalMap(tags.String)
		flow.GrainRate = d.jsonUnmarshalStringIntMap(grainRate.String)
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

func (d *sqliteDatabase) UpsertFlow(flow Flow) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if flow.CreatedAt.IsZero() {
		flow.CreatedAt = now
	}
	flow.UpdatedAt = now

	_, err := d.db.Exec(`
		INSERT INTO flows (id, version, label, description, format, source_id, device_id, parents, tags, media_type, event_type, grain_rate, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			label = excluded.label,
			description = excluded.description,
			format = excluded.format,
			source_id = excluded.source_id,
			device_id = excluded.device_id,
			parents = excluded.parents,
			tags = excluded.tags,
			media_type = excluded.media_type,
			event_type = excluded.event_type,
			grain_rate = excluded.grain_rate,
			updated_at = excluded.updated_at`,
		flow.ID, flow.Version, flow.Label, flow.Description, flow.Format,
		flow.SourceID, flow.DeviceID, d.jsonMarshal(flow.Parents), d.jsonMarshal(flow.Tags),
		flow.MediaType, flow.EventType, d.jsonMarshal(flow.GrainRate),
		flow.CreatedAt, flow.UpdatedAt)

	if err != nil {
		return err
	}

	for _, cb := range d.flowCallbacks {
		go cb("upsert", &flow)
	}

	return nil
}

func (d *sqliteDatabase) DeleteFlow(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM flows WHERE id = ?", id)
	return err
}

// Sender operations

func (d *sqliteDatabase) GetSenders() ([]Sender, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, device_id, flow_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at
		FROM senders ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var senders []Sender
	for rows.Next() {
		var sender Sender
		var interfaceBindings, tags, transportParams sql.NullString
		var manifestHref sql.NullString
		err := rows.Scan(&sender.ID, &sender.Version, &sender.Label, &sender.Description,
			&sender.DeviceID, &sender.FlowID, &sender.Transport, &interfaceBindings,
			&manifestHref, &tags, &transportParams, &sender.CreatedAt, &sender.UpdatedAt)
		if err != nil {
			return nil, err
		}
		sender.InterfaceBindings = d.jsonUnmarshalSlice(interfaceBindings.String)
		if manifestHref.Valid {
			sender.ManifestHref = &manifestHref.String
		}
		sender.Tags = d.jsonUnmarshalMap(tags.String)
		var tp []map[string]interface{}
		if transportParams.String != "" {
			json.Unmarshal([]byte(transportParams.String), &tp)
		}
		sender.TransportParams = tp
		senders = append(senders, sender)
	}
	return senders, rows.Err()
}

func (d *sqliteDatabase) GetSender(id string) (*Sender, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sender Sender
	var interfaceBindings, tags, transportParams sql.NullString
	var manifestHref sql.NullString
	err := d.db.QueryRow(`
		SELECT id, version, label, description, device_id, flow_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at
		FROM senders WHERE id = ?`, id).Scan(
		&sender.ID, &sender.Version, &sender.Label, &sender.Description,
		&sender.DeviceID, &sender.FlowID, &sender.Transport, &interfaceBindings,
		&manifestHref, &tags, &transportParams, &sender.CreatedAt, &sender.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sender.InterfaceBindings = d.jsonUnmarshalSlice(interfaceBindings.String)
	if manifestHref.Valid {
		sender.ManifestHref = &manifestHref.String
	}
	sender.Tags = d.jsonUnmarshalMap(tags.String)
	var tp []map[string]interface{}
	if transportParams.String != "" {
		json.Unmarshal([]byte(transportParams.String), &tp)
	}
	sender.TransportParams = tp
	return &sender, nil
}

func (d *sqliteDatabase) GetSendersByDevice(deviceID string) ([]Sender, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, device_id, flow_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at
		FROM senders WHERE device_id = ? ORDER BY created_at DESC`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var senders []Sender
	for rows.Next() {
		var sender Sender
		var interfaceBindings, tags, transportParams sql.NullString
		var manifestHref sql.NullString
		err := rows.Scan(&sender.ID, &sender.Version, &sender.Label, &sender.Description,
			&sender.DeviceID, &sender.FlowID, &sender.Transport, &interfaceBindings,
			&manifestHref, &tags, &transportParams, &sender.CreatedAt, &sender.UpdatedAt)
		if err != nil {
			return nil, err
		}
		sender.InterfaceBindings = d.jsonUnmarshalSlice(interfaceBindings.String)
		if manifestHref.Valid {
			sender.ManifestHref = &manifestHref.String
		}
		sender.Tags = d.jsonUnmarshalMap(tags.String)
		var tp []map[string]interface{}
		if transportParams.String != "" {
			json.Unmarshal([]byte(transportParams.String), &tp)
		}
		sender.TransportParams = tp
		senders = append(senders, sender)
	}
	return senders, rows.Err()
}

func (d *sqliteDatabase) UpsertSender(sender Sender) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if sender.CreatedAt.IsZero() {
		sender.CreatedAt = now
	}
	sender.UpdatedAt = now

	var manifestHref interface{}
	if sender.ManifestHref != nil {
		manifestHref = *sender.ManifestHref
	}

	_, err := d.db.Exec(`
		INSERT INTO senders (id, version, label, description, device_id, flow_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			label = excluded.label,
			description = excluded.description,
			device_id = excluded.device_id,
			flow_id = excluded.flow_id,
			transport = excluded.transport,
			interface_bindings = excluded.interface_bindings,
			manifest_href = excluded.manifest_href,
			tags = excluded.tags,
			transport_params = excluded.transport_params,
			updated_at = excluded.updated_at`,
		sender.ID, sender.Version, sender.Label, sender.Description,
		sender.DeviceID, sender.FlowID, sender.Transport,
		d.jsonMarshal(sender.InterfaceBindings), manifestHref,
		d.jsonMarshal(sender.Tags), d.jsonMarshal(sender.TransportParams),
		sender.CreatedAt, sender.UpdatedAt)

	if err != nil {
		return err
	}

	for _, cb := range d.senderCallbacks {
		go cb("upsert", &sender)
	}

	return nil
}

func (d *sqliteDatabase) DeleteSender(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM senders WHERE id = ?", id)
	return err
}

// Receiver operations

func (d *sqliteDatabase) GetReceivers() ([]Receiver, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, version, label, description, device_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at
		FROM receivers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receivers []Receiver
	for rows.Next() {
		var receiver Receiver
		var interfaceBindings, tags, transportParams sql.NullString
		var manifestHref sql.NullString
		err := rows.Scan(&receiver.ID, &receiver.Version, &receiver.Label, &receiver.Description,
			&receiver.DeviceID, &receiver.Transport, &interfaceBindings,
			&manifestHref, &tags, &transportParams, &receiver.CreatedAt, &receiver.UpdatedAt)
		if err != nil {
			return nil, err
		}
		receiver.InterfaceBindings = d.jsonUnmarshalSlice(interfaceBindings.String)
		if manifestHref.Valid {
			receiver.ManifestHref = &manifestHref.String
		}
		receiver.Tags = d.jsonUnmarshalMap(tags.String)
		var tp []map[string]interface{}
		if transportParams.String != "" {
			json.Unmarshal([]byte(transportParams.String), &tp)
		}
		receiver.TransportParams = tp
		receivers = append(receivers, receiver)
	}
	return receivers, rows.Err()
}

func (d *sqliteDatabase) GetReceiver(id string) (*Receiver, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var receiver Receiver
	var interfaceBindings, tags, transportParams sql.NullString
	var manifestHref sql.NullString
	err := d.db.QueryRow(`
		SELECT id, version, label, description, device_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at
		FROM receivers WHERE id = ?`, id).Scan(
		&receiver.ID, &receiver.Version, &receiver.Label, &receiver.Description,
		&receiver.DeviceID, &receiver.Transport, &interfaceBindings,
		&manifestHref, &tags, &transportParams, &receiver.CreatedAt, &receiver.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	receiver.InterfaceBindings = d.jsonUnmarshalSlice(interfaceBindings.String)
	if manifestHref.Valid {
		receiver.ManifestHref = &manifestHref.String
	}
	receiver.Tags = d.jsonUnmarshalMap(tags.String)
	var tp []map[string]interface{}
	if transportParams.String != "" {
		json.Unmarshal([]byte(transportParams.String), &tp)
	}
	receiver.TransportParams = tp
	return &receiver, nil
}

func (d *sqliteDatabase) UpsertReceiver(receiver Receiver) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if receiver.CreatedAt.IsZero() {
		receiver.CreatedAt = now
	}
	receiver.UpdatedAt = now

	var manifestHref interface{}
	if receiver.ManifestHref != nil {
		manifestHref = *receiver.ManifestHref
	}

	_, err := d.db.Exec(`
		INSERT INTO receivers (id, version, label, description, device_id, transport, interface_bindings, manifest_href, tags, transport_params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version = excluded.version,
			label = excluded.label,
			description = excluded.description,
			device_id = excluded.device_id,
			transport = excluded.transport,
			interface_bindings = excluded.interface_bindings,
			manifest_href = excluded.manifest_href,
			tags = excluded.tags,
			transport_params = excluded.transport_params,
			updated_at = excluded.updated_at`,
		receiver.ID, receiver.Version, receiver.Label, receiver.Description,
		receiver.DeviceID, receiver.Transport,
		d.jsonMarshal(receiver.InterfaceBindings), manifestHref,
		d.jsonMarshal(receiver.Tags), d.jsonMarshal(receiver.TransportParams),
		receiver.CreatedAt, receiver.UpdatedAt)

	return err
}

func (d *sqliteDatabase) DeleteReceiver(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM receivers WHERE id = ?", id)
	return err
}

// Shure device operations

func (d *sqliteDatabase) GetShureDevices() ([]ShureDevice, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT address, device_id, device_oid, device_instance, model_family, last_seen,
			   parameter_oids, channel_oids, source_ids, flow_ids, sender_ids, created_at, updated_at
		FROM shure_devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []ShureDevice
	for rows.Next() {
		var dev ShureDevice
		var parameterOids, channelOids, sourceIDs, flowIDs, senderIDs sql.NullString
		err := rows.Scan(&dev.Address, &dev.DeviceID, &dev.DeviceOID, &dev.DeviceInstance,
			&dev.ModelFamily, &dev.LastSeen, &parameterOids, &channelOids, &sourceIDs, &flowIDs, &senderIDs,
			&dev.CreatedAt, &dev.UpdatedAt)
		if err != nil {
			return nil, err
		}
		dev.ParameterOIDs = d.jsonUnmarshalIntMap(parameterOids.String)
		dev.ChannelOIDs = d.jsonUnmarshalIntIntMap(channelOids.String)
		dev.SourceIDs = d.jsonUnmarshalStringStringMapMap(sourceIDs.String)
		dev.FlowIDs = d.jsonUnmarshalStringStringMapMap(flowIDs.String)
		dev.SenderIDs = d.jsonUnmarshalStringStringMapMap(senderIDs.String)
		devices = append(devices, dev)
	}
	return devices, rows.Err()
}

func (d *sqliteDatabase) GetShureDevice(address string) (*ShureDevice, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var dev ShureDevice
	var parameterOids, channelOids, sourceIDs, flowIDs, senderIDs sql.NullString
	err := d.db.QueryRow(`
		SELECT address, device_id, device_oid, device_instance, model_family, last_seen,
			   parameter_oids, channel_oids, source_ids, flow_ids, sender_ids, created_at, updated_at
		FROM shure_devices WHERE address = ?`, address).Scan(
		&dev.Address, &dev.DeviceID, &dev.DeviceOID, &dev.DeviceInstance,
		&dev.ModelFamily, &dev.LastSeen, &parameterOids, &channelOids, &sourceIDs, &flowIDs, &senderIDs,
		&dev.CreatedAt, &dev.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dev.ParameterOIDs = d.jsonUnmarshalIntMap(parameterOids.String)
	dev.ChannelOIDs = d.jsonUnmarshalIntIntMap(channelOids.String)
	dev.SourceIDs = d.jsonUnmarshalStringStringMapMap(sourceIDs.String)
	dev.FlowIDs = d.jsonUnmarshalStringStringMapMap(flowIDs.String)
	dev.SenderIDs = d.jsonUnmarshalStringStringMapMap(senderIDs.String)
	return &dev, nil
}

func (d *sqliteDatabase) UpsertShureDevice(dev ShureDevice) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if dev.CreatedAt.IsZero() {
		dev.CreatedAt = now
	}
	dev.UpdatedAt = now

	_, err := d.db.Exec(`
		INSERT INTO shure_devices (address, device_id, device_oid, device_instance, model_family, last_seen,
								  parameter_oids, channel_oids, source_ids, flow_ids, sender_ids, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(address) DO UPDATE SET
			device_id = excluded.device_id,
			device_oid = excluded.device_oid,
			device_instance = excluded.device_instance,
			model_family = excluded.model_family,
			last_seen = excluded.last_seen,
			parameter_oids = excluded.parameter_oids,
			channel_oids = excluded.channel_oids,
			source_ids = excluded.source_ids,
			flow_ids = excluded.flow_ids,
			sender_ids = excluded.sender_ids,
			updated_at = excluded.updated_at`,
		dev.Address, dev.DeviceID, dev.DeviceOID, dev.DeviceInstance, dev.ModelFamily, dev.LastSeen,
		d.jsonMarshal(dev.ParameterOIDs), d.jsonMarshal(dev.ChannelOIDs),
		d.jsonMarshal(dev.SourceIDs), d.jsonMarshal(dev.FlowIDs), d.jsonMarshal(dev.SenderIDs),
		dev.CreatedAt, dev.UpdatedAt)

	return err
}

func (d *sqliteDatabase) UpdateShureDeviceLastSeen(address string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("UPDATE shure_devices SET last_seen = ?, updated_at = ? WHERE address = ?",
		time.Now(), time.Now(), address)
	return err
}

func (d *sqliteDatabase) DeleteShureDevice(address string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM shure_devices WHERE address = ?", address)
	return err
}

// NCP object operations

func (d *sqliteDatabase) GetNCPObject(oid int) (*NCPObject, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var obj NCPObject
	var properties, classID sql.NullString
	var ownerOID sql.NullInt64
	err := d.db.QueryRow(`
		SELECT oid, class_id, role, label, properties, owner_oid, created_at, updated_at
		FROM ncp_objects WHERE oid = ?`, oid).Scan(
		&obj.OID, &classID, &obj.Role, &obj.Label, &properties, &ownerOID, &obj.CreatedAt, &obj.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	obj.ClassID = d.jsonUnmarshalIntSlice(classID.String)
	obj.Properties = d.jsonUnmarshalMap(properties.String)
	if ownerOID.Valid {
		oid := int(ownerOID.Int64)
		obj.OwnerOID = &oid
	}
	return &obj, nil
}

func (d *sqliteDatabase) GetNCPObjectsByOwner(ownerOID int) ([]NCPObject, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT oid, class_id, role, label, properties, owner_oid, created_at, updated_at
		FROM ncp_objects WHERE owner_oid = ?`, ownerOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []NCPObject
	for rows.Next() {
		var obj NCPObject
		var properties, classID sql.NullString
		var owner sql.NullInt64
		err := rows.Scan(&obj.OID, &classID, &obj.Role, &obj.Label, &properties, &owner, &obj.CreatedAt, &obj.UpdatedAt)
		if err != nil {
			return nil, err
		}
		obj.ClassID = d.jsonUnmarshalIntSlice(classID.String)
		obj.Properties = d.jsonUnmarshalMap(properties.String)
		if owner.Valid {
			oid := int(owner.Int64)
			obj.OwnerOID = &oid
		}
		objects = append(objects, obj)
	}
	return objects, rows.Err()
}

func (d *sqliteDatabase) UpsertNCPObject(obj NCPObject) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if obj.CreatedAt.IsZero() {
		obj.CreatedAt = now
	}
	obj.UpdatedAt = now

	var ownerOID interface{}
	if obj.OwnerOID != nil {
		ownerOID = *obj.OwnerOID
	}

	_, err := d.db.Exec(`
		INSERT INTO ncp_objects (oid, class_id, role, label, properties, owner_oid, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(oid) DO UPDATE SET
			class_id = excluded.class_id,
			role = excluded.role,
			label = excluded.label,
			properties = excluded.properties,
			owner_oid = excluded.owner_oid,
			updated_at = excluded.updated_at`,
		obj.OID, d.jsonMarshal(obj.ClassID), obj.Role, obj.Label,
		d.jsonMarshal(obj.Properties), ownerOID, obj.CreatedAt, obj.UpdatedAt)

	if err != nil {
		return err
	}

	for _, cb := range d.ncpObjectCallbacks {
		go cb("upsert", &obj)
	}

	return nil
}

func (d *sqliteDatabase) UpdateNCPObjectProperty(oid int, propertyID string, value interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	obj, err := d.GetNCPObject(oid)
	if err != nil || obj == nil {
		return err
	}

	if obj.Properties == nil {
		obj.Properties = make(map[string]interface{})
	}
	obj.Properties[propertyID] = value

	_, err = d.db.Exec("UPDATE ncp_objects SET properties = ?, updated_at = ? WHERE oid = ?",
		d.jsonMarshal(obj.Properties), time.Now(), oid)
	return err
}

func (d *sqliteDatabase) DeleteNCPObject(oid int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM ncp_objects WHERE oid = ?", oid)
	return err
}

// IS-07 event operations

func (d *sqliteDatabase) GetLastEvent(sourceID string) (*IS07Event, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var event IS07Event
	var data sql.NullString
	err := d.db.QueryRow(`
		SELECT source_id, event_type, data, timestamp, updated_at
		FROM is07_events WHERE source_id = ?`, sourceID).Scan(
		&event.SourceID, &event.EventType, &data, &event.Timestamp, &event.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	event.Data = d.jsonUnmarshalMap(data.String)
	return &event, nil
}

func (d *sqliteDatabase) UpsertIS07Event(event IS07Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}

	_, err := d.db.Exec(`
		INSERT INTO is07_events (source_id, event_type, data, timestamp, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET
			event_type = excluded.event_type,
			data = excluded.data,
			timestamp = excluded.timestamp,
			updated_at = excluded.updated_at`,
		event.SourceID, event.EventType, d.jsonMarshal(event.Data), event.Timestamp, event.UpdatedAt)

	return err
}

// Callback registration

func (d *sqliteDatabase) OnDeviceChange(callback func(eventType string, device *Device)) {
	d.deviceCallbacks = append(d.deviceCallbacks, callback)
}

func (d *sqliteDatabase) OnSourceChange(callback func(eventType string, source *Source)) {
	d.sourceCallbacks = append(d.sourceCallbacks, callback)
}

func (d *sqliteDatabase) OnFlowChange(callback func(eventType string, flow *Flow)) {
	d.flowCallbacks = append(d.flowCallbacks, callback)
}

func (d *sqliteDatabase) OnSenderChange(callback func(eventType string, sender *Sender)) {
	d.senderCallbacks = append(d.senderCallbacks, callback)
}

func (d *sqliteDatabase) OnNCPObjectChange(callback func(eventType string, obj *NCPObject)) {
	d.ncpObjectCallbacks = append(d.ncpObjectCallbacks, callback)
}

var _ Database = (*sqliteDatabase)(nil)