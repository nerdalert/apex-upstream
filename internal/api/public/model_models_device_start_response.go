/*
Nexodus API

This is the Nexodus API Server.

API version: 1.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package public

import (
	"time"
)

// ModelsDeviceStartResponse struct for ModelsDeviceStartResponse
type ModelsDeviceStartResponse struct {
	ClientId string `json:"client_id,omitempty"`
	// TODO: Remove this once golang/oauth2 supports device flow and when coreos/go-oidc adds device_authorization_endpoint discovery
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint,omitempty"`
	Issuer                      string `json:"issuer,omitempty"`
	// the current time on the server, can be used by a client to get an idea of what the time skew is in relation to the server.
	ServerTime time.Time `json:"server_time,omitempty"`
}
