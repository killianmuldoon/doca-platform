/*
Copyright 2024 NVIDIA

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

package powercycle

import (
	"fmt"
	"strings"
)

const (
	OverrideKey = "nvidia.com/dpuOperator-override-powercycle-command"
	Cycle       = "cycle"
	Reset       = "reset"
	Manual      = ""
	// Skip ensures no power cycle is done on the host.
	// Note: This may cause issues with the BFB firmware installation and configuration.
	Skip = "skip"
)

func Validate(m map[string]string) error {
	v, ok := m[OverrideKey]
	if !ok {
		return nil
	}
	supported := []string{Cycle, Reset, Manual, Skip}
	for _, s := range supported {
		if v == s {
			return nil
		}
	}
	return fmt.Errorf("invalid value %q, supported values: %q", v, supported)
}

func GenerateCmd(m map[string]string) (string, error) {
	if err := Validate(m); err != nil {
		return "", err
	}
	cmd := []string{"ipmitool", "chassis", "power"}
	v, ok := m[OverrideKey]
	if !ok {
		cmd = append(cmd, Cycle)
		return strings.Join(cmd, " "), nil
	}

	switch v {
	case Cycle:
		cmd = append(cmd, Cycle)
	case Reset:
		cmd = append(cmd, Reset)
	case Skip:
		cmd = []string{Skip}
	case Manual:
		return "", nil
	}
	return strings.Join(cmd, " "), nil
}
