{{- define "cluster.name" -}}
    {{- .Values.clusterName | trunc 63 }}
{{- end }}

{{- define "awsmachinetemplate.controlplane.name" -}}
    {{- include "cluster.name" . }}-cp-mt
{{- end }}

{{- define "awsmachinetemplate.worker.name" -}}
    {{- include "cluster.name" . }}-worker-mt
{{- end }}

{{- define "k0scontrolplane.name" -}}
    {{- include "cluster.name" . }}-cp
{{- end }}

{{- define "k0sworkerconfigtemplate.name" -}}
    {{- include "cluster.name" . }}-machine-config
{{- end }}

{{- define "machinedeployment.name" -}}
    {{- include "cluster.name" . }}-md
{{- end }}

{{- define "k0sconfig.name" -}}
    {{- include "cluster.name" . }}-k0sconfig
{{- end }}