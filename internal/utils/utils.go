package utils

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceQuantity creates a resource.Quantity from an integer value
func ResourceQuantity(value int) *resource.Quantity {
	q := resource.NewQuantity(int64(value), resource.DecimalSI)
	return q
}

// ResourceQuantityFromBytes creates a resource.Quantity from bytes
func ResourceQuantityFromBytes(bytes int) *resource.Quantity {
	q := resource.NewQuantity(int64(bytes), resource.BinarySI)
	return q
}
