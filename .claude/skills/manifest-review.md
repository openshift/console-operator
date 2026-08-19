---
name: manifest-review
description: Review RBAC manifests and CVO manifest files for cluster profile annotations and correctness
tags: [review, manifest, rbac, cvo]
---

# Manifest Review Skill

Review Kubernetes manifest files in `manifests/`, `bindata/assets/`, `quickstarts/`, and `examples/` directories.

## What to Check

### 1. Cluster Profile Annotations
All CVO manifests MUST include appropriate cluster profile annotations:

```yaml
annotations:
  include.release.openshift.io/hypershift: "true"
  include.release.openshift.io/ibm-cloud-managed: "true"
  include.release.openshift.io/self-managed-high-availability: "true"
  include.release.openshift.io/single-node-developer: "true"
  capability.openshift.io/name: Console
```

**Common profiles:**
- `hypershift` - HyperShift hosted control planes
- `ibm-cloud-managed` - IBM Cloud managed OpenShift
- `self-managed-high-availability` - Standard self-managed HA clusters
- `single-node-developer` - Single-node OpenShift (SNO)

### 2. Capability Annotations
Console resources should include:
```yaml
capability.openshift.io/name: Console
```

### 3. RBAC Rules
Check RBAC manifests for:
- **Principle of least privilege**: Only grant necessary permissions
- **Resource specificity**: Avoid wildcards unless truly needed
- **Verb appropriateness**: Match verbs to actual needs (get/list/watch vs create/update/delete)
- **apiGroups correctness**: Use proper API groups ("" for core, specific groups for CRDs)

### 4. Namespace Consistency
- Resources in `openshift-console` namespace for console workload
- Resources in `openshift-console-operator` namespace for operator
- Cross-namespace references are explicit and intentional

### 5. YAML Formatting
- Proper indentation (2 spaces)
- Consistent ordering of fields
- Include `---` separator between multiple resources in one file

### 6. Service Account References
When binding roles to service accounts, verify which SA is appropriate:

**Operator SA (cross-namespace bindings):**
```yaml
subjects:
  - kind: ServiceAccount
    name: console-operator
    namespace: openshift-console-operator
```

**Console workload SA (same-namespace bindings):**
```yaml
subjects:
  - kind: ServiceAccount
    name: console
    namespace: openshift-console
```

> Verify which SA is appropriate based on what the role grants access to:
> use `console-operator` when the operator needs cross-namespace permissions,
> and `console` when the binding is scoped to the console workload in `openshift-console`.

### 7. Profile-Specific Patches
Check `profile-patches/` for overrides that should only apply to specific deployments.

## Red Flags

- Missing cluster profile annotations on new manifests
- RBAC with `*` wildcards without clear justification
- Cross-namespace permissions without clear need
- Missing capability annotations
- Hardcoded values that should be configurable

## Output Format

For each issue:
- **File**: manifest filename
- **Issue**: What's wrong
- **Impact**: Which clusters/profiles are affected
- **Fix**: Recommended correction

## Example Review Comments

**Critical**: manifests/new-role.yaml missing cluster profile annotations. This resource won't be deployed to any clusters.

**Warning**: RBAC grants `*` verbs on configmaps. Consider restricting to specific verbs needed.

**Info**: Consider adding hypershift annotation if this resource is needed for hosted control planes.
