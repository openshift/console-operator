# Use AWS ALB as alternative ingress on ROSA HCP

This doc aims at showing the effort needed to expose the OpenShift console via AWS ALB on a ROSA HCP cluster.
The use case in mind is [HyperShift hosted clusters where the Ingress capability is disabled](https://github.com/openshift/enhancements/pull/1415).

## Requirements

- ROSA HCP OpenShift cluster.
- [AWS Load Balancer Operator installed and its controller created](https://docs.openshift.com/rosa/networking/aws-load-balancer-operator.html).
- User logged as a cluster admin.

## Procedure

### Create certificate in AWS Certificate Manager

In order to configure an HTTPS listener on AWS ALB you need to have a certificate created in AWS Certificate Manager.
You can import an existing certificate or request a new one. Make sure the certificate is created in the same region as your cluster.
Note the certificate ARN and the DNS name used in the certificate, you will need it later.

### Create Ingress resources for the NodePort services

To provision ALBs create the following resources:
```bash
cat <<EOF | oc apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: instance
    alb.ingress.kubernetes.io/backend-protocol: HTTPS
    alb.ingress.kubernetes.io/certificate-arn: ${CERTIFICATE_ARN}
  name: console
  namespace: openshift-console
spec:
  ingressClassName: alb
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: console
                port:
                  number: 443
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: instance
    alb.ingress.kubernetes.io/backend-protocol: HTTP
    alb.ingress.kubernetes.io/certificate-arn: ${CERTIFICATE_ARN}
  name: downloads
  namespace: openshift-console
spec:
  ingressClassName: alb
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: downloads
                port:
                  number: 80
EOF
```

### Update console config

Once the console ALBs are ready you need to let the console operator know which urls to use.

#### Add custom trusted CA (optional)

To add the CA of the certificates used in the ingress objects to [the trusted bundle of the OpenShift cluster](https://docs.openshift.com/container-platform/latest/networking/configuring-a-custom-pki.html#nw-proxy-configure-object_configuring-a-custom-pki), follow these steps:
```bash
$ oc -n openshift-config create configmap console-ca-bundle --from-file=ca-bundle.crt=/path/to/pemencoded/cacert
$ oc patch proxy cluster --type=merge -p '{"spec":{"trustedCA":{"name":"console-ca-bundle"}}}'
```

#### Setup DNS (optional)

The console ALBs have public DNS names that might not match the Subject Alternative Name (SAN) from the certificates. Ensure public DNS records matching the certificates' SANs are created and target the following hostnames:
```bash
$ oc -n openshift-console get ing console -o yaml | yq .status.loadBalancer.ingress[0].hostname
k8s-openshif-console-xxxxxxxxxx-xxxxxxxx.us-east-2.elb.amazonaws.comdd
$ oc -n openshift-console get ing downloads -o yaml | yq .status.loadBalancer.ingress[0].hostname
k8s-openshif-download-xxxxxxxxxx-xxxxxxxxxx.us-east-2.elb.amazonaws.com
```

#### Update console operator config

Update the console operator config providing the custom urls:
```bash
$ oc patch console.operator.openshift.io cluster --type=merge -p "{\"spec\":{\"ingress\":{\"consoleURL\":\"https://${CONSOLE_HOST}\",\"clientDownloadsURL\":\"https://${DOWNLOADS_HOST}\"}}}"
```
**Note**: ensure that the hosts used in the urls match the SAN from the corresponding certificates.

## Notes

1. ROSA HCP does not have the authentication operator, the authentication server is managed centrally by the HyperShift layer:
```bash
$ oc -n openshift-authentication-operator get deploy,route
No resources found

$ oc -n openshift-authentication get pods,routes
No resources found

$ oc get oauthclient | grep -v console
NAME                           SECRET                                        WWW-CHALLENGE   TOKEN-MAX-AGE   REDIRECT URIS
openshift-browser-client                                                     false           default         https://oauth.mytestcluster.5199.s3.devshift.org:443/oauth/token/display
openshift-challenging-client                                                 true            default         https://oauth.mytestcluster.5199.s3.devshift.org:443/oauth/token/implicit

$ oc -n openshift-console rsh deploy/console curl -k https://openshift.default.svc/.well-known/oauth-authorization-server
{
"issuer": "https://oauth.mytestcluster.5199.s3.devshift.org:443",
"authorization_endpoint": "https://oauth.mytestcluster.5199.s3.devshift.org:443/oauth/authorize",
"token_endpoint": "https://oauth.mytestcluster.5199.s3.devshift.org:443/oauth/token",
```

2. When the ingress capability is disabled, the console operator relies on the end user to provide the console and download URLs (using the operator API) for health checks and oauthclient.

3. When the ingress capability is disabled, the console operator skips the implementation of the component route customization.

4. To simulate the absence of ingress connectivity when the ingress capability is disabled, set the desired replicas to zero in the default ingress controller:
```bash
$ oc -n openshift-ingress-operator patch ingresscontroller default --type='json' -p='[{"op": "replace", "path": "/spec/replicas", "value":0}]'
```
