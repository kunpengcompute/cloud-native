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
	"testing"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/k8s-mpam-controller/v1alpha1"
)

func TestTranslateQoSPolicySpec(t *testing.T) {
	spec := qosv1alpha1.QoSPolicySpec{
		MB: qosv1alpha1.MBPolicy{HDL: 1, PRI: 3, MIN: 10, MAX: 90},
		L3: qosv1alpha1.L3Policy{PRI: 2, MIN: 20, MAX: 80, Ways: 4},
	}

	got := translateQoSPolicySpec(spec)

	if got.MBHDL != spec.MB.HDL ||
		got.MBPRI != spec.MB.PRI ||
		got.MBMIN != spec.MB.MIN ||
		got.MB != spec.MB.MAX ||
		got.L3PRI != spec.L3.PRI ||
		got.L3MIN != spec.L3.MIN ||
		got.L3MAX != spec.L3.MAX ||
		got.L3 != spec.L3.Ways {
		t.Fatalf("translateQoSPolicySpec() got=%+v, want mapped values from spec=%+v", got, spec)
	}
}
