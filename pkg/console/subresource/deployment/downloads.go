package deployment

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/config"
)

func DownloadsDeployment(ipVMode config.IPMode) *appsv1.Deployment {

	replicas := int32(ConsoleReplicas)

	labels := map[string]string{
		"app":       "console",
		"component": "downloads",
	}

	gracePeriod := int64(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.OpenShiftDownloadsName,
			Namespace: api.OpenShiftConsoleName,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   api.OpenShiftDownloadsName,
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					NodeSelector: nodeSelector(),
					Tolerations:  tolerations(),
					Containers: []corev1.Container{
						downloadsContainer(ipVMode),
					},
					TerminationGracePeriodSeconds: &gracePeriod,
				},
			},
		},
	}
	return deployment
}

func nodeSelector() map[string]string {
	return map[string]string{
		"kubernetes.io/os": "linux",
	}
}

func downloadsContainer(ipVMode config.IPMode) corev1.Container {
	return corev1.Container{
		Name:                     api.OpenShiftDownloadsServerName,
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Image:                    downloadsImage(),
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Ports:                    downloadsPorts(),
		LivenessProbe:            downloadsLiveness(),
		ReadinessProbe:           downloadsReadiness(),
		Command: []string{
			"/bin/sh",
		},
		Resources: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		Args: downloadsArgs(ipVMode),
	}
}

// TODO: does this already in manifests/image-references
// do we need to do anything here?  read it from command line
// like console deployment IMAGE=<image>?
func downloadsImage() string {
	return "registry.svc.ci.openshift.org/openshift:cli-artifacts"
}

func downloadsPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{{
		Name:          "http",
		Protocol:      corev1.ProtocolTCP,
		ContainerPort: 8080,
	}}
}

func downloadsLiveness() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/",
				Port:   intstr.FromInt(8080),
				Scheme: corev1.URISchemeHTTP,
			},
		},
	}
}

func downloadsReadiness() *corev1.Probe {
	probe := downloadsLiveness()
	probe.FailureThreshold = 3
	return probe
}

func downloadsArgs(ipVMode config.IPMode) []string {
	return []string{
		"-c",
		downloadsInlineScript(ipVMode),
	}
}

func downloadsInlineScript(ipVMode config.IPMode) string {
	// defaults to IPv4
	addressFamily := "AF_INET"

	// if we get a IPv4v6 value, we will have to panic as we don't know how to support both yet
	if ipVMode == config.IPv4v6Mode {
		panic(fmt.Sprintf("mode not supported: %v", ipVMode))
	}

	if ipVMode == config.IPv6Mode {
		addressFamily = "AF_INET6"
	}

	return `|
          cat <<EOF >>/tmp/serve.py
          import BaseHTTPServer, os, re, signal, SimpleHTTPServer, socket, sys, tarfile, tempfile, threading, time, zipfile

          signal.signal(signal.SIGTERM, lambda signum, frame: sys.exit(0))

          # Launch multiple listeners as threads
          class Thread(threading.Thread):
              def __init__(self, i, socket):
                  threading.Thread.__init__(self)
                  self.i = i
                  self.socket = socket
                  self.daemon = True
                  self.start()

              def run(self):
                  httpd = BaseHTTPServer.HTTPServer(addr, SimpleHTTPServer.SimpleHTTPRequestHandler, False)

                  # Prevent the HTTP server from re-binding every handler.
                  # https://stackoverflow.com/questions/46210672/
                  httpd.socket = self.socket
                  httpd.server_bind = self.server_close = lambda self: None

                  httpd.serve_forever()

          temp_dir = tempfile.mkdtemp()
          print('serving from {}'.format(temp_dir))
          os.chdir(temp_dir)
          for arch in ['amd64']:
              os.mkdir(arch)
              for operating_system in ['linux', 'mac', 'windows']:
                  os.mkdir(os.path.join(arch, operating_system))
          for arch in ['arm64', 'ppc64le', 's390x']:
              os.mkdir(arch)
              for operating_system in ['linux']:
                  os.mkdir(os.path.join(arch, operating_system))

          for arch, operating_system, path in [
                  ('amd64', 'linux', '/usr/share/openshift/linux_amd64/oc'),
                  ('amd64', 'mac', '/usr/share/openshift/mac/oc'),
                  ('amd64', 'windows', '/usr/share/openshift/windows/oc.exe'),
                  ('arm64', 'linux', '/usr/share/openshift/linux_arm64/oc'),
                  ('ppc64le', 'linux', '/usr/share/openshift/linux_ppc64le/oc'),
                  ('s390x', 'linux', '/usr/share/openshift/linux_s390x/oc'),
                  ]:
              basename = os.path.basename(path)
              target_path = os.path.join(arch, operating_system, basename)
              os.symlink(path, target_path)
              base_root, _ = os.path.splitext(basename)
              archive_path_root = os.path.join(arch, operating_system, base_root)
              with tarfile.open('{}.tar'.format(archive_path_root), 'w') as tar:
                  tar.add(path, basename)
              with zipfile.ZipFile('{}.zip'.format(archive_path_root), 'w') as zip:
                  zip.write(path, basename)

          # Create socket
          addr = ('', 8080)
          sock = socket.socket(socket.` + addressFamily + `, socket.SOCK_STREAM)
          sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
          sock.bind(addr)
          sock.listen(5)

          [Thread(i, socket=sock) for i in range(100)]
          time.sleep(9e9)
          EOF
          exec python2 /tmp/serve.py  # the cli image only has Python 2.7`
}
