{{- define "cluster.name" -}}
    {{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "azuremachinetemplate.name" -}}
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

{{- define "azure.json" -}}
{
  "cloud": "AzurePublicCloud",
  "tenantId": "{{ .Values.tenantID }}",
  "subscriptionId": "{{ .Values.subscriptionID }}",
  "aadClientId": "{{ .Values.clientID }}",
  "aadClientSecret": "{{ .Values.clientSecret }}",
  "resourceGroup": "{{ .Values.resourceGroup }}",
  "securityGroupName": "{{ .Values.network.securityGroupName }}",
  "securityGroupResourceGroup": "{{ .Values.resourceGroup }}",
  "location": "{{ .Values.location }}",
  "vmType": "vmss",
  "vnetName": "{{ .Values.network.vnetName }}",
  "vnetResourceGroup": "{{ .Values.resourceGroup }}",
  "subnetName": "{{ .Values.network.nodeSubnetName }}",
  "routeTableName": "{{ .Values.routeTableName }}",
  "loadBalancerSku": "Standard",
  "loadBalancerName": "",
  "maximumLoadBalancerRuleCount": 250,
  "useManagedIdentityExtension": false,
  "useInstanceMetadata": true
}
{{- end }}
