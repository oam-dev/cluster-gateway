package v1alpha1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcestrategy"
)

var _ resourcestrategy.TableConverter = &ClusterGateway{}
var _ resourcestrategy.TableConverter = &ClusterGatewayList{}

var (
	definitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "the name of the cluster"},
		{Name: "Provider", Type: "string", Description: "the cluster provider type"},
		{Name: "Type", Type: "string", Description: "the credential type"},
		{Name: "Endpoint", Type: "string", Description: "the apiserver endpoint"},
	}
)

func (in *ClusterGateway) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return &metav1.Table{
		ColumnDefinitions: definitions,
		Rows:              []metav1.TableRow{printClusterGateway(in)},
	}, nil
}

func (in *ClusterGatewayList) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	t := &metav1.Table{
		ColumnDefinitions: definitions,
	}
	for _, c := range in.Items {
		t.Rows = append(t.Rows, printClusterGateway(&c))
	}
	return t, nil
}

func printClusterGateway(c *ClusterGateway) metav1.TableRow {
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
