{{- /*gotype:gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/core/rootfs.templateModel*/ -}}
{{ .WriteFile "etc/systemd/system/systemd-journald.service.d/override.conf" 0o644 }}
{{ .WriteFile "etc/systemd/system/systemd-networkd.service.d/override.conf" 0o644 }}

[Service]
WatchdogSec=0