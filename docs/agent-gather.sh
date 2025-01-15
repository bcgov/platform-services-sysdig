#!/usr/bin/env bash

# agent-gather.sh - This script helps the Sysdig team get initial data of Kubernetes and Openshift clusters

# Usage: ./agent-gather.sh --destination <path>
# The script has the following dependencies:
# kubectl installed and able to connect to the required cluster with Admin privileges.
# metrics server installed on the cluster. This is not a hard requirement, but is better to get usage information
# jq installed for parsing json output from the k8s api server
# Please report faiures to deiver.kielretana@sysdig.com or danial.taracks@sysdig.com

version=1.3

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0;37m'
BOLD='\033[1m'
NOBOLD='\033[0m'
LC_NUMERIC=en_US.UTF-8

date_time=$(date +"%d-%m-%Y--%H-%M")
log_file=$(mktemp /tmp/sysdig-agent-gather-log.XXXXXX)
top_pods=$(mktemp /tmp/sysdig-agent-gather-pods_top.XXXXXX)
temp_file=$(mktemp /tmp/sysdig-agent-gather-temp.XXXXXX)
top_nodes=$(mktemp /tmp/sysdig-agent-gather-nodes_top.XXXXXX)
descriptions=$(mktemp /tmp/sysdig-agent-gather-nodes_description.XXXXXX)
cluster_yaml=$(mktemp /tmp/sysdig-agent-gather-cluster_yaml.XXXXXX)
metadata_file=$(mktemp /tmp/sysdig-agent-gather-metadata.XXXXXX)
cluster_summary=$(mktemp /tmp/sysdig-agent-gather-cluster_summary.XXXXXX)
user_cluster_proxy=""
user_cluster_proxy_port=""
use_cluster_name=""
platform_provider=""
input_user_provider=""
input_user_cluster_type=""
input_user_cluster_name=""
destination_directory=""
default_install_method="Platform customer"
tag_keys=()
tag_values=()
registry_list=()
install_methods=("Secure only" "Monitor only" "Platform customer")
required_permissions=("describe nodes" "top nodes" "top pods" "get nodes" "get projects" "config current-context" "get deployments" "version")
oldest_yq_version=4.18.1


usage() {
    echo "Usage: $0 -d /path [OPTIONS]"
    echo "Required:"
    echo "  -d, --destination DIRECTORY  Specify the destination directory."
    echo ""
    echo "Options:"
    echo "  -h, --help                  Show this help message and exit."
    exit 1
}

echo_status() {
    local status=$1
    [[ ${status} == 0 ]] && echo -en "${GREEN}DONE: ${NC}"
    [[ ${status} == 1 ]] && echo -en "${RED}FAIL: ${NC}"
    [[ ${status} == 2 ]] && { printf '=%.0s' $(seq 1 80) >> ${metadata_file}; echo >> ${metadata_file}; }
    [[ ${status} == 3 ]] && echo -en "${YELLOW}SKIPPED: ${NC}"
    [[ ${status} == 4 ]] && { printf '=%.0s' $(seq 1 80) >> ${cluster_summary}; echo >> ${cluster_summary}; }
    [[ ${status} == 5 ]] && { printf '-' >> ${metadata_file}; printf '=%.0s' $(seq 1 79) >> ${metadata_file}; echo >> ${metadata_file}; }
}

catch_logs() {
    local log_type=$1
    local message=$2
    local timestamp=$(date +"%d-%m-%Y %H:%M:%S")
    echo "${timestamp} - ${log_type}: ${message}" >> ${log_file}
}

print_header() {
    local message="$1"
    local cls=$2
    local len=${#message}
    local min_width=80
    [[ $((len + 4)) > min_width ]] || min_width=$(( len + 4 ))
    printf '=%.0s' $(seq 1 $min_width)
    echo
    printf "= ${GREEN}%-${len}s = ${NC}\n" "$message"
    printf '=%.0s' $(seq 1 $min_width)
    echo
}

add_entry() {
    keys+=("$1")
    values+=("$2")
}

print_sysdig() {
    echo ' ______   ______  ____ ___ ____'
    echo '/ ___\ \ / / ___||  _ \_ _/ ___|'
    echo '\___ \\ V /\___ \| | | | | |  _ '
    echo ' ___) || |  ___) | |_| | | |_| |'
    echo '|____/ |_| |____/|____/___\____|'
    echo "Agent gather version ${version}"
}

progress_bar() {
    local number_of_task=$1
    local number_of_total_tasks=$2
    local progress=$((100 * ${number_of_task} / ${number_of_total_tasks}))
    local bar_length=$((progress / 2))
    local spaces=$((50 - bar_length))

    printf "["
    for ((i_bar = 0; i_bar < bar_length; i_bar++)); do
        printf "="
    done

    for ((i_spaces = 0; i_spaces < spaces; i_spaces++)); do
        printf " "
    done
    printf "] %d%%\r" "$progress"
}

cleanup() {
    if [ -n "${log_file}" ] && [ -f "${log_file}" ]
    then
         rm ${log_file}
    fi
    if [ -n "${metadata_file}" ] && [ -f "${metadata_file}" ]
    then
         rm ${metadata_file}
    fi
    if [ -n "${descriptions}" ] && [ -f "${descriptions}" ]
    then
         rm ${descriptions}
    fi
    if [ -n "${top_nodes}" ] && [ -f "${top_nodes}" ]
    then
         rm ${top_nodes}
    fi
    if [ -n "${top_pods}" ] && [ -f "${top_pods}" ]
    then
         rm ${top_pods}
    fi
    if [ -n "${cluster_summary}" ] && [ -f "${cluster_summary}" ]
    then
         rm ${cluster_summary}
    fi
    if [ -n "${cluster_yaml}" ] && [ -f "${cluster_yaml}" ]
    then
        rm ${cluster_yaml}         
    fi
}

validate_kubectl() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if ! command -v kubectl &>/dev/null; then
        echo_status 1
        echo "kubectl is not installed. Please install kubectl according your kubernetes version"
        catch_logs "ERROR" "kubectl is not installed"
        exit 1
    fi
}

