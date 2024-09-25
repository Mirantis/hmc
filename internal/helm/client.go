// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewMemoryRESTClientGetter(cfg *rest.Config, mapper meta.RESTMapper) *MemoryRESTClientGetter {
	return &MemoryRESTClientGetter{
		Config:     cfg,
		RestMapper: mapper,
	}
}

type MemoryRESTClientGetter struct {
	Config     *rest.Config
	RestMapper meta.RESTMapper
}

func (c *MemoryRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.Config, nil
}

func (c *MemoryRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	client, err := discovery.NewDiscoveryClientForConfig(c.Config)
	return memory.NewMemCacheClient(client), err
}

func (c *MemoryRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return c.RestMapper, nil
}

func (*MemoryRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
}
