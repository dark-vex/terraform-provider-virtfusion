// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

// Typed representations of the real VirtFusion API shape, taken from the
// account's own OpenAPI spec (https://vps.hostbrr.com/account/api) plus
// live GET verification. This intentionally does not model fields that
// haven't been confirmed against either source.

// APIServerListEnvelope is the Laravel-style pagination envelope returned by
// GET /server.
type APIServerListEnvelope struct {
	CurrentPage int         `json:"current_page"`
	Data        []APIServer `json:"data"`
	Total       int         `json:"total"`
}

// APIServerDetailEnvelope is returned by GET /server/{id}.
type APIServerDetailEnvelope struct {
	Data APIServer `json:"data"`
}

// APIServer is the server object as returned by both the list and detail
// endpoints.
type APIServer struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Hostname       *string  `json:"hostname"`
	Suspended      bool     `json:"suspended"`
	Protected      bool     `json:"protected"`
	Migrating      bool     `json:"migrating"`
	Deleting       bool     `json:"deleting"`
	BackupCreating bool     `json:"backupCreating"`
	Rescue         bool     `json:"rescue"`
	VNCEnabled     bool     `json:"vncEnabled"`
	ISOMounted     bool     `json:"isoMounted"`
	UEFI           bool     `json:"uefi"`
	BootOrder      []string `json:"bootOrder"`

	// Memory/CPU/storage capacity are free-form display strings on the wire
	// (e.g. "10240 MB", "2 Core", "80 GB"); numeric companions are derived
	// best-effort in the resource's Read logic, not here.
	Memory string `json:"memory"`
	CPU    string `json:"cpu"`

	Storage []APIStorageItem `json:"storage"`
	Network APINetwork       `json:"network"`

	CurrentMonthlyPeriod APIMonthlyPeriod `json:"currentMonthlyPeriod"`
	Created              string           `json:"created"`

	// State has only ever been observed as null; semantics are unconfirmed.
	State *string `json:"state"`
}

// APIStorageItem is one entry of the server's storage array.
type APIStorageItem struct {
	Capacity string `json:"capacity"`
	Enabled  bool   `json:"enabled"`
	Primary  bool   `json:"primary"`
	Created  string `json:"created"`
}

// APINetwork holds the primary NIC and any secondary NICs.
type APINetwork struct {
	Primary   APINetworkInterface   `json:"primary"`
	Secondary []APINetworkInterface `json:"secondary"`
}

// APINetworkInterface is one network interface (primary or secondary).
//
// Limit is a string on the wire (observed live as e.g. "2000 GB"), kept as
// raw string rather than guessing at parsing semantics.
type APINetworkInterface struct {
	MAC   string    `json:"mac"`
	Limit string    `json:"limit"`
	IPv4  []APIIPv4 `json:"ipv4"`
	IPv6  []APIIPv6 `json:"ipv6"`
}

type APIIPv4 struct {
	Address string `json:"address"`
	Gateway string `json:"gateway"`
	Netmask string `json:"netmask"`
}

type APIIPv6 struct {
	Subnet    string   `json:"subnet"`
	Gateway   string   `json:"gateway"`
	Addresses []string `json:"addresses"`
}

// APIMonthlyPeriod is the server's current billing period window.
type APIMonthlyPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// APITaskEnvelope wraps a task handle, returned by mutating endpoints that
// trigger an async job (build, bootOrder, restart, etc).
type APITaskEnvelope struct {
	Data struct {
		Task APITask `json:"task"`
	} `json:"data"`
}

// APITask is the status of a single async job, also returned directly (not
// wrapped) by GET /server/{serverId}/task/{taskId}.
type APITask struct {
	ID        int    `json:"id"`
	Action    string `json:"action"`
	Started   string `json:"started"`
	Updated   string `json:"updated"`
	Finished  string `json:"finished"`
	Completed bool   `json:"completed"`
	Status    string `json:"status"`
	Success   bool   `json:"success"`
}

// APISSHKey is one entry in the account's SSH key list.
type APISSHKey struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
	Type      string `json:"type"`
	Enabled   bool   `json:"enabled"`
	Created   string `json:"created"`
}

// APISSHKeyListEnvelope is the Laravel-style pagination envelope returned by
// GET /account/sshKeys (and, on this deployment, also by POST
// /account/sshKeys — the create response is the full updated list, not the
// single created object).
type APISSHKeyListEnvelope struct {
	Data        []APISSHKey `json:"data"`
	NextPageURL *string     `json:"next_page_url"`
	Total       int         `json:"total"`
}

// APICreateServerResponse is the bare response from
// POST /resourcePack/{resourcePackId}/{createId} — no envelope, unlike
// almost every other endpoint.
type APICreateServerResponse struct {
	ID string `json:"id"`
}
