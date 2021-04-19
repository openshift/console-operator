// Code generated for package assets by go-bindata DO NOT EDIT. (@generated)
// sources:
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

var _routesConsoleCustomRouteYaml = []byte(`# This route 'console-custom' manifest is used in case a a custom console route is set
# either on the ingress config or console-operator config.
# The 'console-custom' route will be pointing to the 'console' service. 
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
  wildcardPolicy: None`)

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
  wildcardPolicy: None`)

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
  wildcardPolicy: None`)

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
  wildcardPolicy: None`)

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
kind: Service
apiVersion: v1
metadata:
  name: console-redirect
  namespace: openshift-console
  labels:
    app: console
  annotations:
    service.alpha.openshift.io/serving-cert-secret-name: console-serving-cert
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
    service.alpha.openshift.io/serving-cert-secret-name: console-serving-cert
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
  sessionAffinity: None`)

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
  sessionAffinity: None`)

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
	"routes/console-custom-route.yaml":       routesConsoleCustomRouteYaml,
	"routes/console-redirect-route.yaml":     routesConsoleRedirectRouteYaml,
	"routes/console-route.yaml":              routesConsoleRouteYaml,
	"routes/downloads-custom-route.yaml":     routesDownloadsCustomRouteYaml,
	"routes/downloads-route.yaml":            routesDownloadsRouteYaml,
	"services/console-redirect-service.yaml": servicesConsoleRedirectServiceYaml,
	"services/console-service.yaml":          servicesConsoleServiceYaml,
	"services/downloads-service.yaml":        servicesDownloadsServiceYaml,
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
