/*
Nexodus API

This is the Nexodus API Server.

API version: 1.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package public

// ModelsAddDevice struct for ModelsAddDevice
type ModelsAddDevice struct {
	AdvertiseCidrs  []string         `json:"advertise_cidrs,omitempty"`
	Endpoints       []ModelsEndpoint `json:"endpoints,omitempty"`
	Hostname        string           `json:"hostname,omitempty"`
	Ipv4TunnelIps   []ModelsTunnelIP `json:"ipv4_tunnel_ips,omitempty"`
	Os              string           `json:"os,omitempty"`
	PublicKey       string           `json:"public_key,omitempty"`
	Relay           bool             `json:"relay,omitempty"`
	SecurityGroupId string           `json:"security_group_id,omitempty"`
	SymmetricNat    bool             `json:"symmetric_nat,omitempty"`
	VpcId           string           `json:"vpc_id,omitempty"`
}
