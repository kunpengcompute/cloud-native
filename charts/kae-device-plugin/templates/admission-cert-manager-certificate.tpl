{{- if and .Values.admissionWebhook.enabled (eq .Values.admissionWebhook.cert.mode "certManager") }}
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: {{ .Values.admissionWebhook.cert.certManager.issuerName }}
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ .Values.admissionWebhook.cert.certManager.caCertificateName }}
spec:
  isCA: true
  commonName: {{ .Values.admissionWebhook.cert.certManager.caCertificateName }}
  secretName: {{ .Values.admissionWebhook.cert.certManager.caCertificateName }} # pragma: allowlist secret
  duration: {{ .Values.admissionWebhook.cert.certManager.duration }}
  renewBefore: {{ .Values.admissionWebhook.cert.certManager.renewBefore }}
  issuerRef:
    name: {{ .Values.admissionWebhook.cert.certManager.issuerName }}
    kind: Issuer
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: {{ .Values.admissionWebhook.cert.certManager.caCertificateName }}
spec:
  ca:
    secretName: {{ .Values.admissionWebhook.cert.certManager.caCertificateName }} # pragma: allowlist secret
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ .Values.admissionWebhook.cert.certManager.servingCertificateName }}
spec:
  secretName: {{ .Values.admissionWebhook.tlsSecretName }} # pragma: allowlist secret
  duration: {{ .Values.admissionWebhook.cert.certManager.duration }}
  renewBefore: {{ .Values.admissionWebhook.cert.certManager.renewBefore }}
  dnsNames:
    - {{ .Values.admissionWebhook.name }}
    - {{ .Values.admissionWebhook.name }}.{{ .Release.Namespace }}
    - {{ .Values.admissionWebhook.name }}.{{ .Release.Namespace }}.svc
  issuerRef:
    name: {{ .Values.admissionWebhook.cert.certManager.caCertificateName }}
    kind: Issuer
{{- end }}
