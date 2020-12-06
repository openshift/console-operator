# Contributing Quick Starts

Quick starts walk users through completing different tasks in the console. In
OpenShift 4.7, we added a
[quick start custom resource](https://github.com/openshift/enhancements/blob/master/enhancements/console/quick-starts.md).
This allows operators and administrators to contribute new quick starts to the
cluster beyond the out-of-the-box set. Typically, quick starts for operators are
created by the operator itself after the operator is installed. In a few cases,
we have out-of-the-box quick starts that guide administrators through the
process of installing an operator. These need to be created before operator
installation through OperatorHub. Any out-of-the-box quick start should be
contributed to the `quickstarts` folder here in the console-operator repo.

To contribute out-of-the-box quickstarts, follow the
[guidelines](http://openshift.github.io/openshift-origin-design/conventions/documentation/quick-starts.html)
for writing a quick start and getting the content reviewed. When the
quick start is ready, add the quick start YAML to this folder and open a PR.
Request review from `@jhadvig` and `@spadgett` on the PR.

Quick start resources in this repository should contain the following
cluster profile annotations:

```yaml
  annotations:
    include.release.openshift.io/ibm-cloud-managed: "true"
    include.release.openshift.io/self-managed-high-availability: "true"
    include.release.openshift.io/single-node-developer: "true"
    include.release.openshift.io/single-node-production-edge: "true"
```

See the
[Cluster Profiles](https://github.com/openshift/enhancements/blob/master/enhancements/update/cluster-profiles.md)
enhancement proposal for details.

## Quick Start API

To see the quick start API documentation, you can use the `oc explain` command.

```
$ oc explain consolequickstarts
```

Check `oc explain -h` for more details on `oc explain`.

Details about the API are also covered in the
[quick start enhancement proposal](https://github.com/openshift/enhancements/blob/master/enhancements/console/quick-starts.md).

## Updating Quick Starts in Previous Releases

The console-operator repo has branches for each OpenShift release. The `master`
branch tracks the next unreleased minor (`y` version) of OpenShift. Releases
that have already shipped are tracked through branches like `release-4.6`. If
you need to backport a quick start change to a previous release, you will need
a Bugzilla bug. The `/cherry-pick` bot command will automatically create a
new Bugzilla if the `master` PR had a Bugzilla attached. See the notes on
[backporting fixes](https://github.com/openshift/console/blob/master/CONTRIBUTING.md#backporting-fixes)
in the openshift/console contributing guide.

Avoid changing the resource name of existing quick starts. This will cause
duplicate quick starts to show up when upgrading from one release to the next as
the ClusterVersionOperator will not delete the old quick start with the previous
name.
