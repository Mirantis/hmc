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

package utils

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureDeleteAllOf(ctx context.Context, cl client.Client, gvk schema.GroupVersionKind, opts *client.ListOptions) error {
	itemsList := &v1.PartialObjectMetadataList{}
	itemsList.SetGroupVersionKind(gvk)
	if err := cl.List(ctx, itemsList, opts); err != nil {
		return err
	}
	var errs error
	for _, item := range itemsList.Items {
		if item.DeletionTimestamp.IsZero() {
			if err := cl.Delete(ctx, &item); err != nil && !apierrors.IsNotFound(err) {
				errs = errors.Join(err)
				continue
			}
		}
		errs = errors.Join(fmt.Errorf("waiting for %s %s/%s removal", gvk.Kind, item.Namespace, item.Name))
	}
	return errs
}
