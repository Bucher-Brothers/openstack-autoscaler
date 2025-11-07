package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/bucher-brothers/openstack-autoscaler/internal/utils"
	"github.com/bucher-brothers/openstack-autoscaler/pkg/config"
)

// OpenStackNodeGroup represents a node group in OpenStack
type OpenStackNodeGroup struct {
	Config   *config.NodeGroupConfig
	Provider *OpenStackProvider
	mutex    sync.RWMutex

	// Cache for template node info
	templateNodeInfo *apiv1.Node
	lastRefresh      time.Time
}

// NewOpenStackNodeGroup creates a new OpenStack node group
func NewOpenStackNodeGroup(cfg *config.NodeGroupConfig, provider *OpenStackProvider) (*OpenStackNodeGroup, error) {
	ng := &OpenStackNodeGroup{
		Config:   cfg,
		Provider: provider,
	}

	// Validate configuration
	if err := ng.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid node group configuration: %w", err)
	}

	return ng, nil
}

// validateConfig validates the node group configuration
func (ng *OpenStackNodeGroup) validateConfig() error {
	if ng.Config.ID == "" {
		return fmt.Errorf("node group ID is required")
	}
	if ng.Config.MinSize < 0 {
		return fmt.Errorf("minSize cannot be negative")
	}
	if ng.Config.MaxSize < ng.Config.MinSize {
		return fmt.Errorf("maxSize (%d) must be >= minSize (%d)", ng.Config.MaxSize, ng.Config.MinSize)
	}
	if ng.Config.FlavorName == "" {
		return fmt.Errorf("flavorName is required")
	}
	if ng.Config.ImageName == "" && ng.Config.ImageID == "" {
		return fmt.Errorf("either imageName or imageId is required")
	}
	return nil
}

// ID returns the node group ID
func (ng *OpenStackNodeGroup) ID() string {
	return ng.Config.ID
}

// MinSize returns the minimum size of the node group
func (ng *OpenStackNodeGroup) MinSize() int {
	return ng.Config.MinSize
}

// MaxSize returns the maximum size of the node group
func (ng *OpenStackNodeGroup) MaxSize() int {
	return ng.Config.MaxSize
}

// TargetSize returns the current target size of the node group
func (ng *OpenStackNodeGroup) TargetSize() (int, error) {
	instances, err := ng.getInstances()
	if err != nil {
		return 0, fmt.Errorf("failed to get instances: %w", err)
	}

	// Count only running and creating instances
	count := 0
	for _, instance := range instances {
		if instance.Status == "ACTIVE" || instance.Status == "BUILD" {
			count++
		}
	}

	return count, nil
}

// IncreaseSize increases the size of the node group
func (ng *OpenStackNodeGroup) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("delta must be positive, got %d", delta)
	}

	currentSize, err := ng.TargetSize()
	if err != nil {
		return fmt.Errorf("failed to get current size: %w", err)
	}

	newSize := currentSize + delta
	if newSize > ng.Config.MaxSize {
		return fmt.Errorf("cannot increase size to %d, max size is %d", newSize, ng.Config.MaxSize)
	}

	klog.Infof("Increasing node group %s from %d to %d nodes", ng.Config.ID, currentSize, newSize)

	// Create new servers
	for i := 0; i < delta; i++ {
		if err := ng.createServer(); err != nil {
			klog.Errorf("Failed to create server %d/%d for node group %s: %v", i+1, delta, ng.Config.ID, err)
			return fmt.Errorf("failed to create server: %w", err)
		}
	}

	return nil
}

// DecreaseTargetSize decreases the target size of the node group
func (ng *OpenStackNodeGroup) DecreaseTargetSize(delta int) error {
	if delta >= 0 {
		return fmt.Errorf("delta must be negative, got %d", delta)
	}

	currentSize, err := ng.TargetSize()
	if err != nil {
		return fmt.Errorf("failed to get current size: %w", err)
	}

	newSize := currentSize + delta // delta is negative
	if newSize < ng.Config.MinSize {
		return fmt.Errorf("cannot decrease size to %d, min size is %d", newSize, ng.Config.MinSize)
	}

	klog.Infof("Decreasing node group %s from %d to %d nodes", ng.Config.ID, currentSize, newSize)

	// We don't actually delete nodes here, just reduce the target size
	// The cluster autoscaler will handle the actual node deletion
	return nil
}

// DeleteNodes deletes the specified nodes from the group
func (ng *OpenStackNodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	klog.Infof("Deleting %d nodes from node group %s", len(nodes), ng.Config.ID)

	for _, node := range nodes {
		if err := ng.deleteNode(node); err != nil {
			klog.Errorf("Failed to delete node %s: %v", node.Name, err)
			return fmt.Errorf("failed to delete node %s: %w", node.Name, err)
		}
	}

	return nil
}

