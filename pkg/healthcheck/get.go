/*
Copyright 2023.

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
package healthcheck

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Throw a request to kube-apiserver on the remote Kubernetes cluster
// and check if it can communicate successfully.
// The following function throws a request to the "livez" API endpoint.
// https://kubernetes.io/docs/reference/using-api/health-checks/
func HealthChecks(target clientcmdapi.Cluster) error {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// Since no communication is made using certificates,　certificate
				// verification is skipped.
				InsecureSkipVerify: true,
			},
		},
		// If no response is received within 2 seconds,
		// the communication is considered to have failed.
		Timeout: 2 * time.Second,
	}

	u := fmt.Sprintf("%s%s", target.Server, "/livez")
	resp, err := client.Get(u)
	if err != nil {
		return fmt.Errorf("failed to get response from %s: %w", u, err)
	}
	defer resp.Body.Close()

	return nil
}
