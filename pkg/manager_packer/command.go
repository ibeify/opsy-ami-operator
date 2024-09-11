/*
Copyright 2024.

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

package manager_packer

import (
	"context"
	"strings"

	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
)

func (m *ManagerPacker) ConstructCommand(ctx context.Context, packerBuilder *amiv1alpha1.PackerBuilder) error {
	m.ensureDefaultCommands(packerBuilder)

	var commands []string
	seenBuild := false

	for _, cmd := range packerBuilder.Spec.Builder.Commands {
		if cmd.SubCommand == "build" {
			seenBuild = true
		}

		if !seenBuild || cmd.SubCommand == "build" {
			commands = append(commands, cmd.String())
		}
	}

	packerBuilder.Status.Command = strings.Join(commands, " && ")
	return m.Status.Update(ctx, packerBuilder)
}

func (m *ManagerPacker) ensureDefaultCommands(packerBuilder *amiv1alpha1.PackerBuilder) {
	commands := &packerBuilder.Spec.Builder.Commands

	m.ensureCommandAt(commands, 0, "init")

	m.ensureCommandAt(commands, 1, "validate")

	buildIndex := m.findCommandIndex(*commands, "build")
	if buildIndex == -1 {
		*commands = append(*commands, amiv1alpha1.Command{SubCommand: "build"})
	} else if buildIndex < 2 {
		m.moveCommandToIndex(commands, buildIndex, 2)
	}

	for i := range *commands {
		m.updateCommand(packerBuilder, i)
	}
}

func (m *ManagerPacker) ensureCommandAt(commands *[]amiv1alpha1.Command, index int, subCommand string) {
	if index >= len(*commands) || (*commands)[index].SubCommand != subCommand {
		cmd := amiv1alpha1.Command{
			SubCommand: subCommand,
			Color:      false,
			WorkingDir: "",
		}
		*commands = append((*commands)[:index], append([]amiv1alpha1.Command{cmd}, (*commands)[index:]...)...)
	}
}

func (m *ManagerPacker) findCommandIndex(commands []amiv1alpha1.Command, subCommand string) int {
	for i, cmd := range commands {
		if cmd.SubCommand == subCommand {
			return i
		}
	}
	return -1
}

func (m *ManagerPacker) moveCommandToIndex(commands *[]amiv1alpha1.Command, from, to int) {
	cmd := (*commands)[from]
	*commands = append((*commands)[:from], (*commands)[from+1:]...)
	*commands = append((*commands)[:to], append([]amiv1alpha1.Command{cmd}, (*commands)[to:]...)...)
}

func (m *ManagerPacker) updateCommand(packerBuilder *amiv1alpha1.PackerBuilder, index int) {
	cmd := &packerBuilder.Spec.Builder.Commands[index]

	if cmd.WorkingDir == "" && packerBuilder.Spec.Builder.Dir != "" {
		cmd.WorkingDir = packerBuilder.Spec.Builder.Dir + "/"
	}

	if cmd.SubCommand == "build" && packerBuilder.Spec.AMIFilters != nil {
		if cmd.Variables == nil {
			cmd.Variables = make(map[string]string)
		}
		cmd.Variables["ami_id"] = packerBuilder.Status.LastRunBaseImageID
		cmd.Variables["region"] = packerBuilder.Spec.Region
	}
}
