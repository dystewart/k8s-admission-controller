oc apply -f mutatingwebhookconfiguration.yaml
oc annotate validatingwebhookconfigurations <validating_webhook_name> service.beta.openshift.io/inject-cabundle=true
oc apply -f service.yaml