validate_role_permissions(){
    catch_logs "RUN" "${FUNCNAME[0]}"
    for action_resource in "${required_permissions[@]}"
    do
        IFS=' ' read -r verb resource <<< "$action_resource"
        validation_result=$(kubectl auth can-i "${verb}" "${resource}" 2>&1 | tail -1)
        if [[ "${validation_result}" == "yes" ]]
        then
            echo_status 0
            echo "Permission for kubectl ${action_resource} validated"
            catch_logs "INFO" "Permission ${action_resource} granted"
        else
            echo_status 1
            echo "Permission for kubectl ${action_resource} is not granted"
            catch_logs "INFO" "Permission ${action_resource} is not granted"
            case "${action_resource}" in 
                "describe nodes")
                    echo_status 1
                    echo "Fatal fail"
                    echo "\"${action_resource}\" permission is required. If this permission is not granted, then, script can not continue."
                    catch_logs "ERROR" "Fatal error with required permissions \"${action_resource}\""
                    exit 1
                esac
        fi
    done
}

compare_yq_versions() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local ver_required=$(echo ${oldest_yq_version} | tr -d '.')
    local ver_system=$(yq -V | sed -n 's/.*version v\([0-9.]*\).*/\1/p' | tr -d '.')
    if [[ $ver_required -gt $ver_system ]]
    then
        echo_status 1
        echo "System's yq version is older than required"
    elif [[ $ver_required -le $ver_system ]]
    then
        echo_status 0
        echo "System's yq version is good for the script's requirements"
    else
        echo_status 1
        echo "yq versions cannot be compared properly"
    fi
}

remove_previous_files() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local file_name=$1
    if [[ -f ${file_name} ]]
    then
        rm ${file_name}
        catch_logs "INFO" "Removing previous file: ${file_name}"
    fi
}

autodetect_platform_provider() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local node_labels=$(kubectl get nodes -o jsonpath='{.items[0].metadata.labels}')
    case "${node_labels}" in
        *eks.amazonaws.com*)
            local platform_provider="aws"
            ;;
        *instance-type*)
            local platform_provider="aws"
            ;;
        *agentpool*)
            local platform_provider="azure"
            ;;
        *cloud.google.com/gke-nodepool*)
            local platform_provider="gcp"
            ;;
        *)
            local platform_provider="unknown"
            ;;
    esac
    echo "${platform_provider}"
}

autodetect_cluster_type() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local current_context=$(kubectl config current-context)
    if [[ -z "${current_context}" ]]
    then
        echo_status 1
        echo -e "${RED}Error: Failed to get the current context.${NC}"
        echo -e "${RED}Check your kubefile.${NC}"
        catch_logs "ERROR" "Failed to get the current context"
        return 1
    fi
    if kubectl get projects &> /dev/null; then
        current_context="openshift"
    fi
    case "${current_context}" in
        *".eks."*)
            local autodetected_cluster_type="eks"
            ;;
        *".gke."*)
            local autodetected_cluster_type="gke"
            ;;
        *".aks."*)
            local autodetected_cluster_type="aks"
            ;;
        *".oke."*)
            local autodetected_cluster_type="oke"
            ;;
        *".kops."*)
            local autodetected_cluster_type="kops"
            ;;
        "openshift")
            local autodetected_cluster_type="openshift"
            ;;
        *)
            local autodetected_cluster_type="unknown"
            ;;
    esac
    echo "${autodetected_cluster_type}"
}

autodetect_cluster_name() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local current_context=$(kubectl config current-context)
    local autodetected_cluster_type=$(autodetect_cluster_type)
 
    if [[ ${autodetected_cluster_type} == "openshift" ]]
    then
        local autodetect_cluster_name=$(echo ${current_context} | cut -d '/' -f2 | awk -F':' '{print $1}')
    else
        local autodetect_cluster_name="${current_context##*/}"
    fi

    if [[ -z "${autodetect_cluster_name}" ]]
    then
        echo_status 1
        echo "${RED}Error: Failed to get the cluster name for the current context.${NC}"
        echo "${RED}Please check that you are connected to the correct kubernetes cluster${NC}"
        catch_logs "ERROR" "Failed to get the cluster name from the current context ${current_context}"
        return 1
    fi
    echo ${autodetect_cluster_name}
}

