package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	definitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "the name of the cluster"},
		{Name: "Provider", Type: "string", Description: "the cluster provider type"},
		{Name: "Type", Type: "string", Description: "the credential type"},
		{Name: "Endpoint", Type: "string", Description: "the apiserver endpoint"},
	}
)

func printClusterGateway(in *ClusterGateway) *metav1.Table {
	return &metav1.Table{
		ColumnDefinitions: definitions,
		Rows:              []metav1.TableRow{printClusterGatewayRow(in)},
	}
}

func printClusterGatewayList(in *ClusterGatewayList) *metav1.Table {
	t := &metav1.Table{
		ColumnDefinitions: definitions,
	}
	for _, c := range in.Items {
		t.Rows = append(t.Rows, printClusterGatewayRow(&c))
	}
	return t
}

func printClusterGatewayRow(c *ClusterGateway) metav1.TableRow {
	name := c.Name
	provideType := c.Spec.Provider
	credType := "<none>"
	if c.Spec.Access.Credential != nil {
		credType = string(c.Spec.Access.Credential.Type)
	}
	ep := c.Spec.Access.Endpoint
	row := metav1.TableRow{
		Object: runtime.RawExtension{Object: c},
	}
	row.Cells = append(row.Cells, name, provideType, credType, ep)
	return row
}
