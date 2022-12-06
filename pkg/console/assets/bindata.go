// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
// bindata/configmaps/console-configmap.yaml
// bindata/configmaps/console-public-configmap.yaml
// bindata/deployments/console-deployment.yaml
// bindata/deployments/downloads-deployment.yaml
// bindata/managedclusteractions/console-create-oauth-client.yaml
// bindata/managedclusterviews/console-oauth-client.yaml
// bindata/managedclusterviews/console-oauth-server-cert.yaml
// bindata/managedclusterviews/olm-config.yaml
// bindata/pdb/console-pdb.yaml
// bindata/pdb/downloads-pdb.yaml
// bindata/routes/console-custom-route.yaml
// bindata/routes/console-redirect-route.yaml
// bindata/routes/console-route.yaml
// bindata/routes/downloads-custom-route.yaml
// bindata/routes/downloads-route.yaml
// bindata/services/console-redirect-service.yaml
// bindata/services/console-service.yaml
// bindata/services/downloads-service.yaml
package assets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _configmapsConsoleConfigmapYaml = []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  namespace: openshift-console
  labels:
    app: "console"
  annotations: {}
`)

func configmapsConsoleConfigmapYamlBytes() ([]byte, error) {
	return _configmapsConsoleConfigmapYaml, nil
}

func configmapsConsoleConfigmapYaml() (*asset, error) {
	bytes, err := configmapsConsoleConfigmapYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "configmaps/console-configmap.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _configmapsConsolePublicConfigmapYaml = []byte(`# This configmap 'console-public' manifest is used to expose the console URL
# to all authenticated users
apiVersion: v1
kind: ConfigMap
metadata:
  name: console-public
  namespace: openshift-config-managed
`)

func configmapsConsolePublicConfigmapYamlBytes() ([]byte, error) {
	return _configmapsConsolePublicConfigmapYaml, nil
}

func configmapsConsolePublicConfigmapYaml() (*asset, error) {
	bytes, err := configmapsConsolePublicConfigmapYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "configmaps/console-public-configmap.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _deploymentsConsoleDeploymentYaml = []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: console
  namespace: openshift-console
  labels:
    app: console
    component: ui
spec:
  selector:
    matchLabels:
      app: console
      component: ui
  strategy:
    type: RollingUpdate
  template:
    metadata:
      name: console
      labels:
        app: console
        component: ui
      annotations:
        target.workload.openshift.io/management: '{"effect": "PreferredDuringScheduling"}'
    spec:
      nodeSelector:
        node-role.kubernetes.io/master: ""
      restartPolicy: Always
      serviceAccountName: console
      schedulerName: default-scheduler
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      terminationGracePeriodSeconds: 40
      priorityClassName: system-cluster-critical
      containers:
        - resources:
            requests:
              cpu: 10m
              memory: 100Mi
          readinessProbe:
            httpGet:
              path: /health
              port: 8443
              scheme: HTTPS
            timeoutSeconds: 1
            periodSeconds: 10
            successThreshold: 1
            failureThreshold: 3
          lifecycle:
            preStop:
              exec:
                command:
                  - sleep
                  - "25"
          name: console
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
              - ALL
          command:
            - /opt/bridge/bin/bridge
            - "--public-dir=/opt/bridge/static"
            - "--config=/var/console-config/console-config.yaml"
            - "--service-ca-file=/var/service-ca/service-ca.crt"
          livenessProbe:
            httpGet:
              path: /health
              port: 8443
              scheme: HTTPS
            initialDelaySeconds: 150
            timeoutSeconds: 1
            periodSeconds: 10
            successThreshold: 1
            failureThreshold: 3
          ports:
            - name: https
              containerPort: 8443
              protocol: TCP
          imagePullPolicy: IfNotPresent
          terminationMessagePolicy: FallbackToLogsOnError
          image: ${IMAGE}
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule
        - key: node.kubernetes.io/unreachable
          operator: Exists
          effect: NoExecute
          tolerationSeconds: 120
        - key: node.kubernetes.io/not-reachable
          operator: Exists
          effect: NoExecute
          tolerationSeconds: 120
`)