confirm_information() {
    local platform_providers=("Azure" "AWS" "GCP" "OCI" "OnPrem" "Other")
    local cluster_types=("OpenShift" "Kubernetes")
    local autodetect_platform_provider=$(autodetect_platform_provider)
    local autodetect_cluster_type=$(autodetect_cluster_type)
    local autodetect_cluster_name=$(autodetect_cluster_name)
    local default_option=""
    local options_len=${#install_methods[@]}
    local user_choice_install_method=""
    local user_default_install_method=""

    while [ "${default_option}" != "Y" ] && [ "${default_option}" != "y" ]
    do
        print_header "Step 1: Validate autodetected information" "clear"
        print_header "Confirm the platform provider:"
        shopt -s nocasematch
        for provider in "${!platform_providers[@]}"
        do
            if [[ "${autodetect_platform_provider}" == "${platform_providers[$provider]}" ]]
            then
                default_option=$((provider+1))
            fi
            echo "$((provider+1)). ${platform_providers[$provider]}"
        done

        while true 
        do
            read -p "Enter the number for your choice [${default_option}]: " provider_choice
            provider_choice=${provider_choice:-$default_option}
            if [[ $provider_choice -ge 1 && $provider_choice -le ${#platform_providers[@]} ]]
            then
                input_user_provider=${platform_providers[$((provider_choice-1))]}
                break
            else
                echo "Invalid choice. Please enter a number between 1 and ${#platform_providers[@]}."
            fi 
        done
        print_header "Step 2: Validate autodetected information" "clear"
        print_header "Choose a cluster type:"
        for cluster_type in "${!cluster_types[@]}"
        do
            if [[ "${autodetect_cluster_type}" == "${cluster_types[$cluster_type]}" ]]
            then
                default_option=$((cluster_type+1))
            fi
            echo "$((cluster_type+1)). ${cluster_types[$cluster_type]}"
        done
        while true
        do
            
            read -p "Enter the number for your choice [${default_option}]: " cluster_type_choice
            cluster_type_choice=${cluster_type_choice:-$default_option}
            if [[ $cluster_type_choice -ge 1 && $cluster_type_choice -le ${#cluster_types[@]} ]]
            then
                input_user_cluster_type=${cluster_types[$((cluster_type_choice-1))]}
                break
            else
                echo "Invalid choice. Please enter a number between 1 and ${#cluster_types[@]}."
            fi
        done
        shopt -u nocasematch

        print_header "Step 3: Validate autodetected information" "clear"
        read -p "Enter the cluster name [${autodetect_cluster_name}]: " input_user_cluster_name
        input_user_cluster_name=${input_user_cluster_name:-$autodetect_cluster_name}

        print_header "Step 4: Installation Methods" "clear"
        print_header "Choose a installation method:"
        for ((option=0; option<${options_len}; option++))
        do
            if [[ "${install_methods[$option]}" == ${default_install_method} ]]
            then
                echo "$((option + 1)). ${install_methods[$option]} (Default)"
                user_default_install_method=$((option + 1))
            else
                echo "$((option + 1)). ${install_methods[$option]}"
            fi
        done
        read -p "Enter your required installation method (1-${options_len}): " user_choice_install_method
        user_choice_install_method=${user_choice_install_method:-$user_default_install_method}

        print_header "Step 5: Select which registry would be used." "clear"
        print_header "The following registries can be used as images repository"
        echo "Registry from which you want to pull agent components images: "
        local index=1
        local registry
        local registry_array_size=${#registry_list[@]}
        echo "0: quay.io/ (Default)"
        for registry in "${registry_list[@]}"
        do
            echo "${index}: ${registry}"
            index=$((index + 1))
        done
        read -p 'Please select a registry from above list(eg., 2): ' user_reg_num
        if [[ "${user_reg_num}" -ge 1 &&  "${user_reg_num}" -le $((registry_array_size + 1)) ]]
        then
            user_reg_num=${user_reg_num:-1}
            user_selected_registry="${registry_list[$((user_reg_num - 1))]}"
        else
            user_selected_registry="quay.io/"
        fi

        print_header "Step 6: Proxy to be used." "clear"
        print_header "Is proxy required for this cluster?"
        read -p 'Please enter Yes/No (Default: no): ' user_proxy_required
        if [[ ${user_proxy_required} == "Yes" ||  ${user_proxy_required} == "yes" ]]
        then
            user_proxy_required=true
            read -p 'Please enter the proxy hostname: ' user_cluster_proxy
            read -p 'Please enter the proxy port: ' user_cluster_proxy_port
        else
            user_proxy_required=false
        fi
        print_header "Step 7: Set the cluster's environment." "clear"
        read -p "Enter the environment for this cluster. Example: preprod, nonprod, prod. (Default: nonprod): " user_cluster_environment
        user_cluster_environment=${user_cluster_environment:-nonprod}

        default_option="y"
        print_header "This is the information that will be used" "clear"
        echo -e "Platform Provider: ${BOLD}${input_user_provider}${NOBOLD}"
        echo -e "Cluster Type: ${BOLD}${input_user_cluster_type}${NOBOLD}"
        echo -e "Cluster Name: ${BOLD}${input_user_cluster_name}${NOBOLD}"
        echo -e "Installation method in this cluster: ${BOLD}${install_methods[$((user_choice_install_method - 1))]}${NOBOLD}"
        echo -e "Registry to use in this cluster: ${BOLD}${user_selected_registry}${NOBOLD}"
        echo -e "Proxy Required: ${BOLD}${user_proxy_required}${NOBOLD}"
        [ -n "${user_cluster_proxy}" ] && echo -e "Proxy hostname to use in this cluster: ${BOLD}${user_cluster_proxy}${NOBOLD}"
        [ -n "${user_cluster_proxy_port}" ] && echo -e "Proxy port to use in this cluster: ${BOLD}${user_cluster_proxy_port}${NOBOLD}"
        echo -e "Cluster environment: ${BOLD}${user_cluster_environment}${NOBOLD}"
        read -p "Please confirm you want to continue (Y) or not (n) to edit the information. Cancel (c): ? [Y/n/c]: " default_option
        default_option=${default_option:-y}
        if [[ "${default_option}" == "c" ]]
        then
            cleanup
            exit 10
        elif [[ "${default_option}" == "n" ]]
        then
            unset input_user_provider input_user_cluster_type input_user_cluster_name user_selected_registry user_proxy_required user_cluster_proxy user_cluster_proxy_port user_cluster_environment
        fi
    done
    clear
    print_sysdig
    echo_status 0
    echo "Initial information was validated"
    echo_status 2
    echo "CLUSTER INFORMATION SUMMARY:" >> ${metadata_file}
    echo "Platform Provider: ${input_user_provider}" >> ${metadata_file}
    echo "Autodetected Platform Provider: ${autodetect_platform_provider}" >> ${metadata_file}
    echo "Cluster Type: ${input_user_cluster_type}" >> ${metadata_file}
    echo "Autodetected Cluster Type: ${autodetect_cluster_type}" >> ${metadata_file}
    echo "Cluster Name: ${input_user_cluster_name}" >> ${metadata_file}
    echo "Autodetected Cluster Name: ${autodetect_cluster_name}" >> ${metadata_file}
    echo "Installation method in this cluster: ${install_methods[$((user_choice_install_method - 1))]}" >> ${metadata_file}
    echo "Registry to use in this cluster: ${user_selected_registry}" >> ${metadata_file}
    echo "Proxy Required: ${user_proxy_required}" >> ${metadata_file}
    [ -n "${user_cluster_proxy}" ] && echo "Proxy hostname to use in this cluster: ${user_cluster_proxy}" >> ${metadata_file}
    [ -n "${user_cluster_proxy_port}" ] && echo "Proxy port to use in this cluster: ${user_cluster_proxy_port}" >> ${metadata_file}
    echo "Cluster environment: ${user_cluster_environment}" >> ${metadata_file}
}

test_kubectl_top() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local top_nodes=$(kubectl top nodes 2>&1)
    if [[ $? -ne 0 ]]
    then
        echo_status 1
        echo -e "${RED}Error with 'kubectl top nodes': $top_nodes${NC}"
        catch_logs "ERROR" "Metrics seems to not be working, kubectl top pods did not work correctly"
        return 1
    else
        echo_status 0
        echo "kubectl top nodes is working."
        catch_logs "INFO" "Metrics was detected correctly"
    fi

    local top_pods=$(kubectl top pods --all-namespaces 2>&1)
    if [[ $? -ne 0 ]]
    then
        echo_status 1
        echo -e "${RED}Error with 'kubectl top pods': $top_pods${NC}"
        catch_logs "ERROR" "Metrics seems to not be working, kubectl top pods did not work correctly"
        return 1
    else
        echo_status 0
        echo "kubectl top pods is working."
    fi
}

verify_metrics_server() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if [[ "${input_user_cluster_type}" == "Kubernetes" ]]
    then
        local metrics_server=$(kubectl get deployment -n kube-system | grep metrics-server)
    else
        local metrics_server=$(kubectl top nodes)
    fi
    if [[ -z "${metrics_server}" ]]
    then
        echo_status 1
        echo -e "${RED}Error: Metrics Server is not running in the kube-system namespace.${NC}"
        echo "Please visit Metrics server project page: https://github.com/kubernetes-sigs/metrics-server"
        catch_logs "ERROR" "Metrics server is not installed in the cluster"
    else
        echo_status 0
        echo "Metrics Server was validated."
    fi
}

get_cluster_version() {
    catch_logs "RUN" "$FUNCNAME"
    echo_status 2
    echo "CLUSTER AND CLIENT VERSIONS:" >> ${metadata_file}
    kubectl version --output=yaml 2>/dev/null >> ${metadata_file}
    if [[ $? -ne 0 ]]
    then
        catch_logs "ERROR" "Command \"kubectl version --output=yaml\" could not run correctly"
        echo_status 1
        echo "Command \"kubectl version --output=yaml\" did not run correctly. Check this command manually"
    fi
    catch_logs "INFO" "Cluster version for ${input_user_cluster_name} was generated at: ${date_time}"
    echo_status 0
    echo "Getting cluster version"
}

get_images_list() {
    catch_logs "RUN" "$FUNCNAME"
    echo_status 2
    echo "IMAGES INFORMATION:" >> ${metadata_file}
    kubectl get nodes -o json | jq -r '.items[].status.images[] | "\(.sizeBytes / 1000000) \(.names[])"' | while read size name; do printf "%.2fMB %s\n" "$size" "$name"; done | sort -nr | head -1 >> ${metadata_file}
    kubectl get nodes -o json | jq -r '.items[].status.images[] | "\(.sizeBytes / 1000000) \(.names[])"' | while read size name; do printf "%.2fMB %s\n" "$size" "$name"; done | sort -nr | tail -1 >> ${metadata_file}
    if [[ $? -ne 0 ]]
    then
        catch_logs "ERROR" "Command \"kubectl get nodes -o json | jq -r '.items[].status.images[] | ...\" could not run correctly to get the images list"
        echo_status 1
        echo "Command \"kubectl get nodes -o json | jq -r '.items[].status.images[] | ...\" did not run correctly. Check this command manually"
    fi
    catch_logs "INFO" "Images list for ${input_user_cluster_name} was generated at: ${date_time}"
    echo_status 0
    echo "Getting images list"
}

get_metadata() {
    catch_logs "RUN" "$FUNCNAME"
    get_cluster_version
    get_images_list
}

nodes_hardware_configuration_details() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "NODES HARDWARE CONFIGURATION:" >> ${metadata_file}
    if [[ $(autodetect_platform_provider) == "aws" ]]
    then
        local data=$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.labels.node\.kubernetes\.io/instance-type}{"\t"}{.status.capacity.cpu}{" CPUs\t"}{.status.capacity.memory}{" Memory\n"}{end}')
    fi
    echo "${data}"
}

nodes_os_details() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "NODES OPERATIVE SYSTEM DETAILS:" >> ${metadata_file}
    echo -e "Node Name\t\tOS Image\t\tKernel Version\t\tContainer Runtime Version\t\tKubelet Version\t\tKube-Proxy Version" >> ${metadata_file}
    local data=$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{","}{.status.nodeInfo.osImage}{","}{.status.nodeInfo.kernelVersion}{","}{.status.nodeInfo.containerRuntimeVersion}{","}{.status.nodeInfo.kubeletVersion}{","}{.status.nodeInfo.kubeProxyVersion}{"\n"}{end}')
    echo "${data}"
}

nodes_taints() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "NODES TAINTS DETAILS:" >> ${metadata_file}
    local data=$(kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}:{'\n'}{range .spec.taints[*]}{.key}={.value}:{.effect}{'\n'}{end}{'\n'}{end}")
    echo "${data}"
}

pods_running_by_node() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "PODS RUNNING BY NODE:" >> ${metadata_file}
    echo -e "Node Name\t\t\tNumber of Pods Running" >> ${metadata_file}
    for node in $(kubectl get nodes --no-headers --output=custom-columns=NAME:.metadata.name)
    do
        pod_count=$(kubectl describe node $node | grep "Non-terminated Pods:" | awk -F'[()]' '{print $2}' | awk '{print $1}')
        echo -e "$node\t$pod_count" >> ${metadata_file}
    done
}

nodes_metadata() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    list_size=$(nodes_hardware_configuration_details | wc -l)
    echo_status 2
    echo "AMOUNT OF NODES: ${list_size}" >> ${metadata_file}
    nodes_hardware_configuration_details >> ${metadata_file}
    nodes_os_details >> ${metadata_file}
    nodes_taints >> ${metadata_file}
    pods_running_by_node
}

