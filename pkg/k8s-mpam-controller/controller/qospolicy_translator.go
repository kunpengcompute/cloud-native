/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.

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

package controller

import (
	"fmt"

	mpamv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/k8s-mpam-controller/v1alpha1"
)

// DefaultPolicyTranslator converts QoSPolicySpec into ResctrlConfig with
// defensive validation.
type DefaultPolicyTranslator struct{}

// Translate validates QoSPolicySpec and maps it to ResctrlConfig.
func (t DefaultPolicyTranslator) Translate(spec mpamv1alpha1.QoSPolicySpec) (ResctrlConfig, error) {
	if err := validatePolicySpec(spec); err != nil {
		return ResctrlConfig{}, err
	}

	return ResctrlConfig{
		MBHDL: spec.MB.HDL,
		MBPRI: spec.MB.PRI,
		L3PRI: spec.L3.PRI,
		MBMIN: spec.MB.MIN,
		L3MIN: spec.L3.MIN,
		L3MAX: spec.L3.MAX,
		MB:    spec.MB.MAX,
		L3:    spec.L3.Ways,
	}, nil
}

func validatePolicySpec(spec mpamv1alpha1.QoSPolicySpec) error {
	if err := validateRange("mb.hdl", spec.MB.HDL, 0, 1); err != nil {
		return err
	}
	if err := validateRange("mb.pri", spec.MB.PRI, 0, 7); err != nil {
		return err
	}
	if err := validateRange("l3.pri", spec.L3.PRI, 0, 3); err != nil {
		return err
	}
	if err := validateRange("mb.min", spec.MB.MIN, 0, 100); err != nil {
		return err
	}
	if err := validateRange("l3.min", spec.L3.MIN, 0, 100); err != nil {
		return err
	}
	if err := validateRange("l3.max", spec.L3.MAX, 0, 100); err != nil {
		return err
	}
	if err := validateRange("mb.max", spec.MB.MAX, 0, 100); err != nil {
		return err
	}
	if err := validateRange("l3.ways", spec.L3.Ways, 1, 1<<30); err != nil {
		return err
	}
	if spec.L3.MIN > spec.L3.MAX {
		return fmt.Errorf("invalid l3 range: l3.min(%d) must be <= l3.max(%d)", spec.L3.MIN, spec.L3.MAX)
	}

	return nil
}

func validateRange(name string, value int32, min int32, max int32) error {
	if value < min || value > max {
		return fmt.Errorf("invalid %s: %d, expected in [%d, %d]", name, value, min, max)
	}
	return nil
}
