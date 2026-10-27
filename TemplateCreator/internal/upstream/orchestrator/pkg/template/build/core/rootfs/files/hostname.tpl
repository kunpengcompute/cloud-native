{{- /*gotype:gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/core/rootfs.templateModel*/ -}}
{{ .WriteFile "/etc/hostname" 0o644 }}

{{ .Hostname }}