cluster_describe_nodes() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local nodes_hostname=$(kubectl get nodes --no-headers --output=custom-columns=NAME:.metadata.name)
    local list_size=$(echo "${nodes_hostname}" | wc -l)
    echo "This cluster has ${list_size} nodes"
    local counter=0
    IFS=$'\n'
    for node in ${nodes_hostname}
    do
        ((counter++))
        kubectl describe node ${node} >> ${descriptions}
        if [[ $? -ne 0 ]]
        then
            catch_logs "ERROR" "Command \"kubectl describe node ${node}\" did not run correctly"
            echo_status 1
            echo "I could not get description for node: ${node}"
        else
            catch_logs "INFO" "Description for node ${node} was successfully saved"
            echo "Description for node ${node} was successfully saved"
        fi
    done
    unset IFS
    catch_logs "INFO" "File: ${input_user_cluster_name}_nodes_description.txt was generated at: ${date_time}"
    echo_status 0
    echo "Getting nodes description"
}

cluster_network_policies() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "CLUSTER NETWORK POLICIES:" >> ${metadata_file}
    local data=$(kubectl get networkpolicies --all-namespaces 2>&1)
    echo "${data}"
}

cluster_priority_classes() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "CLUSTER PRIORITY CLASSES:" >> ${metadata_file}
    local data=$(kubectl get priorityclass)
    echo "${data}" >> ${metadata_file}
}

