// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

// Typed representations of the real VirtFusion server API shape observed
// against this fork's deployment (see CODE-27). This intentionally does not
// attempt to model fields that have not been observed on the wire.

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
// endpoints (detail additionally includes State).
type APIServer struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Hostname       string   `json:"hostname"`
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

	// Memory/CPU are free-form strings on the wire (e.g. "10240 MB",
	// "2 Core"); parsed numeric companions are derived best-effort in the
	// resource's Read logic, not here.
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
// Limit is a string on the wire (observed live, not the numeric type
// originally assumed from planning notes) — kept as raw string rather than
// guessing at parsing semantics that haven't been confirmed.
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
