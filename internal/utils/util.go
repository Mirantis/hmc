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

// SliceToMapKeys converts a given slice to a map with slice's values
// as the map's keys zeroing value for each.
func SliceToMapKeys[S ~[]K, M ~map[K]V, K comparable, V any](s S) M {
	m := make(M)
	for i := range s {
		m[s[i]] = *new(V)
	}
	return m
}

// DiffSliceSubset finds missing items of a given slice in a given map.
// If the slice is a subset of the map, returns empty slice.
// Boolean return argument indicates whether the slice is a subset.
func DiffSliceSubset[S ~[]K, M ~map[K]V, K comparable, V any](s S, m M) (diff S, isSubset bool) {
	for _, v := range s {
		if _, ok := m[v]; !ok {
			diff = append(diff, v)
		}
	}

	return diff, len(diff) == 0
}