cluster_metadata() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    cluster_network_policies >> ${metadata_file}
    cluster_priority_classes >> ${metadata_file}
}

cluster_get_top_nodes() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    kubectl top nodes --no-headers >> ${top_nodes}
    if [[ $? -ne 0 ]]
    then
        catch_logs "ERROR" "Command \"kubectl top nodes --no-headers\" did not run correctly"
        echo_status 1
        echo "Command \"kubectl top nodes --no-headers\" did not run correctly. Check this command manually"
    fi
    catch_logs "INFO" "File: ${input_user_cluster_name}_nodes_top.txt was generated at: ${date_time}"
    echo_status 0
    echo "Getting top nodes"
}

cluster_get_top_pods() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    kubectl top pods -A --no-headers >> ${top_pods}
    if [[ $? -ne 0 ]]
    then
        catch_logs "ERROR" "Command \"kubectl top pods -A --no-headers\" did not run correctly"
        echo_status 1
        echo "Command \"kubectl top pods -A --no-headers\" did not run correctly. Check this command manually"
    fi
    catch_logs "INFO" "File: "${input_user_cluster_name}_pods_top.txt" was generated at: ${date_time}"
    echo_status 0
    echo "Getting top pods"
}

get_host_type() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local node_data="$1"
    local host_type=$(echo "$node_data" | grep "kubernetes.io/instance-type=" | sed 's/.*=//' | head -1)
    if [[ -z "${host_type}" ]]
    then
        local host_type="baremetal"
    fi
    echo ${host_type}
}