// Nodes returns a list of all nodes in the group
func (ng *OpenStackNodeGroup) Nodes() ([]servers.Server, error) {
	instances, err := ng.getInstances()
	if err != nil {
		return nil, fmt.Errorf("failed to get instances: %w", err)
	}

	return instances, nil
}

// TemplateNodeInfo returns a template node info for scale-up simulations
func (ng *OpenStackNodeGroup) TemplateNodeInfo() (*apiv1.Node, error) {
	ng.mutex.Lock()
	defer ng.mutex.Unlock()

	// Use cached template if available and not too old
	if ng.templateNodeInfo != nil && time.Since(ng.lastRefresh) < 10*time.Minute {
		return ng.templateNodeInfo.DeepCopy(), nil
	}

	// Create template node info
	node, err := ng.buildTemplateNodeInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to build template node info: %w", err)
	}

	ng.templateNodeInfo = node
	ng.lastRefresh = time.Now()

	return node.DeepCopy(), nil
}

// buildTemplateNodeInfo builds a template node info based on the node group configuration
func (ng *OpenStackNodeGroup) buildTemplateNodeInfo() (*apiv1.Node, error) {
	// Get flavor information
	flavor, err := ng.getFlavor()
	if err != nil {
		return nil, fmt.Errorf("failed to get flavor: %w", err)
	}

	// Create node template
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-template", ng.Config.ID),
			Labels: map[string]string{
				"kubernetes.io/arch":               "amd64",
				"kubernetes.io/os":                 "linux",
				"node.kubernetes.io/instance-type": flavor.Name,
			},
		},
		Spec: apiv1.NodeSpec{
			ProviderID: fmt.Sprintf("%s://template-%s", ProviderName, ng.Config.ID),
		},
		Status: apiv1.NodeStatus{
			Capacity: apiv1.ResourceList{
				apiv1.ResourceCPU:    *utils.ResourceQuantity(flavor.VCPUs),
				apiv1.ResourceMemory: *utils.ResourceQuantityFromBytes(flavor.RAM * 1024 * 1024), // Convert MB to bytes
			},
			Allocatable: apiv1.ResourceList{
				apiv1.ResourceCPU:    *utils.ResourceQuantity(flavor.VCPUs),
				apiv1.ResourceMemory: *utils.ResourceQuantityFromBytes(flavor.RAM * 1024 * 1024), // Convert MB to bytes
			},
			Conditions: []apiv1.NodeCondition{
				{
					Type:   apiv1.NodeReady,
					Status: apiv1.ConditionTrue,
				},
			},
		},
	}

	// Add custom labels from config
	for k, v := range ng.Config.Labels {
		node.Labels[k] = v
	}

	return node, nil
}

// ContainsNode checks if a server belongs to this node group
func (ng *OpenStackNodeGroup) ContainsNode(server *servers.Server) bool {
	// Check if server has the node group metadata
	if nodeGroupID, exists := server.Metadata["nodegroup"]; exists {
		return nodeGroupID == ng.Config.ID
	}

	// Fallback: check if server name contains node group ID
	return strings.Contains(server.Name, ng.Config.ID)
}

// createServer creates a new server in OpenStack
func (ng *OpenStackNodeGroup) createServer() error {
	// Get image ID
	imageID, err := ng.getImageID()
	if err != nil {
		return fmt.Errorf("failed to get image ID: %w", err)
	}

	// Get flavor ID
	flavor, err := ng.getFlavor()
	if err != nil {
		return fmt.Errorf("failed to get flavor: %w", err)
	}

	// Prepare user data
	userData := ng.Config.UserData
	if userData != "" {
		userData = base64.StdEncoding.EncodeToString([]byte(userData))
	}

	// Prepare metadata
	metadata := make(map[string]string)
	for k, v := range ng.Config.Metadata {
		metadata[k] = v
	}
	metadata["nodegroup"] = ng.Config.ID
	metadata["created_by"] = "openstack-autoscaler"

	// Prepare security groups
	securityGroups := make([]string, len(ng.Config.SecurityGroups))
	copy(securityGroups, ng.Config.SecurityGroups)

	// Create server options
	serverName := fmt.Sprintf("%s-%d", ng.Config.ID, time.Now().Unix())
	createOpts := servers.CreateOpts{
		Name:           serverName,
		ImageRef:       imageID,
		FlavorRef:      flavor.ID,
		UserData:       []byte(userData),
		Metadata:       metadata,
		SecurityGroups: securityGroups,
	}

	if ng.Config.KeyName != "" {
		// SSH key will be handled in user data or metadata
		metadata["key_name"] = ng.Config.KeyName
	}

	if ng.Config.AvailabilityZone != "" {
		createOpts.AvailabilityZone = ng.Config.AvailabilityZone
	}

	// Add networks if specified
	if ng.Config.NetworkID != "" {
		createOpts.Networks = []servers.Network{
			{UUID: ng.Config.NetworkID},
		}
	}

	klog.Infof("Creating server %s for node group %s", serverName, ng.Config.ID)
	server, err := servers.Create(context.TODO(), ng.Provider.computeClient, createOpts, nil).Extract()
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	klog.Infof("Server %s (%s) created successfully for node group %s", server.Name, server.ID, ng.Config.ID)
	return nil
}

