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

package scheme

import (
	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

var (
	Scheme  = runtime.NewScheme()
	Codecs  = serializer.NewCodecFactory(Scheme)
	Builder = runtime.SchemeBuilder{
		clientgoscheme.AddToScheme,
		v1alpha1.AddToScheme,
		sourcev1.AddToScheme,
		hcv2.AddToScheme,
	}
)
var Encoder = json.NewYAMLSerializer(json.DefaultMetaFactory, Scheme, Scheme)

func init() {
	err := Builder.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}
}

func Decode(yaml []byte) (runtime.Object, error) {
	return runtime.Decode(Codecs.UniversalDeserializer(), yaml)
}

func Encode(obj runtime.Object) ([]byte, error) {
	return runtime.Encode(Encoder, obj)
}
