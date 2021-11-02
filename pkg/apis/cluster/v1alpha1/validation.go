/*
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

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateClusterGateway(c *ClusterGateway) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, ValidateClusterGatewaySpec(&c.Spec, field.NewPath("spec"))...)
	return errs
}

func ValidateClusterGatewaySpec(c *ClusterGatewaySpec, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(c.Provider) == 0 {
		errs = append(errs, field.Required(path.Child("provider"), "should set provider"))
	}
	errs = append(errs, ValidateClusterGatewaySpecAccess(&c.Access, path.Child("access"))...)
	return errs
}

func ValidateClusterGatewaySpecAccess(c *ClusterAccess, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	switch c.Endpoint.Type {
	case ClusterEndpointTypeConst:
		if len(c.Endpoint.Const.Address) == 0 {
			errs = append(errs, field.Required(path.Child("endpoint"), "should provide cluster endpoint"))
		}
		u, err := url.Parse(c.Endpoint.Const.Address)
		if err != nil {
			errs = append(errs, field.Invalid(path.Child("endpoint"), c.Endpoint, fmt.Sprintf("failed parsing as URL: %v", err)))
			return errs
		}
		if u.Scheme != "https" {
			errs = append(errs, field.Invalid(path.Child("endpoint"), c.Endpoint, "scheme must be https"))
		}
		if len(c.Endpoint.Const.CABundle) == 0 &&
			(c.Endpoint.Const.Insecure == nil || *c.Endpoint.Const.Insecure == false) {
			errs = append(errs, field.Required(path.Child("caBundle"), "required for non-insecure endpoint"))
		}
	}
	if c.Credential != nil {
		errs = append(errs, ValidateClusterGatewaySpecAccessCredential(c.Credential, path.Child("credential"))...)
	}
	return errs
}

func ValidateClusterGatewaySpecAccessCredential(c *ClusterAccessCredential, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	supportedCredTypes := sets.NewString(string(CredentialTypeServiceAccountToken), string(CredentialTypeX509Certificate))
	if !supportedCredTypes.Has(string(c.Type)) {
		errs = append(errs, field.NotSupported(path.Child("type"), c.Type, supportedCredTypes.List()))
	}
	switch c.Type {
	case CredentialTypeServiceAccountToken:
		if _, err := base64.StdEncoding.DecodeString(c.ServiceAccountToken); err == nil {
			errs = append(errs, field.Invalid(path.Child("serviceAccountToken"), c.ServiceAccountToken, "should not be base64 encoded"))
		}
		if len(c.ServiceAccountToken) == 0 {
			errs = append(errs, field.Required(path.Child("serviceAccountToken"), "should provide service-account token"))
		}
	case CredentialTypeX509Certificate:
		if c.X509 == nil {
			errs = append(errs, field.Required(path.Child("x509"), "should provide x509 certificate and private-key"))
		} else {
			if len(c.X509.Certificate) == 0 {
				errs = append(errs, field.Required(path.Child("x509").Child("certificate"), "should provide x509 certificate"))
			}
			if len(c.X509.PrivateKey) == 0 {
				errs = append(errs, field.Required(path.Child("x509").Child("privateKey"), "should provide x509 private key"))
			}
			// TODO: test if certificate and private-key matches modulus
		}
	}
	return errs
}
