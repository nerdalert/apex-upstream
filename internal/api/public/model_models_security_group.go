/*
Nexodus API

This is the Nexodus API Server.

API version: 1.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package public

// ModelsSecurityGroup struct for ModelsSecurityGroup
type ModelsSecurityGroup struct {
	GroupDescription string               `json:"group_description,omitempty"`
	GroupName        string               `json:"group_name,omitempty"`
	Id               string               `json:"id,omitempty"`
	InboundRules     []ModelsSecurityRule `json:"inbound_rules,omitempty"`
	OrgId            string               `json:"org_id,omitempty"`
	OutboundRules    []ModelsSecurityRule `json:"outbound_rules,omitempty"`
}