func deploymentsConsoleDeploymentYamlBytes() ([]byte, error) {
	return _deploymentsConsoleDeploymentYaml, nil
}

func deploymentsConsoleDeploymentYaml() (*asset, error) {
	bytes, err := deploymentsConsoleDeploymentYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "deployments/console-deployment.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _deploymentsDownloadsDeploymentYaml = []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: downloads
  namespace: openshift-console
  labels:
    app: console
    component: downloads
  annotations: {}
spec:
  selector:
    matchLabels:
      app: console
      component: downloads
  strategy:
    type: RollingUpdate
  template:
    metadata:
      name: downloads
      labels:
        app: console
        component: downloads
      annotations:
        target.workload.openshift.io/management: '{"effect": "PreferredDuringScheduling"}'
    spec:
      nodeSelector:
        kubernetes.io/os: linux
      terminationGracePeriodSeconds: 0
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - resources:
            requests:
              cpu: 10m
              memory: 50Mi
          readinessProbe:
            httpGet:
              path: /
              port: 8080
              scheme: HTTP
            timeoutSeconds: 1
            periodSeconds: 10
            successThreshold: 1
            failureThreshold: 3
          name: download-server
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
              - ALL
          command:
            - /bin/sh
          livenessProbe:
            httpGet:
              path: /
              port: 8080
              scheme: HTTP
            timeoutSeconds: 1
            periodSeconds: 10
            successThreshold: 1
            failureThreshold: 3
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          imagePullPolicy: IfNotPresent
          terminationMessagePolicy: FallbackToLogsOnError
          image: ${IMAGE}
          args:
            - '-c'
            - |
              cat <<EOF >>/tmp/serve.py
              import errno, http.server, os, re, signal, socket, sys, tarfile, tempfile, threading, time, zipfile

              signal.signal(signal.SIGTERM, lambda signum, frame: sys.exit(0))

              def write_index(path, message):
                with open(path, 'wb') as f:
                  f.write('\n'.join([
                    '<!doctype html>',
                    '<html lang="en">',
                    '<head>',
                    '  <meta charset="utf-8">',
                    '</head>',
                    '<body>',
                    '  {}'.format(message),
                    '</body>',
                    '</html>',
                    '',
                  ]).encode('utf-8'))

              # Launch multiple listeners as threads
              class Thread(threading.Thread):
                def __init__(self, i, socket):
                  threading.Thread.__init__(self)
                  self.i = i
                  self.socket = socket
                  self.daemon = True
                  self.start()

                def run(self):
                  server = http.server.SimpleHTTPRequestHandler
                  server.server_version = "OpenShift Downloads Server"
                  server.sys_version = ""
                  httpd = http.server.HTTPServer(addr, server, False)

                  # Prevent the HTTP server from re-binding every handler.
                  # https://stackoverflow.com/questions/46210672/
                  httpd.socket = self.socket
                  httpd.server_bind = self.server_close = lambda self: None

                  httpd.serve_forever()

              temp_dir = tempfile.mkdtemp()
              print('serving from {}'.format(temp_dir))
              os.chdir(temp_dir)
              for arch in ['amd64', 'arm64', 'ppc64le', 's390x']:
                os.mkdir(arch)
              content = ['<a href="oc-license">license</a>']
              os.symlink('/usr/share/openshift/LICENSE', 'oc-license')

              for arch, operating_system, path in [
                  ('amd64', 'linux', '/usr/share/openshift/linux_amd64/oc'),
                  ('amd64', 'mac', '/usr/share/openshift/mac/oc'),
                  ('amd64', 'windows', '/usr/share/openshift/windows/oc.exe'),
                  ('arm64', 'linux', '/usr/share/openshift/linux_arm64/oc'),
                  ('arm64', 'mac', '/usr/share/openshift/mac_arm64/oc'),
                  ('ppc64le', 'linux', '/usr/share/openshift/linux_ppc64le/oc'),
                  ('s390x', 'linux', '/usr/share/openshift/linux_s390x/oc'),
                  ]:
                basename = os.path.basename(path)
                target_path = os.path.join(arch, operating_system, basename)
                os.mkdir(os.path.join(arch, operating_system))
                os.symlink(path, target_path)
                base_root, _ = os.path.splitext(basename)
                archive_path_root = os.path.join(arch, operating_system, base_root)
                with tarfile.open('{}.tar'.format(archive_path_root), 'w') as tar:
                  tar.add(path, basename)
                with zipfile.ZipFile('{}.zip'.format(archive_path_root), 'w') as zip:
                  zip.write(path, basename)
                content.append('<a href="{0}">oc ({1} {2})</a> (<a href="{0}.tar">tar</a> <a href="{0}.zip">zip</a>)'.format(target_path, arch, operating_system))

              for root, directories, filenames in os.walk(temp_dir):
                root_link = os.path.relpath(temp_dir, os.path.join(root, 'child')).replace(os.path.sep, '/')
                for directory in directories:
                  write_index(
                    path=os.path.join(root, directory, 'index.html'),
                    message='<p>Directory listings are disabled.  See <a href="{}">here</a> for available content.</p>'.format(root_link),
                  )

              write_index(
                path=os.path.join(temp_dir, 'index.html'),
                message='\n'.join(
                  ['<ul>'] +
                  ['  <li>{}</li>'.format(entry) for entry in content] +
                  ['</ul>']
                ),
              )

              # Create socket
              # IPv6 should handle IPv4 passively so long as it is not bound to a
              # specific address or set to IPv6_ONLY
              # https://stackoverflow.com/questions/25817848/python-3-does-http-server-support-ipv6
              try:
                addr = ('::', 8080)
                sock = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
              except socket.error as err:
                # errno.EAFNOSUPPORT is "socket.error: [Errno 97] Address family not supported by protocol"
                # When IPv6 is disabled, socket will bind using IPv4.
                if err.errno == errno.EAFNOSUPPORT:
                  addr = ('', 8080)
                  sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                else:
                  raise    
              sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
              sock.bind(addr)
              sock.listen(5)

              [Thread(i, socket=sock) for i in range(100)]
              time.sleep(9e9)
              EOF
              exec python3 /tmp/serve.py
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule
        - key: node.kubernetes.io/unreachable
          operator: Exists
          effect: NoExecute
          tolerationSeconds: 120
        - key: node.kubernetes.io/not-reachable
          operator: Exists
          effect: NoExecute
          tolerationSeconds: 120
      priorityClassName: system-cluster-critical
`)

func deploymentsDownloadsDeploymentYamlBytes() ([]byte, error) {
	return _deploymentsDownloadsDeploymentYaml, nil
}

func deploymentsDownloadsDeploymentYaml() (*asset, error) {
	bytes, err := deploymentsDownloadsDeploymentYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "deployments/downloads-deployment.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _managedclusteractionsConsoleCreateOauthClientYaml = []byte(`apiVersion: action.open-cluster-management.io/v1beta1
kind: ManagedClusterAction
spec:
  actionType: Create
  kube:
    resource: OAuthClient
    template:
      apiVersion: oauth.openshift.io/v1
      kind: OAuthClient
      grantMethod: auto
`)

func managedclusteractionsConsoleCreateOauthClientYamlBytes() ([]byte, error) {
	return _managedclusteractionsConsoleCreateOauthClientYaml, nil
}

func managedclusteractionsConsoleCreateOauthClientYaml() (*asset, error) {
	bytes, err := managedclusteractionsConsoleCreateOauthClientYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "managedclusteractions/console-create-oauth-client.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _managedclusterviewsConsoleOauthClientYaml = []byte(`apiVersion: view.open-cluster-management.io/v1beta1
kind: ManagedClusterView
spec:
  scope:
    apiVersion: oauth.openshift.io/v1
    resource: OAuthClient
    name: console-managed-cluster-oauth-client
`)

func managedclusterviewsConsoleOauthClientYamlBytes() ([]byte, error) {
	return _managedclusterviewsConsoleOauthClientYaml, nil
}

func managedclusterviewsConsoleOauthClientYaml() (*asset, error) {
	bytes, err := managedclusterviewsConsoleOauthClientYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "managedclusterviews/console-oauth-client.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _managedclusterviewsConsoleOauthServerCertYaml = []byte(`apiVersion: view.open-cluster-management.io/v1beta1
kind: ManagedClusterView
spec:
  scope:
    kind: ConfigMap
    version: v1
    name: default-ingress-cert
    namespace: openshift-config-managed
`)

func managedclusterviewsConsoleOauthServerCertYamlBytes() ([]byte, error) {
	return _managedclusterviewsConsoleOauthServerCertYaml, nil
}

func managedclusterviewsConsoleOauthServerCertYaml() (*asset, error) {
	bytes, err := managedclusterviewsConsoleOauthServerCertYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "managedclusterviews/console-oauth-server-cert.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _managedclusterviewsOlmConfigYaml = []byte(`apiVersion: view.open-cluster-management.io/v1beta1
kind: ManagedClusterView
spec:
  scope:
    apiVersion: operators.coreos.com/v1
    resource: OLMConfig
    name: cluster
`)

func managedclusterviewsOlmConfigYamlBytes() ([]byte, error) {
	return _managedclusterviewsOlmConfigYaml, nil
}

func managedclusterviewsOlmConfigYaml() (*asset, error) {
	bytes, err := managedclusterviewsOlmConfigYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "managedclusterviews/olm-config.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pdbConsolePdbYaml = []byte(`apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: console
  namespace: openshift-console
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: console
      component: ui`)

func pdbConsolePdbYamlBytes() ([]byte, error) {
	return _pdbConsolePdbYaml, nil
}

func pdbConsolePdbYaml() (*asset, error) {
	bytes, err := pdbConsolePdbYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pdb/console-pdb.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pdbDownloadsPdbYaml = []byte(`apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: downloads
  namespace: openshift-console
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: console
      component: downloads`)

func pdbDownloadsPdbYamlBytes() ([]byte, error) {
	return _pdbDownloadsPdbYaml, nil
}

func pdbDownloadsPdbYaml() (*asset, error) {
	bytes, err := pdbDownloadsPdbYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pdb/downloads-pdb.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _routesConsoleCustomRouteYaml = []byte(`# This route 'console-custom' manifest is used in case a custom console route is set
# either on the ingress config or console-operator config.
# The 'console-custom' route will be pointing to the 'console' service. 
# Only a single custom console route is supported.
kind: Route
apiVersion: route.openshift.io/v1
metadata:
  name: console-custom
  namespace: openshift-console
  labels:
    app: console
spec:
  to:
    kind: Service
    name: console
    weight: 100
  port:
    targetPort: https
  tls:
    termination: reencrypt
    insecureEdgeTerminationPolicy: Redirect
  wildcardPolicy: None
`)

func routesConsoleCustomRouteYamlBytes() ([]byte, error) {
	return _routesConsoleCustomRouteYaml, nil
}

func routesConsoleCustomRouteYaml() (*asset, error) {
	bytes, err := routesConsoleCustomRouteYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "routes/console-custom-route.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _routesConsoleRedirectRouteYaml = []byte(`# This 'console' route manifest is used in case a custom console route is set
# either on the ingress config or console-operator config.
# The 'console' route will be used for redirect to the 'console-custom' route
# by the console backend.
# Only a single custom console route is supported.
kind: Route
apiVersion: route.openshift.io/v1
metadata:
  name: console
  namespace: openshift-console
  labels:
    app: console
spec:
  to:
    kind: Service
    name: console-redirect
    weight: 100
  port:
    targetPort: custom-route-redirect
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  wildcardPolicy: None
`)

func routesConsoleRedirectRouteYamlBytes() ([]byte, error) {
	return _routesConsoleRedirectRouteYaml, nil
}

func routesConsoleRedirectRouteYaml() (*asset, error) {
	bytes, err := routesConsoleRedirectRouteYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "routes/console-redirect-route.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _routesConsoleRouteYaml = []byte(`# Default 'console' route manifest.
# The 'console' route will be pointing to the 'console' service. 
kind: Route
apiVersion: route.openshift.io/v1
metadata:
  name: console
  namespace: openshift-console
  labels:
    app: console
spec:
  to:
    kind: Service
    name: console
    weight: 100
  port:
    targetPort: https
  tls:
    termination: reencrypt
    insecureEdgeTerminationPolicy: Redirect
  wildcardPolicy: None
`)

func routesConsoleRouteYamlBytes() ([]byte, error) {
	return _routesConsoleRouteYaml, nil
}

func routesConsoleRouteYaml() (*asset, error) {
	bytes, err := routesConsoleRouteYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "routes/console-route.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _routesDownloadsCustomRouteYaml = []byte(`# This route 'downloads-custom' manifest is used in case a custom downloads route is set
# on the ingress config.
# The 'downloads-custom' route will be pointing to the 'downloads' service.
# Only a single custom downloads route is supported.
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  namespace: openshift-console
  name: downloads-custom
spec:
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  port:
    targetPort: http
  to:
    kind: Service
    name: downloads
  wildcardPolicy: None
`)

func routesDownloadsCustomRouteYamlBytes() ([]byte, error) {
	return _routesDownloadsCustomRouteYaml, nil
}

func routesDownloadsCustomRouteYaml() (*asset, error) {
	bytes, err := routesDownloadsCustomRouteYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "routes/downloads-custom-route.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _routesDownloadsRouteYaml = []byte(`# Default 'downloads' route manifest.
# The 'downloads' route will be pointing to the 'downloads' service. 
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  namespace: openshift-console
  name: downloads
spec:
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  port:
    targetPort: http
  to:
    kind: Service
    name: downloads
    weight: 100
  wildcardPolicy: None
`)

func routesDownloadsRouteYamlBytes() ([]byte, error) {
	return _routesDownloadsRouteYaml, nil
}

func routesDownloadsRouteYaml() (*asset, error) {
	bytes, err := routesDownloadsRouteYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "routes/downloads-route.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _servicesConsoleRedirectServiceYaml = []byte(`# This 'console-redirect' service manifest is used in case a custom downloads route is set
# either on the ingress config or console-operator config.
# Service will forward the request to the 'console' deployment's backend under 8444 port,
# which backend will redirect to the custom route.
# Only a single custom downloads route is supported.
kind: Service
apiVersion: v1
metadata:
  name: console-redirect
  namespace: openshift-console
  labels:
    app: console
  annotations:
    service.beta.openshift.io/serving-cert-secret-name: console-serving-cert
spec:
  ports:
    - name: custom-route-redirect
      protocol: TCP
      port: 8444
      targetPort: 8444
  selector:
    app: console
    component: ui
  type: ClusterIP
  sessionAffinity: None
`)

func servicesConsoleRedirectServiceYamlBytes() ([]byte, error) {
	return _servicesConsoleRedirectServiceYaml, nil
}

func servicesConsoleRedirectServiceYaml() (*asset, error) {
	bytes, err := servicesConsoleRedirectServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "services/console-redirect-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _servicesConsoleServiceYaml = []byte(`# Default 'console' service manifest.
# The 'console' service will be pointing to the 'console' deployment. 
apiVersion: v1
kind: Service
metadata:
  name: console
  namespace: openshift-console
  labels:
    app: console
  annotations:
    service.beta.openshift.io/serving-cert-secret-name: console-serving-cert
spec:
  ports:
    - name: https
      protocol: TCP
      port: 443
      targetPort: 8443
  selector:
    app: console
    component: ui
  type: ClusterIP
  sessionAffinity: None
`)

func servicesConsoleServiceYamlBytes() ([]byte, error) {
	return _servicesConsoleServiceYaml, nil
}

func servicesConsoleServiceYaml() (*asset, error) {
	bytes, err := servicesConsoleServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "services/console-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _servicesDownloadsServiceYaml = []byte(`# Default 'downloads' service manifest.
# The 'downloads' route will be pointing to the 'downloads' deployment. 
apiVersion: v1
kind: Service
metadata:
  namespace: openshift-console
  name: downloads
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    app: console
    component: downloads
  type: ClusterIP
  sessionAffinity: None
`)

func servicesDownloadsServiceYamlBytes() ([]byte, error) {
	return _servicesDownloadsServiceYaml, nil
}

func servicesDownloadsServiceYaml() (*asset, error) {
	bytes, err := servicesDownloadsServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "services/downloads-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"configmaps/console-configmap.yaml":                      configmapsConsoleConfigmapYaml,
	"configmaps/console-public-configmap.yaml":               configmapsConsolePublicConfigmapYaml,
	"deployments/console-deployment.yaml":                    deploymentsConsoleDeploymentYaml,
	"deployments/downloads-deployment.yaml":                  deploymentsDownloadsDeploymentYaml,
	"managedclusteractions/console-create-oauth-client.yaml": managedclusteractionsConsoleCreateOauthClientYaml,
	"managedclusterviews/console-oauth-client.yaml":          managedclusterviewsConsoleOauthClientYaml,
	"managedclusterviews/console-oauth-server-cert.yaml":     managedclusterviewsConsoleOauthServerCertYaml,
	"managedclusterviews/olm-config.yaml":                    managedclusterviewsOlmConfigYaml,
	"pdb/console-pdb.yaml":                                   pdbConsolePdbYaml,
	"pdb/downloads-pdb.yaml":                                 pdbDownloadsPdbYaml,
	"routes/console-custom-route.yaml":                       routesConsoleCustomRouteYaml,
	"routes/console-redirect-route.yaml":                     routesConsoleRedirectRouteYaml,
	"routes/console-route.yaml":                              routesConsoleRouteYaml,
	"routes/downloads-custom-route.yaml":                     routesDownloadsCustomRouteYaml,
	"routes/downloads-route.yaml":                            routesDownloadsRouteYaml,
	"services/console-redirect-service.yaml":                 servicesConsoleRedirectServiceYaml,
	"services/console-service.yaml":                          servicesConsoleServiceYaml,
	"services/downloads-service.yaml":                        servicesDownloadsServiceYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"configmaps": {nil, map[string]*bintree{
		"console-configmap.yaml":        {configmapsConsoleConfigmapYaml, map[string]*bintree{}},
		"console-public-configmap.yaml": {configmapsConsolePublicConfigmapYaml, map[string]*bintree{}},
	}},
	"deployments": {nil, map[string]*bintree{
		"console-deployment.yaml":   {deploymentsConsoleDeploymentYaml, map[string]*bintree{}},
		"downloads-deployment.yaml": {deploymentsDownloadsDeploymentYaml, map[string]*bintree{}},
	}},
	"managedclusteractions": {nil, map[string]*bintree{
		"console-create-oauth-client.yaml": {managedclusteractionsConsoleCreateOauthClientYaml, map[string]*bintree{}},
	}},
	"managedclusterviews": {nil, map[string]*bintree{
		"console-oauth-client.yaml":      {managedclusterviewsConsoleOauthClientYaml, map[string]*bintree{}},
		"console-oauth-server-cert.yaml": {managedclusterviewsConsoleOauthServerCertYaml, map[string]*bintree{}},
		"olm-config.yaml":                {managedclusterviewsOlmConfigYaml, map[string]*bintree{}},
	}},
	"pdb": {nil, map[string]*bintree{
		"console-pdb.yaml":   {pdbConsolePdbYaml, map[string]*bintree{}},
		"downloads-pdb.yaml": {pdbDownloadsPdbYaml, map[string]*bintree{}},
	}},
	"routes": {nil, map[string]*bintree{
		"console-custom-route.yaml":   {routesConsoleCustomRouteYaml, map[string]*bintree{}},
		"console-redirect-route.yaml": {routesConsoleRedirectRouteYaml, map[string]*bintree{}},
		"console-route.yaml":          {routesConsoleRouteYaml, map[string]*bintree{}},
		"downloads-custom-route.yaml": {routesDownloadsCustomRouteYaml, map[string]*bintree{}},
		"downloads-route.yaml":        {routesDownloadsRouteYaml, map[string]*bintree{}},
	}},
	"services": {nil, map[string]*bintree{
		"console-redirect-service.yaml": {servicesConsoleRedirectServiceYaml, map[string]*bintree{}},
		"console-service.yaml":          {servicesConsoleServiceYaml, map[string]*bintree{}},
		"downloads-service.yaml":        {servicesDownloadsServiceYaml, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
