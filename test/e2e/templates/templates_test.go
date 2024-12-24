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

package templates

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

func Test_Find(t *testing.T) {
	templates := map[string][]hmc.AvailableUpgrade{
		"aws-standalone-cp-0-0-1": {
			{Name: "aws-standalone-cp-0-0-3"},
			{Name: "aws-standalone-cp-0-0-2"},
		},
		"aws-standalone-cp-0-0-3": {},
		"aws-standalone-cp-0-0-2": {},
		"aws-hosted-cp-0-0-1": {
			{Name: "aws-hosted-cp-0-0-4"},
			{Name: "aws-hosted-cp-0-0-2"},
		},
		"aws-hosted-cp-0-0-4": {},
		"aws-hosted-cp-0-0-2": {},
		"azure-standalone-cp-0-0-1": {
			{Name: "azure-standalone-cp-0-0-2"},
		},
		"azure-hosted-cp-0-0-1": {
			{Name: "azure-hosted-cp-0-0-2"},
		},
		"vsphere-standalone-cp-0-0-1": {
			{Name: "vsphere-standalone-cp-0-0-2"},
		},
		"vsphere-hosted-cp-0-0-1": {},
	}
	for _, tt := range []struct {
		title                   string
		upgrade                 bool
		sourceTemplate          string
		clusterTemplates        map[string][]hmc.AvailableUpgrade
		templateType            Type
		expectedTemplate        string
		expectedUpgradeTemplate string
		expectedErr             error
	}{
		{
			title:        "no templates of the provided type supported",
			templateType: "aws-unsupported-cp",
			expectedErr:  errors.New("no Template of the aws-unsupported-cp type is supported"),
		},
		{
			title:            "should find latest template for aws-hosted-cp",
			templateType:     TemplateAWSHostedCP,
			expectedTemplate: "aws-hosted-cp-0-0-4",
		},
		{
			title:        "upgrade: no upgrades are available for this type of templates",
			upgrade:      true,
			templateType: TemplateVSphereHostedCP,
			expectedErr:  errors.New("invalid templates configuration. No vsphere-hosted-cp templates are available for the upgrade"),
		},
		{
			title:          "upgrade: source template provided but it's not supported",
			upgrade:        true,
			sourceTemplate: "aws-standalone-cp-0-0-1-1",
			templateType:   TemplateAWSStandaloneCP,
			expectedErr:    errors.New("invalid templates configuration. Template aws-standalone-cp-0-0-1-1 is not in the list of supported templates"),
		},
		{
			title:          "upgrade: source template provided but no upgrades are available",
			upgrade:        true,
			sourceTemplate: "aws-standalone-cp-0-0-3",
			templateType:   TemplateAWSStandaloneCP,
			expectedErr:    errors.New("invalid templates configuration. No upgrades are available from the Template aws-standalone-cp-0-0-3"),
		},
		{
			title:                   "upgrade: source template provided and the upgrade template was found",
			upgrade:                 true,
			sourceTemplate:          "aws-standalone-cp-0-0-1",
			templateType:            TemplateAWSStandaloneCP,
			expectedTemplate:        "aws-standalone-cp-0-0-1",
			expectedUpgradeTemplate: "aws-standalone-cp-0-0-3",
		},
		{
			title:        "upgrade: no templates are available for the upgrade for this type of templates",
			upgrade:      true,
			templateType: TemplateVSphereHostedCP,
			expectedErr:  errors.New("invalid templates configuration. No vsphere-hosted-cp templates are available for the upgrade"),
		},
		{
			title:                   "upgrade: should find latest template with available upgrades",
			upgrade:                 true,
			templateType:            TemplateAWSHostedCP,
			expectedTemplate:        "aws-hosted-cp-0-0-1",
			expectedUpgradeTemplate: "aws-hosted-cp-0-0-4",
		},
	} {
		t.Run(tt.title, func(t *testing.T) {
			g := NewWithT(t)
			var template, upgradeTemplate string
			var err error
			if tt.upgrade {
				template, upgradeTemplate, err = FindTemplatesToUpgrade(templates, tt.templateType, tt.sourceTemplate)
			} else {
				template, err = FindTemplate(templates, tt.templateType)
			}
			if tt.expectedErr != nil {
				g.Expect(err).To(MatchError(tt.expectedErr))
			} else {
				g.Expect(err).To(Succeed())
			}
			g.Expect(template).To(Equal(tt.expectedTemplate))
			g.Expect(upgradeTemplate).To(Equal(tt.expectedUpgradeTemplate))
		})
	}
}
