/*
Copyright 2025 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dpfctl

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Copied from: https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L48
const (
	firstElemPrefix = `├─`
	lastElemPrefix  = `└─`
	indent          = "  "
	pipe            = `│ `
)

// Copied from: https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L55
var (
	gray   = color.New(color.FgHiBlack)
	red    = color.New(color.FgRed)
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	white  = color.New(color.FgWhite)
	cyan   = color.New(color.FgCyan)
)

// PrintObjectTree prints the DPF status to stdout.
// Adopted from: https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L213
func PrintObjectTree(tree *ObjectTree) {
	// Creates the output table
	tbl := tablewriter.NewWriter(os.Stdout)
	tbl.SetHeader([]string{"NAME", "READY", "REASON", "SINCE", "MESSAGE"})

	formatTableTree(tbl)
	// Add row for the root object, the DPFOperatorConfig, and recursively for all the resources representing the DPF status.
	addObjectRow("", tbl, tree, tree.GetRoot())

	// Prints the output table
	tbl.Render()
}

// formats the table with required attributes.
// Copy from: https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L241
func formatTableTree(tbl *tablewriter.Table) {
	tbl.SetAutoWrapText(false)
	tbl.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	tbl.SetAlignment(tablewriter.ALIGN_LEFT)

	tbl.SetCenterSeparator("")
	tbl.SetColumnSeparator("")
	tbl.SetRowSeparator("")

	tbl.SetHeaderLine(false)
	tbl.SetBorder(false)
	tbl.SetTablePadding("  ")
	tbl.SetNoWhiteSpace(true)
}

// addOtherConditions adds a row for each object condition except the ready condition,
// which is already represented on the object's main row.
// Adopted from:https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L470
func addOtherConditions(prefix string, tbl *tablewriter.Table, objectTree *ObjectTree, obj client.Object) {
	// Add a row for each other condition, taking care of updating the tree view prefix.
	// In this case the tree prefix get a filler, to indent conditions from objects, and eventually
	// an additional pipe if the object has children that should be presented after the conditions.
	filler := strings.Repeat(" ", 10)
	childrenPipe := indent
	if objectTree.IsObjectWithChild(obj.GetUID()) {
		childrenPipe = pipe
	}

	otherConditions := GetOtherConditions(obj)
	for i := range otherConditions {
		otherCondition := otherConditions[i]
		otherDescriptor := newConditionDescriptor(otherCondition, objectTree.options.WrapLines)
		otherConditionPrefix := getChildPrefix(prefix+childrenPipe+filler, i, len(otherConditions))
		tbl.Append([]string{
			fmt.Sprintf("%s%s", gray.Sprint(otherConditionPrefix), cyan.Sprint(otherCondition.Type)),
			otherDescriptor.readyColor.Sprint(otherDescriptor.status),
			otherDescriptor.readyColor.Sprint(otherDescriptor.reason),
			otherDescriptor.age,
			otherDescriptor.message})
	}
}

// addObjectRow add a row for a given object, and recursively for all the object's children.
// NOTE: each row name gets a prefix, that generates a tree view like representation.
// Adopted from: https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L347
func addObjectRow(prefix string, tbl *tablewriter.Table, objectTree *ObjectTree, obj client.Object) {
	// Gets the descriptor for the object's ready condition, if any.
	readyDescriptor := conditionDescriptor{readyColor: gray}
	if ready := getReadyCondition(obj); ready != nil {
		readyDescriptor = newConditionDescriptor(ready, objectTree.options.WrapLines)
	}

	// If the object is a group object, override the condition message with the list of objects in the group. e.g dpu-1, dpu-2, ...
	if IsGroupObject(obj) {
		items := strings.Split(GetGroupItems(obj), GroupItemsSeparator)
		if len(items) <= 2 {
			readyDescriptor.message = gray.Sprintf("See %s", strings.Join(items, GroupItemsSeparator))
		} else {
			readyDescriptor.message = gray.Sprintf("See %s, ...", strings.Join(items[:2], GroupItemsSeparator))
		}
	}

	// Gets the row name for the object.
	// NOTE: The object name gets manipulated in order to improve readability.
	name := getRowName(obj)

	// Add the row representing the object that includes
	// - The row name with the tree view prefix.
	// - The object's ready condition.
	tbl.Append([]string{
		fmt.Sprintf("%s%s", gray.Sprint(prefix), name),
		readyDescriptor.readyColor.Sprint(readyDescriptor.status),
		readyDescriptor.readyColor.Sprint(readyDescriptor.reason),
		readyDescriptor.age,
		readyDescriptor.message})

	// If it is required to show all the conditions for the object, add a row for each object's conditions.
	if IsShowConditionsObject(obj) {
		addOtherConditions(prefix, tbl, objectTree, obj)
	}

	// Add a row for each object's children, taking care of updating the tree view prefix.
	childrenObj := objectTree.GetObjectsByParent(obj.GetUID())

	// printBefore returns true if children[i] should be printed before children[j]. Objects are sorted by z-order and
	// row name such that objects with higher z-order are printed first, and objects with the same z-order are
	// printed in alphabetical order.
	printBefore := func(i, j int) bool {
		if GetZOrder(childrenObj[i]) == GetZOrder(childrenObj[j]) {
			return getRowName(childrenObj[i]) < getRowName(childrenObj[j])
		}

		return GetZOrder(childrenObj[i]) > GetZOrder(childrenObj[j])
	}
	sort.Slice(childrenObj, printBefore)

	for i, child := range childrenObj {
		addObjectRow(getChildPrefix(prefix, i, len(childrenObj)), tbl, objectTree, child)
	}
}

// getChildPrefix return the tree view prefix for a row representing a child object.
// Copy from:https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L496
func getChildPrefix(currentPrefix string, childIndex, childCount int) string {
	nextPrefix := currentPrefix

	// Alter the current prefix for hosting the next child object:

	// All ├─ should be replaced by |, so all the existing hierarchic dependencies are carried on
	nextPrefix = strings.ReplaceAll(nextPrefix, firstElemPrefix, pipe)
	// All └─ should be replaced by " " because we are under the last element of the tree (nothing to carry on)
	nextPrefix = strings.ReplaceAll(nextPrefix, lastElemPrefix, strings.Repeat(" ", len([]rune(lastElemPrefix))))

	// Add the prefix for the new child object (├─ for the firsts children, └─ for the last children).
	if childIndex < childCount-1 {
		return nextPrefix + firstElemPrefix
	}
	return nextPrefix + lastElemPrefix
}

// getRowName returns the object name in the tree view according to following rules:
// - group objects are represented as objects kind, e.g. 3 DPUs...
// - other virtual objects are represented using the object name, e.g. Workers, or meta name if provided.
// - objects with a meta name are represented as meta name - (kind/name), e.g. DPUApplication - Application/flannel
// - other objects are represented as kind/name, e.g. DPU/worker1-0000-08-00
// - if the object is being deleted, a prefix will be added.
// Adopted from:https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L533
func getRowName(obj client.Object) string {
	if IsGroupObject(obj) {
		items := strings.Split(GetGroupItems(obj), GroupItemsSeparator)
		kind := strings.TrimSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "Group")
		if kind[len(kind)-1] != 's' {
			kind = kind + "s"
		}
		return white.Add(color.Bold).Sprintf("%d %s...", len(items), kind)
	}

	if IsVirtualObject(obj) {
		if metaName := GetMetaName(obj); metaName != "" {
			return metaName
		}
		return obj.GetName()
	}

	objName := fmt.Sprintf("%s/%s",
		obj.GetObjectKind().GroupVersionKind().Kind,
		color.New(color.Bold).Sprint(obj.GetName()))

	name := objName
	if objectPrefix := GetMetaName(obj); objectPrefix != "" {
		name = fmt.Sprintf("%s - %s", objectPrefix, gray.Sprintf("%s", name))
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		name = fmt.Sprintf("%s %s", red.Sprintf("!! IN DELETION !!"), name)
	}

	return name
}

// conditionDescriptor contains all the info for representing a condition.
// Copy from:https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L804
type conditionDescriptor struct {
	readyColor *color.Color
	age        string
	status     string
	reason     string
	message    string
}

// newConditionDescriptor returns a conditionDescriptor for the given condition.
// Adopted from:https://github.com/kubernetes-sigs/cluster-api/blob/release-1.9/cmd/clusterctl/cmd/describe_cluster.go#L813
func newConditionDescriptor(c *metav1.Condition, wrapLines bool) conditionDescriptor {
	v := conditionDescriptor{}

	v.status = string(c.Status)
	v.reason = c.Reason
	v.message = c.Message

	// If it's longer than 100 characters wrap the words.
	if len(v.message) > 100 {
		if wrapLines {
			tmp, _ := tablewriter.WrapString(v.message, 100)
			v.message = strings.Join(tmp, "\n")
		} else {
			v.message = fmt.Sprintf("%s...", v.message[:100])
		}
	}

	// Compute the condition age.
	v.age = duration.HumanDuration(time.Since(c.LastTransitionTime.Time))

	// Determine the color to be used for showing the conditions according to Status and Severity in case Status is false.
	switch c.Status {
	case metav1.ConditionTrue:
		v.readyColor = green
	case metav1.ConditionFalse, metav1.ConditionUnknown:
		v.readyColor = white
		if strings.HasSuffix(c.Type, "Ready") {
			v.readyColor = red
		}
		if strings.HasSuffix(c.Type, "Reconciled") {
			v.readyColor = yellow
		}
	default:
		v.readyColor = gray
	}

	return v
}
