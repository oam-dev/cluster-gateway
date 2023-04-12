/*
Copyright 2023 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import "errors"

type emptyCredentialTypeClusterSecretError struct{}

func (e emptyCredentialTypeClusterSecretError) Error() string {
	return "secret is not a valid cluster secret, no credential type found"
}

// NewEmptyCredentialTypeClusterSecretError create an invalid cluster secret error due to empty credential type
func NewEmptyCredentialTypeClusterSecretError() error {
	return emptyCredentialTypeClusterSecretError{}
}

type emptyEndpointClusterSecretError struct{}

func (e emptyEndpointClusterSecretError) Error() string {
	return "secret is not a valid cluster secret, no credential type found"
}

// NewEmptyEndpointClusterSecretError create an invalid cluster secret error due to empty endpoint
func NewEmptyEndpointClusterSecretError() error {
	return emptyEndpointClusterSecretError{}
}

// IsInvalidClusterSecretError check if an error is an invalid cluster secret error
func IsInvalidClusterSecretError(err error) bool {
	return errors.As(err, &emptyCredentialTypeClusterSecretError{}) || errors.As(err, &emptyEndpointClusterSecretError{})
}

type invalidManagedClusterError struct{}

func (e invalidManagedClusterError) Error() string {
	return "managed cluster has no client config"
}

// NewInvalidManagedClusterError create an invalid managed cluster error
func NewInvalidManagedClusterError() error {
	return invalidManagedClusterError{}
}

// IsInvalidManagedClusterError check if an error is an invalid managed cluster error
func IsInvalidManagedClusterError(err error) bool {
	return errors.As(err, &invalidManagedClusterError{})
}
