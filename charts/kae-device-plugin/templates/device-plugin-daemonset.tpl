{{- if and .Values.admissionWebhook.enabled (not .Values.devicePlugin.enabled) }}
{{- fail "admissionWebhook.enabled requires devicePlugin.enabled=true" }}
{{- end }}
{{- if and .Values.admissionWebhook.enabled (not (or (eq .Values.admissionWebhook.cert.mode "manual") (eq .Values.admissionWebhook.cert.mode "certManager"))) }}
{{- fail "admissionWebhook.cert.mode must be manual or certManager" }}
{{- end }}
{{- if and .Values.admissionWebhook.enabled (eq .Values.admissionWebhook.cert.mode "manual") (empty .Values.admissionWebhook.cert.caBundle) }}
{{- fail "admissionWebhook.cert.caBundle is required in manual certificate mode" }}
{{- end }}
{{- if .Values.devicePlugin.enabled }}
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ .Values.devicePlugin.name }}
  labels:
{{ include "kae-device-plugin.devicePluginLabels" . | indent 4 }}
spec:
  selector:
    matchLabels:
      app: {{ .Values.devicePlugin.name }}
  template:
    metadata:
      labels:
{{ include "kae-device-plugin.devicePluginLabels" . | indent 8 }}
    spec:
      containers:
        - name: {{ .Values.devicePlugin.name }}
          image: {{ include "kae-device-plugin.image" . }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          command:
            - /usr/local/bin/kae-device-plugin
          args:
            - "-kernel-vf-drivers={{ join "," .Values.devicePlugin.kernelVfDrivers }}"
{{- if .Values.admissionWebhook.enabled }}
            - -enable-admission-webhook=true
            - "-webhook-listen-addr=:{{ .Values.admissionWebhook.port }}"
            - -webhook-tls-cert-file=/tls/tls.crt
            - -webhook-tls-key-file=/tls/tls.key
            - "-webhook-default-kae-resource={{ .Values.admissionWebhook.defaultKaeResource }}"
            - "-webhook-default-kae-count={{ .Values.admissionWebhook.defaultKaeCount }}"
            - "-webhook-target-container-index={{ .Values.admissionWebhook.targetContainerIndex }}"
            - "-webhook-inject-envs={{ .Values.admissionWebhook.injectEnvs }}"
            - "-webhook-excluded-namespaces={{ join "," .Values.admissionWebhook.excludedNamespaces }}"
{{- end }}
          env:
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
{{- if .Values.admissionWebhook.enabled }}
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          ports:
            - name: https
              containerPort: {{ .Values.admissionWebhook.port }}
              protocol: TCP
          readinessProbe:
            tcpSocket:
              port: https
{{- end }}
          resources:
{{ toYaml .Values.devicePlugin.resources | indent 12 }}
          volumeMounts:
            - name: pcidir
              mountPath: /sys/bus/pci
            - name: kubeletsockets
              mountPath: /var/lib/kubelet/device-plugins
{{- if .Values.admissionWebhook.enabled }}
            - name: tls
              mountPath: /tls
              readOnly: true
{{- end }}
      volumes:
        - name: pcidir
          hostPath:
            path: /sys/bus/pci
        - name: kubeletsockets
          hostPath:
            path: /var/lib/kubelet/device-plugins
{{- if .Values.admissionWebhook.enabled }}
        - name: tls
          secret:
            secretName: {{ .Values.admissionWebhook.tlsSecretName }} # pragma: allowlist secret
{{- end }}
{{- with .Values.devicePlugin.nodeSelector }}
      nodeSelector:
{{ toYaml . | indent 8 }}
{{- end }}
{{- with .Values.devicePlugin.tolerations }}
      tolerations:
{{ toYaml . | indent 8 }}
{{- end }}
{{- with .Values.devicePlugin.affinity }}
      affinity:
{{ toYaml . | indent 8 }}
{{- end }}
{{- end }}
