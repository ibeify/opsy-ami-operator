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
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"

	configurations "github.com/ibeify/opsy-ami-operator/pkg/config"

	corev1 "k8s.io/api/core/v1"
)

func (m *ManagerPacker) StreamLogs(ctx context.Context, name, namespace string) error {
	req := m.Clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Follow: true,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	return m.processLogStream(ctx, stream)
}

func (m *ManagerPacker) processLogStream(ctx context.Context, stream io.ReadCloser) error {
	scanner := bufio.NewScanner(stream)
	patterns := m.getLogPatterns()

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			m.Log.Info("Log processing stopped due to context cancellation")
			return ctx.Err()
		default:
			line := scanner.Text()
			if finished := m.processLogLine(ctx, line, patterns); finished {
				return nil
			}
		}
	}

	if err := scanner.Err(); err != nil {

		switch {
		case err == context.Canceled:
			m.Log.Info("Log processing stopped due to context cancellation")
			return ctx.Err()
		case err == context.DeadlineExceeded:
			m.Log.Info("Log streaming timed out")
			return nil
		default:
			m.Log.Error(err, "Error streaming logs")
			return err
		}

	}

	return nil
}

func (m *ManagerPacker) getLogPatterns() []RegExsy {
	return []RegExsy{
		{
			Name:    "BaseAMIID",
			Pattern: regexp.MustCompile(string(configurations.BaseAMIPattern)),
			Found:   new(bool),
			StatusField: func(match string) {
				m.PackerBuilder.Status.LastRunBaseImageID = match
			},
		},
		{
			Name:    "LatestAMIID",
			Pattern: regexp.MustCompile(string(configurations.LastestAMIPattern)),
			Found:   new(bool),
			StatusField: func(match string) {
				m.PackerBuilder.Status.LastRunBuiltImageID = match
			},
		},

		{
			Name:    "SecurityGroupID",
			Pattern: regexp.MustCompile(string(configurations.SecurityGroupPattern)),
			Found:   new(bool),
			StatusField: func(match string) {
				m.PackerBuilder.Status.LastRunSecurityGroupID = match
			},
		},

		{
			Name:    "KeyPair",
			Pattern: regexp.MustCompile(string(configurations.KeyPairPattern)),
			Found:   new(bool),
			StatusField: func(match string) {
				m.PackerBuilder.Status.LastRunKeyPair = match
			},
		},
	}
}

func (m *ManagerPacker) processLogLine(ctx context.Context, line string, patterns []RegExsy) bool {
	allFound := true
	for _, p := range patterns {
		if !*p.Found && p.Pattern.MatchString(line) {
			*p.Found = true
			match := p.Pattern.FindStringSubmatch(line)[1]
			p.StatusField(match)
			m.Log.V(1).Info(fmt.Sprintf("Found match: %s -> %s", p.Name, match), p.Name, match)
		}
		if !*p.Found {
			allFound = false
		}
	}

	if allFound {
		m.Log.Info("All values found, stopping log stream")

		if err := m.Status.Update(ctx, m.PackerBuilder); err != nil {
			m.Log.Error(err, "Error updating PackerBuilder status")
		}

		amiUpdateResult := m.handleAMIUpdates(ctx, m.PackerBuilder)

		if !amiUpdateResult.Success {
			m.Log.Error(amiUpdateResult.Error, "Error updating AMI")
			return false
		}

		m.Log.Info(amiUpdateResult.Message)
		return true
	}
	return false
}
