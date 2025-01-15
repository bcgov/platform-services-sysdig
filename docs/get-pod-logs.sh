# get all logs from node analyzer:
for pod in $(oc get pods --selector=app.kubernetes.io/name=nodeanalyzer -o jsonpath='{.items[*].metadata.name}'); do
    oc logs $pod -c sysdig-host-scanner > zip/${pod}.sysdig-host-scanner.log
    oc logs $pod -c sysdig-kspm-analyzer > zip/${pod}.sysdig-kspm-analyzer.log
done

# get all logs from sysdig agent:
for pod in $(oc get pods --selector=app.kubernetes.io/name=agent -o jsonpath='{.items[*].metadata.name}'); do
    oc logs $pod > zip/${pod}.log
done

# get all logs from cluster shield:
for pod in $(oc get pods --selector=app.kubernetes.io/name=clustershield -o jsonpath='{.items[*].metadata.name}'); do
    oc logs $pod > zip/${pod}.log
done
