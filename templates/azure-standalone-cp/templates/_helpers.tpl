{{- define "cluster.name" -}}
    {{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "azuremachinetemplate.controlplane.name" -}}
    {{- include "cluster.name" . }}-cp-mt
{{- end }}

{{- define "azuremachinetemplate.worker.name" -}}
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

{{- define "azure.json.worker" -}}
{
  "cloud": "AzurePublicCloud",
  "tenantId": "{{ .Values.tenantID }}",
  "subscriptionId": "{{ .Values.subscriptionID }}",
  "aadClientId": "{{ .Values.clientID }}",
  "aadClientSecret": "{{ .Values.clientSecret }}",
  "resourceGroup": "{{ include "cluster.name" . }}",
  "securityGroupName": "{{ include "cluster.name" . }}-node-nsg",
  "securityGroupResourceGroup": "{{ include "cluster.name" . }}",
  "location": "{{ .Values.location }}",
  "vmType": "vmss",
  "vnetName": "{{ include "cluster.name" . }}-vnet",
  "vnetResourceGroup": "{{ include "cluster.name" . }}",
  "subnetName": "{{ include "cluster.name" . }}-node-subnet",
  "routeTableName": "{{ include "cluster.name" . }}-node-routetable",
  "loadBalancerSku": "Standard",
  "loadBalancerName": "",
  "maximumLoadBalancerRuleCount": 250,
  "useManagedIdentityExtension": false,
  "useInstanceMetadata": true
}
{{- end }}

{{- define "azure.json.controller" -}}
{
  "cloud": "AzurePublicCloud",
  "tenantId": "{{ .Values.tenantID }}",
  "subscriptionId": "{{ .Values.subscriptionID }}",
  "aadClientId": "{{ .Values.clientID }}",
  "aadClientSecret": "{{ .Values.clientSecret }}",
  "resourceGroup": "{{ include "cluster.name" . }}",
  "securityGroupName": "{{ include "cluster.name" . }}-controlplane-nsg",
  "securityGroupResourceGroup": "{{ include "cluster.name" . }}",
  "location": "{{ .Values.location }}",
  "vmType": "vmss",
  "vnetName": "{{ include "cluster.name" . }}-vnet",
  "vnetResourceGroup": "{{ include "cluster.name" . }}",
  "subnetName": "{{ include "cluster.name" . }}-controlplane-subnet",
  "routeTableName": "{{ include "cluster.name" . }}-controlplane-routetable",
  "loadBalancerSku": "Standard",
  "loadBalancerName": "",
  "maximumLoadBalancerRuleCount": 250,
  "useManagedIdentityExtension": false,
  "useInstanceMetadata": true
}
{{- end }}