process_node() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    local node_data="$1"
    local host_name=$(echo "${node_data}" | grep "Name:" | sed -e 's/Name:[[:space:]]*//')
    local host_type=$(get_host_type "${node_data}")
    local host_cpu=$(echo "${node_data}" | grep -A 50 "Capacity:" | sed -n '/Allocatable:/q;p' | grep "cpu:" | sed -e 's/cpu:[[:space:]]*//')
    local host_memory=$(($(echo "${node_data}" | grep -A 50 "Capacity:" | sed -n '/Allocatable:/q;p' | grep "memory:" | sed -e 's/memory://;s/Ki$//;s/^[[:space:]]*//;s/[[:space:]]*$//') / 1048576))
    local host_allocatable_ephemeral_Storage=$(echo "${node_data}" | grep -A 50 "Allocatable:"  | grep "ephemeral-storage:" | awk '{print $2}' | sort -n | head -1)

    if [[ -s "${top_nodes}" ]]
    then
        local host_used_memory_percent=$(cat ${top_nodes} | grep "${host_name}" | awk '{print $5}' | sed 's/%//g')
        local host_used_cpu_percent=$(cat ${top_nodes} | grep "${host_name}" | awk '{print $3}' | sed 's/%//g')
        if [[ ${host_used_memory_percent} =~ ^(0|[1-9]?[0-9]|100)$ ]]
        then
            local available_memory=$((${host_memory} - (${host_memory} * ${host_used_memory_percent%\%} / 100)))
        else
            available_memory="N/D"
        fi
    else
        local host_used_memory_percent="N/D"
        local host_used_cpu_percent="N/D"
        local available_memory="N/D"
    fi
    printf "%-50s\t%-16s\t%-16s\t%-16s\t%-15s\t%-16s\t%-16s\t%-16s\n" "${host_name}" "${host_type}" "${host_cpu}" "${host_used_cpu_percent}" "${host_memory}" "${host_used_memory_percent}" "${available_memory}" "${host_allocatable_ephemeral_Storage}">> ${temp_file}
}

get_node_data() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    number_of_nodes=$(cat "${descriptions}" | grep "Name:" | wc -l)
    echo "Nodes to process ${number_of_nodes}"
    file_lines=$(cat "${descriptions}" | wc -l)
    count=0
    for ((i = 1; i <= ${file_lines}; i++))
    do
        line=$(sed -n "${i}p" "${descriptions}") 
        if [[ $line == "Name:"* ]]
        then
            in_node_section=true
        fi

        if [ "$in_node_section" == true ]; then
            current_node+="$line"$'\n'
        fi
        if [[ $line == "Events:"* ]]
        then
            in_node_section=false
        fi
        if [[ -z "$line" ]]
        then
            end_reached=true
        else
            end_reached=false
        fi

        if [[ "$in_node_section" == false ]]
        then
            process_node "$current_node"
            ((count++))
            progress_bar $count ${number_of_nodes}
            current_node=""
            in_node_section=true
        fi
    done
    printf "%-50s\t%-16s\t%-16s\t%-16s\t%-15s\t%-16s\t%-16s\t%-16s\n" "HOSTNAME" "INSTANCE TYPE" "HOST CPU" "USED CPU %" "HOST MEMORY" "USED MEMORY %" "AVAILABLE MEMORY" "EPHEMERAL STORAGE" > ${cluster_summary}
    cat ${temp_file} | sort -k7,7n -r >> ${cluster_summary}
    rm ${temp_file}
    echo -e "\nProcessing completed!"
}

get_nodes_cpu_usage_statistics() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if [ -e "${top_nodes}" ] && [ $(wc -l < "${top_nodes}") -gt 0 ]
    then
        echo_status 4
        echo "NODES CPU % USAGE STATS:" >> ${cluster_summary}
        local cpu_stats=$(cat ${top_nodes} | awk '{print $3}'  | tr -d '%' | sort -n)
        local average_cpu_usage=$(echo "${cpu_stats}" | awk '{sum+=$1} END {if (NR > 0) print sum/NR}')
        local median_cpu_usage=$(echo "${cpu_stats}" | awk '{a[NR]=$1} END {if (NR%2) {print a[(NR+1)/2]} else {print (a[(NR/2)] + a[(NR/2)+1])/2}}')
        local max_cpu_usage=$(echo "${cpu_stats}" | tail -1)
        printf "%-16s\t%-16s\t%-16s\n" "MAX USAGE" "AVERAGE USAGE" "MEDIAN USAGE" >> ${cluster_summary}
        printf "%-16s\t%-16s\t%-16s\n" "${max_cpu_usage}%" "${average_cpu_usage}%" "${median_cpu_usage}%" >> ${cluster_summary}
    else
        echo_status 4
        echo "NODES CPU % USAGE STATS NOT AVAILABLE" >> ${cluster_summary}
    fi
}

get_nodes_ram_usage_statistics() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if [ -e "${top_nodes}" ] && [ $(wc -l < "${top_nodes}") -gt 0 ]
    then
        echo_status 4
        echo "NODES RAM % USAGE STATS:" >> ${cluster_summary}
        local ram_stats=$(cat ${top_nodes} | awk '{print $5}'  | tr -d '%' | sort -n)
        local average_ram_usage=$(echo "${ram_stats}" | awk '{sum+=$1} END {if (NR > 0) print sum/NR}')
        local median_ram_usage=$(echo "${ram_stats}" | awk '{a[NR]=$1} END {if (NR%2) {print a[(NR+1)/2]} else {print (a[(NR/2)] + a[(NR/2)+1])/2}}')
        local max_ram_usage=$(echo "${ram_stats}" | tail -1)
        printf "%-16s\t%-16s\t%-16s\n" "MAX USAGE" "AVERAGE USAGE" "MEDIAN USAGE" >> ${cluster_summary}
        printf "%-16s\t%-16s\t%-16s\n" "${max_ram_usage}%" "${average_ram_usage}%" "${median_ram_usage}%" >> ${cluster_summary}
    else
        echo_status 4
        echo "NODES RAM % USAGE STATS NOT AVAILABLE" >> ${cluster_summary}
    fi
}

