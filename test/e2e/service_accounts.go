/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package e2e

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/version"
	"k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var serviceAccountTokenNamespaceVersion = version.MustParse("v1.2.0")

var _ = Describe("ServiceAccounts", func() {
	f := NewDefaultFramework("svcaccounts")

	It("should ensure a single API token exists", func() {
		// wait for the service account to reference a single secret
		var secrets []api.ObjectReference
		expectNoError(wait.Poll(time.Millisecond*500, time.Second*10, func() (bool, error) {
			By("waiting for a single token reference")
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			if apierrors.IsNotFound(err) {
				Logf("default service account was not found")
				return false, nil
			}
			if err != nil {
				Logf("error getting default service account: %v", err)
				return false, err
			}
			switch len(sa.Secrets) {
			case 0:
				Logf("default service account has no secret references")
				return false, nil
			case 1:
				Logf("default service account has a single secret reference")
				secrets = sa.Secrets
				return true, nil
			default:
				return false, fmt.Errorf("default service account has too many secret references: %#v", sa.Secrets)
			}
		}))

		// make sure the reference doesn't flutter
		{
			By("ensuring the single token reference persists")
			time.Sleep(2 * time.Second)
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			expectNoError(err)
			Expect(sa.Secrets).To(Equal(secrets))
		}

		// delete the referenced secret
		By("deleting the service account token")
		expectNoError(f.Client.Secrets(f.Namespace.Name).Delete(secrets[0].Name))

		// wait for the referenced secret to be removed, and another one autocreated
		expectNoError(wait.Poll(time.Millisecond*500, serviceAccountProvisionTimeout, func() (bool, error) {
			By("waiting for a new token reference")
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			if err != nil {
				Logf("error getting default service account: %v", err)
				return false, err
			}
			switch len(sa.Secrets) {
			case 0:
				Logf("default service account has no secret references")
				return false, nil
			case 1:
				if sa.Secrets[0] == secrets[0] {
					Logf("default service account still has the deleted secret reference")
					return false, nil
				}
				Logf("default service account has a new single secret reference")
				secrets = sa.Secrets
				return true, nil
			default:
				return false, fmt.Errorf("default service account has too many secret references: %#v", sa.Secrets)
			}
		}))

		// make sure the reference doesn't flutter
		{
			By("ensuring the single token reference persists")
			time.Sleep(2 * time.Second)
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			expectNoError(err)
			Expect(sa.Secrets).To(Equal(secrets))
		}

		// delete the reference from the service account
		By("deleting the reference to the service account token")
		{
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			expectNoError(err)
			sa.Secrets = nil
			_, updateErr := f.Client.ServiceAccounts(f.Namespace.Name).Update(sa)
			expectNoError(updateErr)
		}

		// wait for another one to be autocreated
		expectNoError(wait.Poll(time.Millisecond*500, serviceAccountProvisionTimeout, func() (bool, error) {
			By("waiting for a new token to be created and added")
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			if err != nil {
				Logf("error getting default service account: %v", err)
				return false, err
			}
			switch len(sa.Secrets) {
			case 0:
				Logf("default service account has no secret references")
				return false, nil
			case 1:
				Logf("default service account has a new single secret reference")
				secrets = sa.Secrets
				return true, nil
			default:
				return false, fmt.Errorf("default service account has too many secret references: %#v", sa.Secrets)
			}
		}))

		// make sure the reference doesn't flutter
		{
			By("ensuring the single token reference persists")
			time.Sleep(2 * time.Second)
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			expectNoError(err)
			Expect(sa.Secrets).To(Equal(secrets))
		}
	})

	It("should mount an API token into pods [Conformance]", func() {
		var tokenContent string
		var rootCAContent string

		// Standard get, update retry loop
		expectNoError(wait.Poll(time.Millisecond*500, serviceAccountProvisionTimeout, func() (bool, error) {
			By("getting the auto-created API token")
			sa, err := f.Client.ServiceAccounts(f.Namespace.Name).Get("default")
			if apierrors.IsNotFound(err) {
				Logf("default service account was not found")
				return false, nil
			}
			if err != nil {
				Logf("error getting default service account: %v", err)
				return false, err
			}
			if len(sa.Secrets) == 0 {
				Logf("default service account has no secret references")
				return false, nil
			}
			for _, secretRef := range sa.Secrets {
				secret, err := f.Client.Secrets(f.Namespace.Name).Get(secretRef.Name)
				if err != nil {
					Logf("Error getting secret %s: %v", secretRef.Name, err)
					continue
				}
				if secret.Type == api.SecretTypeServiceAccountToken {
					tokenContent = string(secret.Data[api.ServiceAccountTokenKey])
					rootCAContent = string(secret.Data[api.ServiceAccountRootCAKey])
					return true, nil
				}
			}

			Logf("default service account has no secret references to valid service account tokens")
			return false, nil
		}))

		pod := &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name: "pod-service-account-" + string(util.NewUUID()),
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name:  "token-test",
						Image: "index.alauda.cn/googlecontainer/mounttest:0.2",
						Args: []string{
							fmt.Sprintf("--file_content=%s/%s", serviceaccount.DefaultAPITokenMountPath, api.ServiceAccountTokenKey),
						},
					},
					{
						Name:  "root-ca-test",
						Image: "index.alauda.cn/googlecontainer/mounttest:0.2",
						Args: []string{
							fmt.Sprintf("--file_content=%s/%s", serviceaccount.DefaultAPITokenMountPath, api.ServiceAccountRootCAKey),
						},
					},
				},
				RestartPolicy: api.RestartPolicyNever,
			},
		}

		supportsTokenNamespace, _ := serverVersionGTE(serviceAccountTokenNamespaceVersion, f.Client)
		if supportsTokenNamespace {
			pod.Spec.Containers = append(pod.Spec.Containers, api.Container{
				Name:  "namespace-test",
				Image: "index.alauda.cn/googlecontainer/mounttest:0.2",
				Args: []string{
					fmt.Sprintf("--file_content=%s/%s", serviceaccount.DefaultAPITokenMountPath, api.ServiceAccountNamespaceKey),
				},
			})
		}

		f.TestContainerOutput("consume service account token", pod, 0, []string{
			fmt.Sprintf(`content of file "%s/%s": %s`, serviceaccount.DefaultAPITokenMountPath, api.ServiceAccountTokenKey, tokenContent),
		})
		f.TestContainerOutput("consume service account root CA", pod, 1, []string{
			fmt.Sprintf(`content of file "%s/%s": %s`, serviceaccount.DefaultAPITokenMountPath, api.ServiceAccountRootCAKey, rootCAContent),
		})

		if supportsTokenNamespace {
			f.TestContainerOutput("consume service account namespace", pod, 2, []string{
				fmt.Sprintf(`content of file "%s/%s": %s`, serviceaccount.DefaultAPITokenMountPath, api.ServiceAccountNamespaceKey, f.Namespace.Name),
			})
		}
	})
})
