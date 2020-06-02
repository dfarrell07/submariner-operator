/*
© 2019 Red Hat, Inc. and others.

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

package subscription

import (
	"k8s.io/client-go/rest"

	"github.com/submariner-io/submariner-operator/pkg/subctl/operator/common/olmsubscription"
)

func Available(restConfig *rest.Config) (bool, error) {
	return olmsubscription.Available(restConfig)
}

func Ensure(restConfig *rest.Config, namespace string) (bool, error) {
	return olmsubscription.Ensure(restConfig, namespace, "submariner-operator")
}
