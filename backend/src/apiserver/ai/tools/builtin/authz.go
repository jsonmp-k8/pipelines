// Copyright 2024 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package builtin

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/kubeflow/pipelines/backend/src/apiserver/common"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

// checkAccess enforces RBAC authorization for AI tool operations.
// In single-user mode, this is a no-op.
func checkAccess(ctx context.Context, rm *resource.ResourceManager, namespace, verb, resourceType string) error {
	if !common.IsMultiUserMode() {
		return nil
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required in multi-user mode")
	}
	resourceAttributes := &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      verb,
		Group:     common.RbacPipelinesGroup,
		Version:   common.RbacPipelinesVersion,
		Resource:  resourceType,
	}
	return rm.IsAuthorized(ctx, resourceAttributes)
}

// checkRunAccess checks authorization for a specific run by resolving its namespace.
func checkRunAccess(ctx context.Context, rm *resource.ResourceManager, runID, verb string) error {
	if !common.IsMultiUserMode() {
		return nil
	}
	run, err := rm.GetRun(runID)
	if err != nil {
		return fmt.Errorf("failed to get run for authorization: %w", err)
	}
	namespace := run.Namespace
	if rm.IsEmptyNamespace(namespace) {
		experiment, err := rm.GetExperiment(run.ExperimentId)
		if err != nil {
			return fmt.Errorf("failed to get experiment for authorization: %w", err)
		}
		namespace = experiment.Namespace
	}
	return checkAccess(ctx, rm, namespace, verb, common.RbacResourceTypeRuns)
}

// checkPipelineAccess checks authorization for a specific pipeline by resolving its namespace.
func checkPipelineAccess(ctx context.Context, rm *resource.ResourceManager, pipelineID, verb string) error {
	if !common.IsMultiUserMode() {
		return nil
	}
	pipeline, err := rm.GetPipeline(pipelineID)
	if err != nil {
		return fmt.Errorf("failed to get pipeline for authorization: %w", err)
	}
	return checkAccess(ctx, rm, pipeline.Namespace, verb, common.RbacResourceTypePipelines)
}

// checkExperimentAccess checks authorization for a specific experiment by resolving its namespace.
func checkExperimentAccess(ctx context.Context, rm *resource.ResourceManager, experimentID, verb string) error {
	if !common.IsMultiUserMode() {
		return nil
	}
	experiment, err := rm.GetExperiment(experimentID)
	if err != nil {
		return fmt.Errorf("failed to get experiment for authorization: %w", err)
	}
	return checkAccess(ctx, rm, experiment.Namespace, verb, common.RbacResourceTypeExperiments)
}