get_pods_cpu_usage_statistics() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if [ -e "${top_pods}" ] && [ $(wc -l < "${top_pods}") -gt 0 ]
    then
        echo_status 4
        echo "SYSDIG PODS CPU MILLICORES USAGE STATS:" >> ${cluster_summary}
        local cpu_stats=$(cat ${top_pods} | grep -i sysdig | awk '{print $3}'  | tr -d 'm' | sort -n)
        local average_cpu_usage=$(echo "${cpu_stats}" | awk '{sum+=$1} END {if (NR > 0) print sum/NR}')
        local median_cpu_usage=$(echo "${cpu_stats}" | awk '{a[NR]=$1} END {if (NR%2) {print a[(NR+1)/2]} else {print (a[(NR/2)] + a[(NR/2)+1])/2}}')
        local max_cpu_usage=$(echo "${cpu_stats}" | tail -1)
        printf "%-16s\t%-16s\t%-16s\n" "MAX USAGE" "AVERAGE USAGE" "MEDIAN USAGE" >> ${cluster_summary}
        printf "%-16s\t%-16s\t%-16s\n" "${max_cpu_usage}m" "${average_cpu_usage}m" "${median_cpu_usage}m" >> ${cluster_summary}
    else
        echo_status 4
        echo "SYSDIG PODS CPU MILLICORES USAGE STATS NOT AVAILABLE" >> ${cluster_summary}
    fi
}

get_pods_ram_usage_statistics() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if [ -e "${top_pods}" ] && [ $(wc -l < "${top_pods}") -gt 0 ]
    then
        echo_status 4
        echo "SYSDIG PODS MEBIBYTES USAGE STATS:" >> ${cluster_summary}
        local ram_stats=$(cat ${top_pods} | grep -i sysdig | awk '{print $4}'  | tr -d 'Mi' | sort -n)
        local average_ram_usage=$(echo "${ram_stats}" | awk '{sum+=$1} END {if (NR > 0) print sum/NR}')
        local median_ram_usage=$(echo "${ram_stats}" | awk '{a[NR]=$1} END {if (NR%2) {print a[(NR+1)/2]} else {print (a[(NR/2)] + a[(NR/2)+1])/2}}')
        local max_ram_usage=$(echo "${ram_stats}" | tail -1)
        printf "%-16s\t%-16s\t%-16s\n" "MAX USAGE" "AVERAGE USAGE" "MEDIAN USAGE" >> ${cluster_summary}
        printf "%-16s\t%-16s\t%-16s\n" "${max_ram_usage}Mi" "${average_ram_usage}Mi" "${median_ram_usage}Mi" >> ${cluster_summary}
    else
        echo_status 4
        echo "SYSDIG PODS MEBIBYTES USAGE STATS NOT AVAILABLE" >> ${cluster_summary}
    fi
}

show_usage_statistics() {
    get_nodes_cpu_usage_statistics
    get_nodes_ram_usage_statistics
    get_pods_cpu_usage_statistics
    get_pods_ram_usage_statistics
}

list_accesible_registers() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "ACCESSIBLE REGISTRIES:" >> ${metadata_file}
    kubectl get nodes -o json | jq -r '.items[].status.images[] | .names[]' | sort | uniq | awk -F'/' '{print $1"/"}' | uniq >> ${metadata_file}
    IFS=' ' read -ra registry_list <<< $(kubectl get nodes -o json | jq -r '.items[].status.images[] | .names[]' | sort | uniq | awk -F'/' '{print $1"/"}' | uniq | tr '\n' ' ')
}

check_sysdig_ds_dp() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "SYSDIG DAEMONSETS DETAILS:" >> ${metadata_file}
    kubectl get daemonsets --all-namespaces | grep -i "namespace\|sysdig" >> ${metadata_file}
    echo_status 2
    echo "SYSDIG DEPLOYMENTS DETAILS:" >> ${metadata_file}
    kubectl get deployments --all-namespaces | grep -i "namespace\|sysdig" >> ${metadata_file}
}

sysdig_pods_pending_messages() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "SYSDIG PENDING PODS MESSAGES:" >> ${metadata_file}
    pending_pods=$(kubectl get pods -n sysdig-agent --field-selector=status.phase=Pending -o jsonpath='{.items[*].metadata.name}')
    for pod in ${pending_pods}
    do
        message=$(kubectl describe pod ${pod} -n sysdig-agent | grep default-scheduler | sed -n 's/.*\(default-scheduler.*\)/\1/p')
        echo -e "POD: ${pod}: \nMessages:\n${message}" >> ${metadata_file}
        echo "-------------" >> ${metadata_file}
    done
}

get_sysdig_ds_configuration() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    local ds_info=$(kubectl get daemonsets --all-namespaces --no-headers | grep -i sysdig | awk '{print $1" "$2}')
    if [[ -n "${ds_info}" ]]
    then
        echo "SYSDIG DAEMONSETS YAML CONFIGURATIONS:" >> ${metadata_file}
        local counter=0
        while IFS= read -r line
        do
            counter=$((counter + 1))
            local data=$(kubectl get daemonset -n ${line} -o yaml)
            echo "---------DAEMONSET---------: ${counter}" >> ${metadata_file}
            echo "${data}" >> ${metadata_file}
        done <<< "${ds_info}"
    fi
}

get_sysdig_dp_configuration() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    local dp_info=$(kubectl get deployment --all-namespaces --no-headers | grep -i sysdig | awk '{print $1" "$2}')
    if [[ -n "${dp_info}" ]]
    then
        echo "SYSDIG DEPLOYMENTS YAML CONFIGURATIONS:" >> ${metadata_file}
        local counter=0
        while IFS= read -r line
        do
            counter=$((counter + 1))
            local data=$(kubectl get deployment -n ${line} -o yaml)
            echo ""---------DEPLOYMENT"---------: ${counter}" >> ${metadata_file}
            echo "${data}" >> ${metadata_file}
        done <<< "${dp_info}"
    fi
}

