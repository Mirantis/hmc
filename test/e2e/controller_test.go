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

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Controller", Label("controller"), Ordered, func() {
	// Right now we have no Controller specific tests, but our Before/AfterSuite
	// deploys and validates the controller, so we need a dummy test here to
	// ensure that Before/AfterSuite are ran.  With this we can make sure we
	// atleast smoke test the controller outside of the larger provider e2e
	// tests.  When the controller actually has more specific tests this should
	// be removed.
	It("dummy It so that BeforeSuite/AfterSuite are ran", func() {
		_, _ = fmt.Printf("Running dummy test for controller")
	})
})
