# Node Maintenance

## Prerequisites
* oc (OpenShift Client) 

## Install
To install the Node Maintenance Operator, there are several methods available, including using the OpenShift web console and the OpenShift CLI (`oc`). Here are the detailed steps for each method:

### Using the OpenShift Web Console

1. **Log in to the OpenShift Web Console:**
   Ensure you are logged in as a user with `cluster-admin` privileges.

2. **Navigate to OperatorHub:**
   - Go to **Operators** → **OperatorHub**.
   - Search for "Node Maintenance Operator".

3. **Install the Operator:**
   - Click on the Node Maintenance Operator.
   - Click **Install**.
   - Keep the default selection for **Installation mode** and **namespace** to install the Operator in the `openshift-operators` namespace.
   - Click **Install**.

4. **Verify Installation:**
   - Navigate to **Operators** → **Installed Operators**.
   - Check that the Node Maintenance Operator is installed in the `openshift-operators` namespace and that its status is `Succeeded`.

### Using the OpenShift CLI (`oc`)

#### Installing in the `openshift-operators` Namespace

1. **Log in to the OpenShift Cluster:**
   ```sh
   $ oc login <cluster-url> --username=<username> --password=<password>
   ```

2. **Create a Subscription for the Node Maintenance Operator:**
   Create a YAML file named `node-maintenance-subscription.yaml` with the following content:
   ```yaml
   apiVersion: operators.coreos.com/v1alpha1
   kind: Subscription
   metadata:
     name: node-maintenance-operator
     namespace: openshift-operators
   spec:
     channel: stable
     name: node-maintenance-operator
     source: redhat-operators
     sourceNamespace: openshift-marketplace
   ```

3. **Apply the Subscription:**
   ```sh
   $ oc create -f node-maintenance-subscription.yaml
   ```

4. **Verify the Installation:**
   ```sh
   $ oc get csv -n openshift-operators
   ```
   Ensure the CSV resource is in the `Succeeded` phase:
   ```sh
   NAME                                      DISPLAY                     VERSION   REPLACES   PHASE
   node-maintenance-operator.v5.3.0          Node Maintenance Operator   5.3.0                Succeeded
   ```

5. **Verify the Operator Deployment:**
   ```sh
   $ oc get deploy -n openshift-operators
   ```
   Ensure the deployment is running:
   ```sh
   NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
   node-maintenance-operator-controller-manager   1/1     1            1           10d
   ```

#### Installing in a Custom Namespace

1. **Create a Namespace:**
   Create a YAML file named `node-maintenance-namespace.yaml`:
   ```yaml
   apiVersion: v1
   kind: Namespace
   metadata:
     name: nmo-test
   ```
   Apply the Namespace:
   ```sh
   $ oc create -f node-maintenance-namespace.yaml
   ```

2. **Create an OperatorGroup:**
   Create a YAML file named `node-maintenance-operator-group.yaml`:
   ```yaml
   apiVersion: operators.coreos.com/v1
   kind: OperatorGroup
   metadata:
     name: node-maintenance-operator
     namespace: nmo-test
   ```
   Apply the OperatorGroup:
   ```sh
   $ oc create -f node-maintenance-operator-group.yaml
   ```

3. **Create a Subscription:**
   Create a YAML file named `node-maintenance-subscription.yaml`:
   ```yaml
   apiVersion: operators.coreos.com/v1alpha1
   kind: Subscription
   metadata:
     name: node-maintenance-operator
     namespace: nmo-test
   spec:
     channel: stable
     installPlanApproval: Automatic
     name: node-maintenance-operator
     source: redhat-operators
     sourceNamespace: openshift-marketplace
     startingCSV: node-maintenance-operator.v5.3.0
   ```
   Apply the Subscription:
   ```sh
   $ oc create -f node-maintenance-subscription.yaml
   ```

4. **Verify the Installation:**
   ```sh
   $ oc get csv -n nmo-test
   ```
   Ensure the CSV resource is in the `Succeeded` phase:
   ```sh
   NAME                                      DISPLAY                     VERSION   REPLACES   PHASE
   node-maintenance-operator.v5.3.0          Node Maintenance Operator   5.3.0                Succeeded
   ```

5. **Verify the Operator Deployment:**
   ```sh
   $ oc get deploy -n nmo-test
   ```
   Ensure the deployment is running:
   ```sh
   NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
   node-maintenance-operator-controller-manager   1/1     1            1           10d
   ```
