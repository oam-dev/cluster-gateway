package v1alpha1

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	definitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "the name of the cluster"},
		{Name: "Provider", Type: "string", Description: "the cluster provider type"},
		{Name: "Credential-Type", Type: "string", Description: "the credential type"},
		{Name: "Endpoint-Type", Type: "string", Description: "the endpoint type"},
		{Name: "Healthy", Type: "string", Description: "the healthiness of the gateway"},
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
	epType := string(c.Spec.Access.Endpoint.Type)
	row := metav1.TableRow{
		Object: runtime.RawExtension{Object: c},
	}
	row.Cells = append(row.Cells, name, provideType, credType, epType, strconv.FormatBool(c.Status.Healthy))
	return row
}
