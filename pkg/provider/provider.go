package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/imageservice/v2/images"
	"k8s.io/klog/v2"

	"github.com/bucher-brothers/openstack-autoscaler/pkg/config"
)

const (
	ProviderName = "openstack"
)

// OpenStackProvider implements the cloud provider interface for OpenStack
type OpenStackProvider struct {
	config        *config.Config
	computeClient *gophercloud.ServiceClient
	imageClient   *gophercloud.ServiceClient
	nodeGroups    map[string]*OpenStackNodeGroup
	mutex         sync.RWMutex
}

// NewOpenStackProvider creates a new OpenStack provider
func NewOpenStackProvider(cfg *config.Config) (*OpenStackProvider, error) {
	provider := &OpenStackProvider{
		config:     cfg,
		nodeGroups: make(map[string]*OpenStackNodeGroup),
	}

	// Initialize OpenStack clients
	if err := provider.initializeClients(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenStack clients: %w", err)
	}

	// NodeGroups are created dynamically via external-grpc protocol
	// No static initialization needed

	return provider, nil
}

// initializeClients initializes the OpenStack service clients
func (p *OpenStackProvider) initializeClients() error {
	// Create provider client
	authOptions := gophercloud.AuthOptions{
		IdentityEndpoint: p.config.Cloud.AuthURL,
		Username:         p.config.Cloud.Username,
		Password:         p.config.Cloud.Password,
		TenantName:       p.config.Cloud.ProjectName,
		TenantID:         p.config.Cloud.ProjectID,
		DomainName:       p.config.Cloud.UserDomainName,
		DomainID:         p.config.Cloud.ProjectDomainName,
	}

	providerClient, err := openstack.AuthenticatedClient(authOptions)
	if err != nil {
		return fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// Create compute client
	endpointOpts := gophercloud.EndpointOpts{
		Region:       p.config.Cloud.Region,
		Availability: gophercloud.AvailabilityPublic,
	}

	if p.config.Cloud.Interface != "" {
		switch strings.ToLower(p.config.Cloud.Interface) {
		case "public":
			endpointOpts.Availability = gophercloud.AvailabilityPublic
		case "internal":
			endpointOpts.Availability = gophercloud.AvailabilityInternal
		case "admin":
			endpointOpts.Availability = gophercloud.AvailabilityAdmin
		}
	}

	p.computeClient, err = openstack.NewComputeV2(providerClient, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	// Create image client
	p.imageClient, err = openstack.NewImageServiceV2(providerClient, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create image client: %w", err)
	}

	return nil
}

// GetNodeGroups returns all node groups
func (p *OpenStackProvider) GetNodeGroups() []*OpenStackNodeGroup {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	nodeGroups := make([]*OpenStackNodeGroup, 0, len(p.nodeGroups))
	for _, ng := range p.nodeGroups {
		nodeGroups = append(nodeGroups, ng)
	}
	return nodeGroups
}

// GetNodeGroup returns a specific node group by ID
func (p *OpenStackProvider) GetNodeGroup(id string) *OpenStackNodeGroup {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.nodeGroups[id]
}

// AddNodeGroup adds a new node group dynamically
func (p *OpenStackProvider) AddNodeGroup(ngConfig *config.NodeGroupConfig) (*OpenStackNodeGroup, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if node group already exists
	if _, exists := p.nodeGroups[ngConfig.ID]; exists {
		return p.nodeGroups[ngConfig.ID], nil
	}

	// Create new node group
	nodeGroup, err := NewOpenStackNodeGroup(ngConfig, p)
	if err != nil {
		return nil, fmt.Errorf("failed to create node group %s: %w", ngConfig.ID, err)
	}

	p.nodeGroups[ngConfig.ID] = nodeGroup
	klog.Infof("Added node group: %s", ngConfig.ID)
	return nodeGroup, nil
}

// NodeGroupForNode returns the node group for a given node
func (p *OpenStackProvider) NodeGroupForNode(nodeProviderID string) (*OpenStackNodeGroup, error) {
	// Extract server ID from provider ID (format: openstack://server-id)
	serverID := strings.TrimPrefix(nodeProviderID, ProviderName+"://")
	if serverID == nodeProviderID {
		return nil, fmt.Errorf("invalid provider ID format: %s", nodeProviderID)
	}

	// Get server details
	server, err := servers.Get(p.computeClient, serverID).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to get server %s: %w", serverID, err)
	}

	// Find the node group based on server metadata or other attributes
	for _, ng := range p.nodeGroups {
		if ng.ContainsNode(server) {
			return ng, nil
		}
	}

	return nil, nil // No node group found for this node
}

// ValidateConfiguration validates the OpenStack configuration
func (p *OpenStackProvider) ValidateConfiguration(ctx context.Context) error {
	klog.V(2).Info("Validating OpenStack configuration")

	// Test compute client by listing flavors
	allPages, err := flavors.ListDetail(p.computeClient, flavors.ListOpts{}).AllPages()
	if err != nil {
		return fmt.Errorf("failed to validate compute client: %w", err)
	}
	flavorList, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return fmt.Errorf("failed to extract flavors: %w", err)
	}
	klog.V(2).Infof("Found %d flavors in OpenStack", len(flavorList))

	// Test image client by listing images
	allPages, err = images.List(p.imageClient, images.ListOpts{}).AllPages()
	if err != nil {
		return fmt.Errorf("failed to validate image client: %w", err)
	}
	imageList, err := images.ExtractImages(allPages)
	if err != nil {
		return fmt.Errorf("failed to extract images: %w", err)
	}
	klog.V(2).Infof("Found %d images in OpenStack", len(imageList))

	// Validate node group configurations
	for _, ng := range p.nodeGroups {
		if err := ng.ValidateConfiguration(ctx); err != nil {
			return fmt.Errorf("node group %s validation failed: %w", ng.Config.ID, err)
		}
	}

	klog.Info("OpenStack configuration validation successful")
	return nil
}

// Refresh refreshes the provider state
func (p *OpenStackProvider) Refresh() error {
	klog.V(2).Info("Refreshing OpenStack provider state")

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, ng := range p.nodeGroups {
		if err := ng.Refresh(); err != nil {
			klog.Errorf("Failed to refresh node group %s: %v", ng.Config.ID, err)
		}
	}

	return nil
}

// Cleanup performs cleanup operations
func (p *OpenStackProvider) Cleanup() error {
	klog.Info("Cleaning up OpenStack provider")
	return nil
}