// deleteNode deletes a node from OpenStack
func (ng *OpenStackNodeGroup) deleteNode(node *apiv1.Node) error {
	// Extract server ID from provider ID
	providerID := node.Spec.ProviderID
	serverID := strings.TrimPrefix(providerID, ProviderName+"://")
	if serverID == providerID {
		return fmt.Errorf("invalid provider ID format: %s", providerID)
	}

	klog.Infof("Deleting server %s for node %s in node group %s", serverID, node.Name, ng.Config.ID)

	err := servers.Delete(context.TODO(), ng.Provider.computeClient, serverID).ExtractErr()
	if err != nil {
		return fmt.Errorf("failed to delete server %s: %w", serverID, err)
	}

	klog.Infof("Server %s deleted successfully", serverID)
	return nil
}

// getInstances returns all instances belonging to this node group
func (ng *OpenStackNodeGroup) getInstances() ([]servers.Server, error) {
	// List all servers
	allPages, err := servers.List(ng.Provider.computeClient, servers.ListOpts{}).AllPages(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract servers: %w", err)
	}

	// Filter servers belonging to this node group
	var groupServers []servers.Server
	for _, server := range allServers {
		if ng.ContainsNode(&server) {
			groupServers = append(groupServers, server)
		}
	}

	return groupServers, nil
}

// getFlavor returns the flavor for this node group
func (ng *OpenStackNodeGroup) getFlavor() (*flavors.Flavor, error) {
	flavor, err := flavors.Get(context.TODO(), ng.Provider.computeClient, ng.Config.FlavorName).Extract()
	if err != nil {
		// Try to find flavor by name
		allPages, err := flavors.ListDetail(ng.Provider.computeClient, flavors.ListOpts{}).AllPages(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to list flavors: %w", err)
		}

		allFlavors, err := flavors.ExtractFlavors(allPages)
		if err != nil {
			return nil, fmt.Errorf("failed to extract flavors: %w", err)
		}

		for _, f := range allFlavors {
			if f.Name == ng.Config.FlavorName {
				return &f, nil
			}
		}

		return nil, fmt.Errorf("flavor %s not found", ng.Config.FlavorName)
	}

	return flavor, nil
}

// getImageID returns the image ID for this node group
func (ng *OpenStackNodeGroup) getImageID() (string, error) {
	if ng.Config.ImageID != "" {
		return ng.Config.ImageID, nil
	}

	// Find image by name
	listOpts := images.ListOpts{
		Name: ng.Config.ImageName,
	}

	allPages, err := images.List(ng.Provider.imageClient, listOpts).AllPages(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to list images: %w", err)
	}

	allImages, err := images.ExtractImages(allPages)
	if err != nil {
		return "", fmt.Errorf("failed to extract images: %w", err)
	}

	if len(allImages) == 0 {
		return "", fmt.Errorf("image %s not found", ng.Config.ImageName)
	}

	return allImages[0].ID, nil
}

// ValidateConfiguration validates the node group configuration against OpenStack
func (ng *OpenStackNodeGroup) ValidateConfiguration(ctx context.Context) error {
	// Validate flavor
	_, err := ng.getFlavor()
	if err != nil {
		return fmt.Errorf("flavor validation failed: %w", err)
	}

	// Validate image
	_, err = ng.getImageID()
	if err != nil {
		return fmt.Errorf("image validation failed: %w", err)
	}

	klog.V(2).Infof("Node group %s configuration is valid", ng.Config.ID)
	return nil
}

// Refresh refreshes the node group state
func (ng *OpenStackNodeGroup) Refresh() error {
	ng.mutex.Lock()
	defer ng.mutex.Unlock()

	// Clear cached template node info to force refresh
	ng.templateNodeInfo = nil
	ng.lastRefresh = time.Time{}

	return nil
}
