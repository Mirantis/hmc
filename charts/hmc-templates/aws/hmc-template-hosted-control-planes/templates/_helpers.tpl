{{- define "cluster.name" -}}
    {{- .Values.clusterName | trunc 63 }}
{{- end }}

{{- define "awsmachinetemplate.name" -}}
    {{- include "cluster.name" . }}-mt
{{- end }}

{{- define "k0smotroncontrolplane.name" -}}
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