get_sysdig_cm() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    echo_status 2
    echo "SYSDIG CONFIGMAPS:" >> ${metadata_file}
    kubectl get configmaps --all-namespaces | awk '{print $2}' | grep -i sysdig-agent >> ${metadata_file}
    echo "SYSDIG CONFIGMAPS YAML:" >> ${metadata_file}
    echo_status 5
    
}

generate_compress_package() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    mv "${descriptions}" "${destination_directory}/${input_user_cluster_name}_nodes_description.txt"
    mv "${top_nodes}" "${destination_directory}/${input_user_cluster_name}_nodes_top.txt"
    mv "${top_pods}" "${destination_directory}/${input_user_cluster_name}_pods_top.txt"
    mv "${log_file}" "${destination_directory}/${input_user_cluster_name}-${date_time}.log"
    mv "${metadata_file}" "${destination_directory}/${input_user_cluster_name}.metadata"
    mv "${cluster_summary}" "${destination_directory}/${input_user_cluster_name}.summary"
    if command -v zip &>/dev/null
    then
         zip -j ${destination_directory}/${input_user_cluster_name}-${date_time}.zip \
         "${destination_directory}/${input_user_cluster_name}_nodes_description.txt" \
         "${destination_directory}/${input_user_cluster_name}_nodes_top.txt" \
         "${destination_directory}/${input_user_cluster_name}_pods_top.txt" \
         "${destination_directory}/${input_user_cluster_name}-${date_time}.log" \
         "${destination_directory}/${input_user_cluster_name}.summary" \
         "${destination_directory}/${input_user_cluster_name}.metadata" > /dev/null 2>&1
        [[ $? -ne 0 ]] || local zip_success=true
    fi
    if command -v tar &>/dev/null && [[ -z ${zip_success} ]]
    then
        tar -czf ${destination_directory}/${input_user_cluster_name}-${date_time}.tgz \
        ${destination_directory}/${input_user_cluster_name}_nodes_description.txt \
        ${destination_directory}/${input_user_cluster_name}_nodes_top.txt \
        ${destination_directory}/${input_user_cluster_name}_pods_top.txt \
        ${destination_directory}/${input_user_cluster_name}-${date_time}.log \
        ${destination_directory}/${input_user_cluster_name}.summary \
        ${destination_directory}/${input_user_cluster_name}.metadata > /dev/null 2>&1
        [[ $? -ne 0 ]] || local tgz_success=true
    fi

    if [[ ${zip_success} != "true" && ${tgz_success} != "true" ]] 
    then
        echo_status 1
        catch_logs "ERROR" "System could not catch if the file: ${destination_directory}/${input_user_cluster_name}-${date_time}.tgz/zip was not created as expected. Please check logs"
        echo "Compress package was not created correctly. Check this step manually"
    fi
    if [[ ${zip_success} == "true" || ${tgz_success} == "true" ]] 
    then
        rm ${destination_directory}/${input_user_cluster_name}_nodes_description.txt
        rm ${destination_directory}/${input_user_cluster_name}_nodes_top.txt
        rm ${destination_directory}/${input_user_cluster_name}_pods_top.txt
        rm ${destination_directory}/${input_user_cluster_name}-${date_time}.log
        rm ${destination_directory}/${input_user_cluster_name}.summary
        rm ${destination_directory}/${input_user_cluster_name}.metadata
    fi
}

show_summary() {
    catch_logs "RUN" "${FUNCNAME[0]}"
    if [[ -f "${destination_directory}/${input_user_cluster_name}-${date_time}.zip" ]]
    then
        echo_status 0
        echo "New compressed file created: ${destination_directory}/${input_user_cluster_name}-${date_time}.zip"
        catch_logs "INFO" "File: ${destination_directory}/${input_user_cluster_name}-${date_time}.zip was generated at: ${date_time}"
    elif [[ -f "${destination_directory}/${input_user_cluster_name}-${date_time}.tgz" ]]
    then
        echo_status 0
        echo "New compressed file created: ${destination_directory}/${input_user_cluster_name}-${date_time}.tgz"
        catch_logs "INFO" "File: ${destination_directory}/${input_user_cluster_name}-${date_time}.tgz was generated at: ${date_time}"
    else
        echo_status 1
        echo "An unexpted behavior has occured, please check logs"
    fi
}

main() {
    catch_logs "START" "${FUNCNAME[0]}"
    echo "CREATED AT: ${date_time}" >> ${metadata_file}
    clear
    print_sysdig
    compare_yq_versions
    validate_kubectl
    validate_role_permissions
    list_accesible_registers
    confirm_information
    get_metadata
    verify_metrics_server
    cluster_describe_nodes
    nodes_metadata
    cluster_metadata
    cluster_get_top_nodes
    cluster_get_top_pods
    get_node_data
    show_usage_statistics
    check_sysdig_ds_dp
    sysdig_pods_pending_messages
    get_sysdig_ds_configuration
    get_sysdig_dp_configuration
    generate_compress_package
    show_summary
}

while [ "$#" -gt 0 ]
do
    case "$1" in
        --destination|-d)
            if [ "$#" -lt 2 ]
            then
                echo "Error: --destination option requires an argument."
                exit 1
            fi
            destination_directory="${2%/}"
            shift 2
            if [[ -d "${destination_directory}" ]]
            then
                if [[ ! -w "${destination_directory}" ]]
                then
                    echo "You do not have write permission on ${destination_directory}."
                    exit 1
                fi
            else
                echo "The directory does not exist."
                exit 1
            fi
            ;;
        --help|-h)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [ -z "${destination_directory}" ]
then
    echo "Error: --destination option is required."
    usage
fi

main
