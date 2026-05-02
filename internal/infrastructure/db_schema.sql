-- Shure-NMOS Bridge Database Schema
-- SQLite3 In-Memory Database for State Management

-- Devices table (IS-04)
CREATE TABLE devices (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT,
    node_id TEXT,
    tags TEXT, -- JSON object
    senders TEXT, -- JSON array of sender IDs
    receivers TEXT, -- JSON array of receiver IDs
    controls TEXT, -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sources table (IS-04/IS-07)
CREATE TABLE sources (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT,
    format TEXT NOT NULL, -- urn:x-nmos:format:data
    device_id TEXT NOT NULL,
    event_type TEXT,
    tags TEXT, -- JSON object
    grain_rate TEXT, -- JSON object {numerator, denominator}
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id)
);

-- Flows table (IS-04)
CREATE TABLE flows (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT,
    format TEXT NOT NULL, -- urn:x-nmos:format:data
    source_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    parents TEXT, -- JSON array
    tags TEXT, -- JSON object
    media_type TEXT,
    event_type TEXT,
    grain_rate TEXT, -- JSON object
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_id) REFERENCES sources(id),
    FOREIGN KEY (device_id) REFERENCES devices(id)
);

-- Senders table (IS-04/IS-05)
CREATE TABLE senders (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT,
    device_id TEXT NOT NULL,
    flow_id TEXT NOT NULL,
    transport TEXT NOT NULL, -- urn:x-nmos:transport:websocket
    interface_bindings TEXT, -- JSON array
    manifest_href TEXT,
    tags TEXT, -- JSON object
    transport_params TEXT, -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id),
    FOREIGN KEY (flow_id) REFERENCES flows(id)
);

-- Receivers table (IS-04/IS-05)
CREATE TABLE receivers (
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

-- Shure Devices table (tracks active connections to Shure hardware)
CREATE TABLE shure_devices (
    address TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    device_oid INTEGER, -- OID of the device block in NCP tree
    device_instance TEXT, -- Device instance name from mDNS discovery
    model_family TEXT, -- AxientDigital, ULXD, QLXD, SLXD, SLXDPlus
    last_seen DATETIME,
    parameter_oids TEXT, -- JSON: param_key -> oid mapping
    channel_oids TEXT, -- JSON: channel -> OID mapping
    source_ids TEXT, -- JSON: channel -> param -> sourceID
    flow_ids TEXT, -- JSON: channel -> param -> flowID
    sender_ids TEXT, -- JSON: channel -> param -> senderID
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id)
);

-- NCP Objects table (IS-12 - Node Control Protocol)
CREATE TABLE ncp_objects (
    oid INTEGER PRIMARY KEY,
    class_id TEXT NOT NULL, -- JSON array [level, index, ...]
    role TEXT NOT NULL,
    label TEXT,
    properties TEXT, -- JSON: propertyID -> value
    owner_oid INTEGER, -- Parent block OID
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- IS-07 Event Cache (for last events per source)
CREATE TABLE is07_events (
    source_id TEXT PRIMARY KEY,
    event_type TEXT,
    data TEXT, -- JSON event payload
    timestamp DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX idx_sources_device_id ON sources(device_id);
CREATE INDEX idx_flows_source_id ON flows(source_id);
CREATE INDEX idx_flows_device_id ON flows(device_id);
CREATE INDEX idx_senders_device_id ON senders(device_id);
CREATE INDEX idx_senders_flow_id ON senders(flow_id);
CREATE INDEX idx_shure_devices_device_id ON shure_devices(device_id);
CREATE INDEX idx_ncp_objects_owner_oid ON ncp_objects(owner_oid);