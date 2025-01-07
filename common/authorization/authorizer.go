// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination authority_mock.go -self_package github.com/uber/cadence/common/authorization

package authorization

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	clientworker "go.uber.org/cadence/worker"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

const (
	// DecisionDeny means auth decision is deny
	DecisionDeny Decision = iota + 1
	// DecisionAllow means auth decision is allow
	DecisionAllow
)

const (
	// PermissionRead means the user can write on the domain level APIs
	PermissionRead Permission = iota + 1
	// PermissionWrite means the user can write on the domain level APIs
	PermissionWrite
	// PermissionAdmin means the user can read+write on the domain level APIs
	PermissionAdmin
)

type (
	domainData map[string]string
	// Attributes is input for authority to make decision.
	// It can be extended in future if required auth on resources like WorkflowType and TaskList
	Attributes struct {
		Actor        string
		APIName      string
		DomainName   string
		WorkflowType *types.WorkflowType
		TaskList     *types.TaskList
		Permission   Permission
		RequestBody  FilteredRequestBody // request object except for data inputs (PII)
	}

	// Result is result from authority.
	Result struct {
		Decision Decision
	}

	// Decision is enum type for auth decision
	Decision int

	// Permission is enum type for auth permission
	Permission int
)

func NewPermission(permission string) Permission {
	switch permission {
	case "read":
		return PermissionRead
	case "write":
		return PermissionWrite
	case "admin":
		return PermissionAdmin
	default:
		return -1
	}
}

func (d domainData) Groups(groupType string) []string {
	res, ok := d[groupType]
	if !ok {
		return nil
	}
	return strings.Split(res, groupSeparator)
}

// Authorizer is an interface for authorization
type Authorizer interface {
	Authorize(ctx context.Context, attributes *Attributes) (Result, error)
}

func GetAuthProviderClient(privateKey string) (clientworker.AuthorizationProvider, error) {
	pk, err := os.ReadFile(privateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key path %s", privateKey)
	}
	return clientworker.NewAdminJwtAuthorizationProvider(pk), nil
}

// FilteredRequestBody request object except for data inputs (PII)
type FilteredRequestBody interface {
	SerializeForLogging() (string, error)
}

type simpleRequestLogWrapper struct {
	request interface{}
}

func (f *simpleRequestLogWrapper) SerializeForLogging() (string, error) {
	// We have to check if the request is a typed nil. In the interface we have to handle typed nils.
	// The reflection check  is slow but this function is doing json marshalling, so performance
	// shouldn't be an issue.
	if f.request == nil || reflect.ValueOf(f.request).IsNil() {
		return "", nil
	}

	res, err := json.Marshal(f.request)
	if err != nil {
		return "", err
	}

	return string(res), nil
}

func NewFilteredRequestBody(request interface{}) FilteredRequestBody {
	return &simpleRequestLogWrapper{request}
}

func validatePermission(claims *JWTClaims, attributes *Attributes, data domainData) error {
	if (attributes.Permission < PermissionRead) || (attributes.Permission > PermissionAdmin) {
		return fmt.Errorf("permission %v is not supported", attributes.Permission)
	}

	allowedGroups := map[string]bool{}
	// groups that allowed by domain configuration(in domainData)
	// write groups are always checked
	for _, g := range data.Groups(common.DomainDataKeyForWriteGroups) {
		allowedGroups[g] = true
	}

	if attributes.Permission == PermissionRead {
		for _, g := range data.Groups(common.DomainDataKeyForReadGroups) {
			allowedGroups[g] = true
		}
	}

	for _, jwtGroup := range claims.GetGroups() {
		if _, ok := allowedGroups[jwtGroup]; ok {
			return nil
		}
	}

	return fmt.Errorf("token doesn't have the right permission, jwt groups: %v, allowed groups: %v", claims.GetGroups(), allowedGroups)
}
